// Package metrics provides the process-local metrics registry used by the
// protected HTTP metrics endpoint. It deliberately uses the Prometheus text
// exposition format without a third-party client so the registry remains
// instance-scoped and cheap to embed in the application composition root.
package metrics

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	accountsOnlineMetric       = "farmbot_accounts_online"
	accountOnlineMetric        = "farmbot_account_online"
	accountOperationMetric     = "farmbot_account_operation_total"
	websocketConnectionsMetric = "farmbot_websocket_connections"
	loginsMetric               = "farmbot_logins_total"
	tsdkFailuresMetric         = "farmbot_tsdk_heartbeat_failures_total"
	aceFailuresMetric          = "farmbot_ace_heartbeat_failures_total"
	httpDurationMetric         = "farmbot_http_request_duration_seconds"
)

var defaultHistogramBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// AccountSnapshot is the account-local data needed to expose account gauges
// and operation counters. The Operations map is copied during collection, so
// a source may safely reuse its runtime-owned map after returning.
type AccountSnapshot struct {
	AccountID  string
	Online     bool
	Operations map[string]float64
}

// AccountSource is evaluated only when /metrics is scraped. Keeping runtime
// traversal behind a callback avoids making this platform package depend on
// the account manager's lifecycle or storage implementation.
type AccountSource func(context.Context) ([]AccountSnapshot, error)

// Sources contains optional live data adapters. Counters recorded directly on
// Registry remain useful when a source is not available (for example during
// startup or in a focused unit test).
type Sources struct {
	Accounts              AccountSource
	WebSocketConnections  func() int
	TSDKHeartbeatFailures func(context.Context) (uint64, error)
	ACEHeartbeatFailures  func(context.Context) (uint64, error)
}

// Config configures one Registry instance. Sources and all mutable counters
// belong to the resulting instance; there is no package-global registry.
type Config struct {
	Sources Sources

	// The direct fields are convenient for small composition roots. When set,
	// they take precedence over the corresponding Sources field.
	AccountSource         AccountSource
	WebSocketConnections  func() int
	TSDKHeartbeatFailures func(context.Context) (uint64, error)
	ACEHeartbeatFailures  func(context.Context) (uint64, error)

	HistogramBuckets []float64
	Now              func() time.Time
}

type histogramKey struct {
	method string
	route  string
}

type histogram struct {
	buckets []uint64 // cumulative counts, one entry per configured bucket
	sum     float64
	count   uint64
}

// HTTPHistogramSnapshot is an immutable scrape-time copy of one latency
// histogram. Bucket counts are cumulative and align with the registry's
// configured bucket boundaries.
type HTTPHistogramSnapshot struct {
	Method  string
	Route   string
	Buckets []uint64
	Sum     float64
	Count   uint64
}

// Snapshot is the fully materialized view used by Render and by diagnostics.
// It is safe for callers to mutate after Collect returns.
type Snapshot struct {
	CollectedAt time.Time
	Accounts    []AccountSnapshot

	AccountsOnline        int
	WebSocketConnections  int
	LoginSuccesses        uint64
	LoginFailures         uint64
	TSDKHeartbeatFailures uint64
	ACEHeartbeatFailures  uint64

	HTTPHistograms []HTTPHistogramSnapshot
}

// Registry owns all mutable instrumentation for one application instance.
type Registry struct {
	sources Sources
	buckets []float64
	now     func() time.Time

	loginSuccesses        atomic.Uint64
	loginFailures         atomic.Uint64
	tsdkHeartbeatFailures atomic.Uint64
	aceHeartbeatFailures  atomic.Uint64
	websocketConnections  atomic.Int64

	httpMu       sync.RWMutex
	httpCounters map[histogramKey]*histogram
}

