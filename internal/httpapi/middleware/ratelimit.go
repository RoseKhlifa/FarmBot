package middleware

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	DefaultRateLimitRequests          = 120
	DefaultRateLimitWindow            = time.Minute
	DefaultRateLimitBurst             = 30
	DefaultSensitiveRateLimitRequests = 20
	DefaultSensitiveRateLimitBurst    = 5
	DefaultRateLimitMaxEntries        = 10000
)

// RateLimitConfig controls the limiter policy. RequestsPerSecond fields
// are convenience overrides; when set they are converted to the configured
// window. The window/count fields make deterministic low-rate test policies
// possible without introducing a third-party limiter dependency.
type RateLimitConfig struct {
	RequestsPerWindow int
	Window            time.Duration
	Burst             int

	SensitiveRequestsPerWindow int
	SensitiveWindow            time.Duration
	SensitiveBurst             int

	RequestsPerSecond          float64
	SensitiveRequestsPerSecond float64
	MaxEntries                 int
	Backend                    RateLimitBackend

	// SensitivePaths and SensitivePrefixes extend the built-in sensitive
	// endpoint set. Built-ins remain enabled when either field is supplied.
	SensitivePaths    []string
	SensitivePrefixes []string
}

// RateLimitBackend is the distributed-store seam for multi-instance
// deployments. Implementations should atomically check and charge every key
// in one call; the default configuration leaves it nil and uses memory.
type RateLimitBackend interface {
	Allow(keys []string, rate float64, burst int, now time.Time) (bool, time.Duration)
	Reset()
}

// DefaultRateLimitConfig returns the single-instance policy used when no
// configuration is supplied.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerWindow:          DefaultRateLimitRequests,
		Window:                     DefaultRateLimitWindow,
		Burst:                      DefaultRateLimitBurst,
		SensitiveRequestsPerWindow: DefaultSensitiveRateLimitRequests,
		SensitiveWindow:            DefaultRateLimitWindow,
		SensitiveBurst:             DefaultSensitiveRateLimitBurst,
		MaxEntries:                 DefaultRateLimitMaxEntries,
	}
}

type rateLimitBucket struct {
	tokens float64
	last   time.Time
}

// RateLimiter is a process-local token bucket keyed independently by client
// IP and authenticated token. A request must fit both buckets before either
// bucket is charged, so a rejected token check does not consume the IP quota.
type RateLimiter struct {
	mu      sync.Mutex
	config  RateLimitConfig
	buckets map[string]rateLimitBucket
	backend RateLimitBackend
}

// NewRateLimiter creates an in-memory limiter unless Backend is configured.
// The variadic form keeps the zero-argument constructor convenient while
// allowing tests and composition roots to inject one policy.
func NewRateLimiter(config ...RateLimitConfig) *RateLimiter {
	cfg := normalizeRateLimitConfig(RateLimitConfig{})
	if len(config) > 0 {
		cfg = normalizeRateLimitConfig(config[0])
	}
	return &RateLimiter{config: cfg, buckets: make(map[string]rateLimitBucket), backend: cfg.Backend}
}

// NewRateLimit is a compatibility alias for callers that prefer the shorter
// constructor name.
func NewRateLimit(config ...RateLimitConfig) *RateLimiter { return NewRateLimiter(config...) }

// NewRateLimiterWithBackend wires a distributed backend while retaining the
// same middleware and policy API used by the in-memory implementation.
func NewRateLimiterWithBackend(backend RateLimitBackend, config ...RateLimitConfig) *RateLimiter {
	cfg := RateLimitConfig{Backend: backend}
	if len(config) > 0 {
		cfg = config[0]
		cfg.Backend = backend
	}
	return NewRateLimiter(cfg)
}

// RateLimit returns a Gin middleware using a new limiter instance.
func RateLimit(config ...RateLimitConfig) gin.HandlerFunc {
	return NewRateLimiter(config...).Middleware()
}

// RateLimitMiddleware is an explicit alias for RateLimit.
func RateLimitMiddleware(config ...RateLimitConfig) gin.HandlerFunc {
	return RateLimit(config...)
}