// New creates an instance-scoped metrics registry.
func New(cfg Config) *Registry {
	sources := cfg.Sources
	if cfg.AccountSource != nil {
		sources.Accounts = cfg.AccountSource
	}
	if cfg.WebSocketConnections != nil {
		sources.WebSocketConnections = cfg.WebSocketConnections
	}
	if cfg.TSDKHeartbeatFailures != nil {
		sources.TSDKHeartbeatFailures = cfg.TSDKHeartbeatFailures
	}
	if cfg.ACEHeartbeatFailures != nil {
		sources.ACEHeartbeatFailures = cfg.ACEHeartbeatFailures
	}

	buckets := normalizeBuckets(cfg.HistogramBuckets)
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Registry{
		sources:      sources,
		buckets:      buckets,
		now:          now,
		httpCounters: make(map[histogramKey]*histogram),
	}
}

// NewRegistry is a descriptive constructor alias.
func NewRegistry(cfg Config) *Registry { return New(cfg) }

// RecordLoginSuccess increments the successful-login counter.
func (r *Registry) RecordLoginSuccess() {
	if r != nil {
		r.loginSuccesses.Add(1)
	}
}

// RecordLoginFailure increments the failed-login counter.
func (r *Registry) RecordLoginFailure() {
	if r != nil {
		r.loginFailures.Add(1)
	}
}

// RecordLogin is a compact adapter for authentication code. Unknown results
// are ignored so a caller cannot silently create a third Prometheus series.
func (r *Registry) RecordLogin(result string) {
	switch strings.ToLower(strings.TrimSpace(result)) {
	case "success", "succeeded", "ok":
		r.RecordLoginSuccess()
	case "failure", "failed", "error":
		r.RecordLoginFailure()
	}
}

// RecordTSDKHeartbeatFailure increments the TSDK heartbeat failure counter.
func (r *Registry) RecordTSDKHeartbeatFailure() {
	if r != nil {
		r.tsdkHeartbeatFailures.Add(1)
	}
}

// RecordACEHeartbeatFailure increments the ACE heartbeat failure counter.
func (r *Registry) RecordACEHeartbeatFailure() {
	if r != nil {
		r.aceHeartbeatFailures.Add(1)
	}
}

// RecordHeartbeatFailure records a component-specific heartbeat failure.
func (r *Registry) RecordHeartbeatFailure(component string) {
	switch strings.ToLower(strings.TrimSpace(component)) {
	case "tsdk", "sdk":
		r.RecordTSDKHeartbeatFailure()
	case "ace":
		r.RecordACEHeartbeatFailure()
	}
}

// SetWebSocketConnections replaces the current WebSocket connection gauge.
func (r *Registry) SetWebSocketConnections(count int) {
	if r == nil {
		return
	}
	if count < 0 {
		count = 0
	}
	r.websocketConnections.Store(int64(count))
}

// ObserveHTTP records one request duration in seconds. It only updates a
// fixed-size histogram, keeping request instrumentation bounded and cheap.
func (r *Registry) ObserveHTTP(method, route string, duration time.Duration) {
	if r == nil {
		return
	}
	method = normalizeMethod(method)
	route = normalizeRoute(route)
	seconds := duration.Seconds()
	if seconds < 0 {
		seconds = 0
	}
	key := histogramKey{method: method, route: route}

	r.httpMu.Lock()
	h := r.httpCounters[key]
	if h == nil {
		h = &histogram{buckets: make([]uint64, len(r.buckets))}
		r.httpCounters[key] = h
	}
	idx := sort.SearchFloat64s(r.buckets, seconds)
	for i := idx; i < len(h.buckets); i++ {
		h.buckets[i]++
	}
	h.sum += seconds
	h.count++
	r.httpMu.Unlock()
}

// ObserveHTTPRequest is an alias matching the common middleware naming.
func (r *Registry) ObserveHTTPRequest(method, route string, duration time.Duration) {
	r.ObserveHTTP(method, route, duration)
}

// Collect takes a scrape-time snapshot. Source errors are returned to the
// endpoint so monitoring does not report a misleading partial view.
func (r *Registry) Collect(ctx context.Context) (Snapshot, error) {
	if r == nil {
		return Snapshot{}, errors.New("metrics registry is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}

	accounts := make([]AccountSnapshot, 0)
	if r.sources.Accounts != nil {
		value, err := r.sources.Accounts(ctx)
		if err != nil {
			return Snapshot{}, fmt.Errorf("collect account metrics: %w", err)
		}
		accounts = cloneAccounts(value)
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].AccountID < accounts[j].AccountID })
	online := 0
	for _, account := range accounts {
		if account.Online {
			online++
		}
	}

	ws := int(r.websocketConnections.Load())
	if r.sources.WebSocketConnections != nil {
		ws = r.sources.WebSocketConnections()
		if ws < 0 {
			ws = 0
		}
	}
	tsdkFailures, err := sourceCounter(ctx, r.sources.TSDKHeartbeatFailures, r.tsdkHeartbeatFailures.Load())
	if err != nil {
		return Snapshot{}, fmt.Errorf("collect TSDK heartbeat metrics: %w", err)
	}
	aceFailures, err := sourceCounter(ctx, r.sources.ACEHeartbeatFailures, r.aceHeartbeatFailures.Load())
	if err != nil {
		return Snapshot{}, fmt.Errorf("collect ACE heartbeat metrics: %w", err)
	}

	r.httpMu.RLock()
	histograms := make([]HTTPHistogramSnapshot, 0, len(r.httpCounters))
	for key, value := range r.httpCounters {
		histograms = append(histograms, HTTPHistogramSnapshot{
			Method: key.method, Route: key.route,
			Buckets: append([]uint64(nil), value.buckets...),
			Sum:     value.sum, Count: value.count,
		})
	}
	r.httpMu.RUnlock()
	sort.Slice(histograms, func(i, j int) bool {
		if histograms[i].Method == histograms[j].Method {
			return histograms[i].Route < histograms[j].Route
		}
		return histograms[i].Method < histograms[j].Method
	})

	return Snapshot{
		CollectedAt: r.now(), Accounts: accounts,
		AccountsOnline: online, WebSocketConnections: ws,
		LoginSuccesses: r.loginSuccesses.Load(), LoginFailures: r.loginFailures.Load(),
		TSDKHeartbeatFailures: tsdkFailures, ACEHeartbeatFailures: aceFailures,
		HTTPHistograms: histograms,
	}, nil
}

// Render returns Prometheus text exposition data with stable ordering and
// escaped label values. Histogram buckets are cumulative as required by the
// Prometheus format.
func (r *Registry) Render(ctx context.Context) ([]byte, error) {
	snapshot, err := r.Collect(ctx)
	if err != nil {
		return nil, err
	}
	var out strings.Builder
	writeGauge(&out, accountsOnlineMetric, "Number of online FarmBot accounts.", float64(snapshot.AccountsOnline), nil)
	writeMetadata(&out, accountOnlineMetric, "Whether a FarmBot account is online.", "gauge")
	writeMetadata(&out, accountOperationMetric, "Number of operations performed by a FarmBot account.", "counter")
	for _, account := range snapshot.Accounts {
		value := 0.0
		if account.Online {
			value = 1
		}
		writeSample(&out, accountOnlineMetric, labels{"account_id": account.AccountID}, value)
		operations := make([]string, 0, len(account.Operations))
		for operation := range account.Operations {
			operations = append(operations, operation)
		}
		sort.Strings(operations)
		for _, operation := range operations {
			writeSample(&out, accountOperationMetric, labels{"account_id": account.AccountID, "operation": operation}, account.Operations[operation])
		}
	}
	writeGauge(&out, websocketConnectionsMetric, "Number of connected FarmBot WebSocket clients.", float64(snapshot.WebSocketConnections), nil)
	writeMetadata(&out, loginsMetric, "Number of FarmBot login attempts by result.", "counter")
	writeSample(&out, loginsMetric, labels{"result": "success"}, snapshot.LoginSuccesses)
	writeSample(&out, loginsMetric, labels{"result": "failure"}, snapshot.LoginFailures)
	writeCounter(&out, tsdkFailuresMetric, "Number of failed TSDK heartbeat operations.", nil, snapshot.TSDKHeartbeatFailures)
	writeCounter(&out, aceFailuresMetric, "Number of failed ACE heartbeat operations.", nil, snapshot.ACEHeartbeatFailures)

	out.WriteString("# HELP " + httpDurationMetric + " HTTP request duration in seconds.\n")
	out.WriteString("# TYPE " + httpDurationMetric + " histogram\n")
	for _, histogram := range snapshot.HTTPHistograms {
		base := httpDurationMetric + "_bucket"
		for i, bucket := range r.buckets {
			writeSample(&out, base, labels{"method": histogram.Method, "route": histogram.Route, "le": formatFloat(bucket)}, countAt(histogram.Buckets, i))
		}
		writeSample(&out, base, labels{"method": histogram.Method, "route": histogram.Route, "le": "+Inf"}, histogram.Count)
		writeSample(&out, httpDurationMetric+"_sum", labels{"method": histogram.Method, "route": histogram.Route}, histogram.Sum)
		writeSample(&out, httpDurationMetric+"_count", labels{"method": histogram.Method, "route": histogram.Route}, histogram.Count)
	}
	return []byte(out.String()), nil
}