// Middleware applies the limiter to a Gin request. Health probes bypass the
// limiter so orchestrators can still distinguish a live process from a busy
// API regardless of client traffic.
func (l *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c == nil || isHealthCheckPath(c.Request.URL.Path) {
			if c != nil {
				c.Next()
			}
			return
		}

		sensitive := l.isSensitivePath(c.Request.URL.Path)
		allowed, retryAfter := l.Allow(c.ClientIP(), c.GetHeader(AdminTokenHeader), sensitive)
		if allowed {
			c.Next()
			return
		}

		seconds := int(math.Ceil(retryAfter.Seconds()))
		if seconds < 1 {
			seconds = 1
		}
		c.Header("Retry-After", strconv.Itoa(seconds))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"ok":    false,
			"error": "Too Many Requests",
		})
	}
}

// Handler is an alias for Middleware for code that treats middleware as a
// handler factory.
func (l *RateLimiter) Handler() gin.HandlerFunc { return l.Middleware() }

// Allow atomically checks and charges the IP and token buckets. Empty tokens
// are ignored, which keeps unauthenticated public endpoints on the IP quota
// while authenticated traffic receives both dimensions.
func (l *RateLimiter) Allow(ip, token string, sensitive bool) (bool, time.Duration) {
	if l == nil {
		return true, 0
	}

	ip = normalizeRateLimitKey(ip, "unknown")
	token = strings.TrimSpace(token)
	now := time.Now()
	policy := l.policy(sensitive)
	keys := []string{"ip:" + policy.name + ":" + ip}
	if token != "" {
		keys = append(keys, "token:"+policy.name+":"+token)
	}
	if l.backend != nil {
		return l.backend.Allow(keys, policy.rate, policy.burst, now)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	updated := make(map[string]rateLimitBucket, len(keys))
	var retryAfter time.Duration
	for _, key := range keys {
		bucket := l.buckets[key]
		bucket = refillRateLimitBucket(bucket, now, policy.rate, policy.burst)
		updated[key] = bucket
		if bucket.tokens < 1 {
			wait := rateLimitWait(bucket.tokens, policy.rate)
			if wait > retryAfter {
				retryAfter = wait
			}
		}
	}

	if retryAfter > 0 {
		for key, bucket := range updated {
			l.buckets[key] = bucket
		}
		l.cleanup(now)
		return false, retryAfter
	}

	for key, bucket := range updated {
		bucket.tokens--
		l.buckets[key] = bucket
	}
	l.cleanup(now)
	return true, 0
}

// Reset clears all process-local state. It is useful for lifecycle shutdowns
// and isolated tests; it does not alter the configured policy.
func (l *RateLimiter) Reset() {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.buckets = make(map[string]rateLimitBucket)
	l.mu.Unlock()
	if l.backend != nil {
		l.backend.Reset()
	}
}

type rateLimitPolicy struct {
	name  string
	rate  float64
	burst int
}

func (l *RateLimiter) policy(sensitive bool) rateLimitPolicy {
	if sensitive {
		return rateLimitPolicy{
			name: "sensitive", rate: requestsPerSecond(l.config.SensitiveRequestsPerWindow, l.config.SensitiveWindow),
			burst: l.config.SensitiveBurst,
		}
	}
	return rateLimitPolicy{
		name: "normal", rate: requestsPerSecond(l.config.RequestsPerWindow, l.config.Window),
		burst: l.config.Burst,
	}
}

func refillRateLimitBucket(bucket rateLimitBucket, now time.Time, rate float64, burst int) rateLimitBucket {
	if bucket.last.IsZero() {
		bucket.last = now
		bucket.tokens = float64(burst)
		return bucket
	}
	if elapsed := now.Sub(bucket.last).Seconds(); elapsed > 0 {
		bucket.tokens = math.Min(float64(burst), bucket.tokens+elapsed*rate)
		bucket.last = now
	}
	return bucket
}

func rateLimitWait(tokens, rate float64) time.Duration {
	if rate <= 0 {
		return time.Second
	}
	missing := math.Max(0, 1-tokens)
	return time.Duration(math.Ceil(missing / rate * float64(time.Second)))
}

func requestsPerSecond(requests int, window time.Duration) float64 {
	if requests <= 0 || window <= 0 {
		return 1
	}
	return float64(requests) / window.Seconds()
}

func (l *RateLimiter) cleanup(now time.Time) {
	if len(l.buckets) <= l.config.MaxEntries {
		return
	}
	cutoff := now.Add(-2 * maxRateLimitWindow(l.config.Window, l.config.SensitiveWindow))
	for key, bucket := range l.buckets {
		if bucket.last.Before(cutoff) {
			delete(l.buckets, key)
		}
	}
	if len(l.buckets) <= l.config.MaxEntries {
		return
	}
	// If all buckets are active, retain the newest entries and discard the
	// oldest. This bounds memory without requiring a background cleanup loop.
	for len(l.buckets) > l.config.MaxEntries {
		var oldestKey string
		var oldest time.Time
		for key, bucket := range l.buckets {
			if oldestKey == "" || bucket.last.Before(oldest) {
				oldestKey, oldest = key, bucket.last
			}
		}
		delete(l.buckets, oldestKey)
	}
}

func maxRateLimitWindow(left, right time.Duration) time.Duration {
	if left > right {
		return left
	}
	return right
}

func normalizeRateLimitConfig(cfg RateLimitConfig) RateLimitConfig {
	defaults := DefaultRateLimitConfig()
	if cfg.Window <= 0 {
		cfg.Window = defaults.Window
	}
	if cfg.RequestsPerSecond > 0 {
		cfg.RequestsPerWindow = maxRateLimitRequests(1, int(math.Ceil(cfg.RequestsPerSecond*cfg.Window.Seconds())))
	}
	if cfg.RequestsPerWindow <= 0 {
		cfg.RequestsPerWindow = defaults.RequestsPerWindow
	}
	if cfg.Burst <= 0 {
		cfg.Burst = minRateLimitRequests(cfg.RequestsPerWindow, defaults.Burst)
	}
	if cfg.SensitiveWindow <= 0 {
		cfg.SensitiveWindow = cfg.Window
	}
	if cfg.SensitiveRequestsPerSecond > 0 {
		cfg.SensitiveRequestsPerWindow = maxRateLimitRequests(1, int(math.Ceil(cfg.SensitiveRequestsPerSecond*cfg.SensitiveWindow.Seconds())))
	}
	if cfg.SensitiveRequestsPerWindow <= 0 {
		cfg.SensitiveRequestsPerWindow = defaults.SensitiveRequestsPerWindow
	}
	if cfg.SensitiveBurst <= 0 {
		cfg.SensitiveBurst = minRateLimitRequests(cfg.SensitiveRequestsPerWindow, defaults.SensitiveBurst)
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = defaults.MaxEntries
	}
	cfg.SensitivePaths = append(append([]string(nil), defaultSensitivePaths...), cfg.SensitivePaths...)
	cfg.SensitivePrefixes = append(append([]string(nil), defaultSensitivePrefixes...), cfg.SensitivePrefixes...)
	return cfg
}

func maxRateLimitRequests(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func minRateLimitRequests(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func normalizeRateLimitKey(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

var defaultSensitivePaths = []string{
	"/api/login",
	"/api/register",
	"/api/card-claim/claim",
	"/api/public/renew",
	"/api/user/renew",
	"/api/qr/create",
	"/api/qr/check",
}

var defaultSensitivePrefixes = []string{
	"/api/admin/cards",
	"/api/admin/users",
	"/api/capture/sessions",
	"/api/super-admin",
	"/api/yyb",
}

// IsSensitivePath reports whether the default policy treats a path as
// sensitive. Custom policy paths are evaluated by the limiter instance.
func IsSensitivePath(path string) bool {
	return matchesSensitivePath(path, defaultSensitivePaths, defaultSensitivePrefixes)
}

func (l *RateLimiter) isSensitivePath(path string) bool {
	return matchesSensitivePath(path, l.config.SensitivePaths, l.config.SensitivePrefixes)
}

func matchesSensitivePath(path string, exact, prefixes []string) bool {
	path = strings.TrimSuffix(strings.TrimSpace(path), "/")
	for _, candidate := range exact {
		if path == strings.TrimSuffix(strings.TrimSpace(candidate), "/") {
			return true
		}
	}
	for _, prefix := range prefixes {
		prefix = strings.TrimSuffix(strings.TrimSpace(prefix), "/")
		if prefix != "" && (path == prefix || strings.HasPrefix(path, prefix+"/")) {
			return true
		}
	}
	return false
}

func isHealthCheckPath(path string) bool {
	path = strings.TrimSuffix(strings.TrimSpace(path), "/")
	return path == "/api/health" || path == "/health"
}