type labels map[string]string

func writeGauge(out *strings.Builder, name, help string, value float64, labelSet labels) {
	writeMetadata(out, name, help, "gauge")
	writeSample(out, name, labelSet, value)
}

func writeCounter(out *strings.Builder, name, help string, labelSet labels, value uint64) {
	writeMetadata(out, name, help, "counter")
	writeSample(out, name, labelSet, value)
}

func writeMetadata(out *strings.Builder, name, help, metricType string) {
	out.WriteString("# HELP " + name + " " + help + "\n")
	out.WriteString("# TYPE " + name + " " + metricType + "\n")
}

func writeSample(out *strings.Builder, name string, labelSet labels, value any) {
	out.WriteString(name)
	if len(labelSet) > 0 {
		keys := make([]string, 0, len(labelSet))
		for key := range labelSet {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			out.WriteString(key)
			out.WriteString("=\"")
			out.WriteString(escapeLabel(labelSet[key]))
			out.WriteString("\"")
		}
		out.WriteByte('}')
	}
	out.WriteByte(' ')
	switch typed := value.(type) {
	case uint64:
		out.WriteString(strconv.FormatUint(typed, 10))
	case int:
		out.WriteString(strconv.Itoa(typed))
	case float64:
		out.WriteString(formatFloat(typed))
	default:
		out.WriteString(fmt.Sprint(typed))
	}
	out.WriteByte('\n')
}

func normalizeBuckets(input []float64) []float64 {
	if len(input) == 0 {
		return append([]float64(nil), defaultHistogramBuckets...)
	}
	result := make([]float64, 0, len(input))
	for _, bucket := range input {
		if bucket >= 0 && !containsFloat(result, bucket) {
			result = append(result, bucket)
		}
	}
	if len(result) == 0 {
		return append([]float64(nil), defaultHistogramBuckets...)
	}
	sort.Float64s(result)
	return result
}

func containsFloat(values []float64, target float64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func cloneAccounts(input []AccountSnapshot) []AccountSnapshot {
	result := make([]AccountSnapshot, 0, len(input))
	for _, account := range input {
		account.AccountID = strings.TrimSpace(account.AccountID)
		if account.AccountID == "" {
			continue
		}
		if account.Operations != nil {
			account.Operations = cloneOperationMap(account.Operations)
		}
		result = append(result, account)
	}
	return result
}

func cloneOperationMap(input map[string]float64) map[string]float64 {
	result := make(map[string]float64, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		if key != "" {
			result[key] = value
		}
	}
	return result
}

func sourceCounter(ctx context.Context, source func(context.Context) (uint64, error), local uint64) (uint64, error) {
	if source == nil {
		return local, nil
	}
	value, err := source(ctx)
	if err != nil {
		return 0, err
	}
	return value + local, nil
}

func normalizeMethod(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "UNKNOWN"
	}
	return value
}

func normalizeRoute(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return strings.ReplaceAll(value, "\n", `\n`)
}

func formatFloat(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "0"
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func countAt(values []uint64, index int) uint64 {
	if index < 0 || index >= len(values) {
		return 0
	}
	return values[index]
}
