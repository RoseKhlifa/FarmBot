package tsdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RoseKhlifa/FarmBot/assets"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

var officialRuntimeTable = []byte{
	93, 86, 110, 34, 65, 129, 8, 113, 53, 192, 121, 32, 86, 162, 255, 139,
	217, 70, 223, 0, 45, 176, 85, 103, 234, 116, 120, 194, 206, 7, 176, 222,
	56, 6, 161, 159, 154, 231, 93, 229, 39, 107, 197, 136, 167, 52, 155, 228,
	209, 117, 218, 8, 107, 241, 32, 62, 53, 200, 238,
}

var optionalRuntimeExports = []string{"E", "H", "K", "P"}

// Logger receives lifecycle and downgrade messages. It is deliberately a
// small callback so account.Runtime can bridge it to platform/logger without
// making TSDK depend on the application composition root.
type Logger func(level, message string)

// Options configures one isolated TSDK instance.
type Options struct {
	AccountID string
	AppID     string
	GameID    uint32
	AppKey    string
	// Enabled overrides FARM_TSDK_ACE_ENABLED when non-nil. A nil value uses
	// the environment default, which is enabled.
	Enabled       *bool
	DataDir       string
	WASMName      string
	WASMPath      string
	WASMData      []byte
	Device        DeviceInfo
	Logger        Logger
	WallTime      func() time.Time
	MonotonicTime func() time.Duration
}

// Runtime owns exactly one wazero module and all buffers associated with it.
// A Runtime must never be shared by multiple accounts.
type Runtime struct {
	accountID     string
	appID         string
	gameID        uint32
	appKey        string
	enabled       bool
	dataDir       string
	wasmName      string
	wasmPath      string
	wasmData      []byte
	device        DeviceInfo
	logger        Logger
	wallTime      func() time.Time
	monotonicTime func() time.Duration

	initMu    sync.Mutex
	metricsMu sync.Mutex
	warnMu    sync.Mutex
	warned    map[string]struct{}

	wazeroRuntime wazero.Runtime
	compiled      wazero.CompiledModule
	module        api.Module
	memory        api.Memory
	exports       map[string]api.Function

	ready                atomic.Bool
	destroyed            atomic.Bool
	serverTimeGeneration atomic.Uint64
	metrics              Metrics
}

// Metrics contains counters useful for health and diagnostics.
type Metrics struct {
	InitializedAt   int64
	TokenCount      uint64
	LastTokenLength int
	LastTokenAt     int64
	HeartbeatTicks  uint64
	ProcessTicks    uint64
	StatusReports   uint64
	FunctionChecks  uint64
	UserBound       bool
	LastError       string
}

// Status is the public runtime snapshot.
type Status struct {
	AccountID  string
	Version    string
	WASMSHA256 string
	Enabled    bool
	Ready      bool
	Destroyed  bool
	Metrics
}

// TokenResult is the normalized result of the official aa export. Current
// production builds return a token string; older builds return JSON with an
// optional nonce, so both forms are preserved.
type TokenResult struct {
	Token string
	Nonce string
}

// New creates an uninitialized runtime. Call Init before using crypto APIs.
func New(ctx context.Context, options Options) *Runtime {
	if ctx == nil {
		ctx = context.Background()
	}
	_ = ctx // retained in the constructor signature for composition-root clarity
	if options.AccountID == "" {
		options.AccountID = os.Getenv("FARM_ACCOUNT_ID")
		if options.AccountID == "" {
			options.AccountID = "unknown"
		}
	}
	if options.AppID == "" {
		options.AppID = DefaultAppID
	}
	if options.GameID == 0 {
		options.GameID = envUint32("FARM_TSDK_GAME_ID", DefaultGameID)
	}
	if options.AppKey == "" {
		options.AppKey = os.Getenv("FARM_TSDK_APP_KEY")
		if options.AppKey == "" {
			options.AppKey = DefaultAppKey
		}
	}
	if options.WASMName == "" {
		options.WASMName = "tsdk-v3.8.2.wasm"
	}
	if options.DataDir == "" {
		base := os.Getenv("FARM_DATA_DIR")
		if base == "" {
			base = "data"
		}
		options.DataDir = filepath.Join(base, "tsdk", options.AccountID)
	}
	wallTime := options.WallTime
	if wallTime == nil {
		wallTime = time.Now
	}
	startedAt := time.Now()
	monotonicTime := options.MonotonicTime
	if monotonicTime == nil {
		monotonicTime = func() time.Duration { return time.Since(startedAt) }
	}
	enabled := envBool("FARM_TSDK_ACE_ENABLED", true)
	if options.Enabled != nil {
		enabled = *options.Enabled
	}
	return &Runtime{
		accountID:     options.AccountID,
		appID:         options.AppID,
		gameID:        options.GameID,
		appKey:        options.AppKey,
		enabled:       enabled,
		dataDir:       filepath.Clean(options.DataDir),
		wasmName:      options.WASMName,
		wasmPath:      options.WASMPath,
		wasmData:      append([]byte(nil), options.WASMData...),
		device:        options.Device,
		logger:        options.Logger,
		wallTime:      wallTime,
		monotonicTime: monotonicTime,
		warned:        make(map[string]struct{}),
	}
}

// NewWithOptions is a context-free convenience constructor.
func NewWithOptions(options Options) *Runtime {
	return New(context.Background(), options)
}

// NewRuntime is an explicit alias for callers that prefer the type name.
func NewRuntime(ctx context.Context, options Options) *Runtime {
	return New(ctx, options)
}

// TsdkRuntime preserves the spelling used by the Node migration notes.
type TsdkRuntime = Runtime

// Init verifies, compiles, instantiates, decrypts and initializes the official
// TSDK in the exact order required by the Node wrapper.
func (r *Runtime) Init(ctx context.Context) error {
	r.initMu.Lock()
	defer r.initMu.Unlock()
	if r.ready.Load() {
		return nil
	}
	if r.destroyed.Load() {
		return errors.New("TSDK runtime has been destroyed")
	}
	if !r.enabled {
		return r.initFailure(errors.New("TSDK runtime disabled by FARM_TSDK_ACE_ENABLED=false"))
	}
	if ctx == nil {
		ctx = context.Background()
	}

	wasm, err := r.loadWASM()
	if err != nil {
		return r.initFailure(err)
	}
	if err := VerifyWASM(wasm); err != nil {
		return r.initFailure(err)
	}
	if err := os.MkdirAll(r.dataDir, 0o700); err != nil {
		return r.initFailure(fmt.Errorf("create TSDK data directory: %w", err))
	}

	wazeroRuntime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigCompiler().WithCloseOnContextDone(true))
	compiled, err := wazeroRuntime.CompileModule(ctx, wasm)
	if err != nil {
		_ = wazeroRuntime.Close(ctx)
		return r.initFailure(fmt.Errorf("compile TSDK WASM: %w", err))
	}
	if err := VerifyCompiledModule(compiled); err != nil {
		_ = compiled.Close(ctx)
		_ = wazeroRuntime.Close(ctx)
		return r.initFailure(err)
	}
	if err := r.instantiateImports(ctx, wazeroRuntime); err != nil {
		_ = compiled.Close(ctx)
		_ = wazeroRuntime.Close(ctx)
		return r.initFailure(fmt.Errorf("instantiate TSDK imports: %w", err))
	}
	module, err := wazeroRuntime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName("farmbot-tsdk"))
	if err != nil {
		_ = compiled.Close(ctx)
		_ = wazeroRuntime.Close(ctx)
		return r.initFailure(fmt.Errorf("instantiate TSDK WASM: %w", err))
	}

	r.wazeroRuntime = wazeroRuntime
	r.compiled = compiled
	r.module = module
	r.memory = module.Memory()
	r.exports = make(map[string]api.Function)
	for _, symbol := range requiredExports {
		if symbol != "w" {
			r.exports[symbol] = module.ExportedFunction(symbol)
		}
	}
	for _, symbol := range optionalRuntimeExports {
		if function := module.ExportedFunction(symbol); function != nil {
			r.exports[symbol] = function
		}
	}
	if r.memory == nil {
		return r.initFailure(errors.New("TSDK exported memory w is missing"))
	}

	decryptor := module.ExportedFunction("__mergewasm_shared____wasm_decrypt_strings")
	for _, segment := range mergedDataSegments {
		if !r.inBounds(segment[0], segment[1]) {
			return r.initFailure(fmt.Errorf("TSDK merged-data segment out of bounds: ptr=%d length=%d", segment[0], segment[1]))
		}
		if _, err := decryptor.Call(ctx, uint64(segment[0]), uint64(segment[1]), uint64(mergedDataKey)); err != nil {
			return r.initFailure(fmt.Errorf("decrypt TSDK merged data: %w", err))
		}
	}
	if constructors := module.ExportedFunction("__wasm_call_ctors"); constructors != nil {
		if _, err := constructors.Call(ctx); err != nil {
			return r.initFailure(fmt.Errorf("call TSDK constructors: %w", err))
		}
	}

	keyPtr, err := r.allocCString(ctx, r.appKey)
	if err != nil {
		return r.initFailure(err)
	}
	defer r.free(ctx, keyPtr)
	if _, err := r.exports["G"].Call(ctx, uint64(r.gameID), uint64(keyPtr)); err != nil {
		return r.initFailure(fmt.Errorf("initialize TSDK runtime: %w", err))
	}

	r.ready.Store(true)
	r.metricsMu.Lock()
	r.metrics.InitializedAt = time.Now().UnixMilli()
	r.metrics.LastError = ""
	r.metricsMu.Unlock()
	r.log("info", fmt.Sprintf("TSDK initialized: %s", OfficialVersion))
	return nil
}

func (r *Runtime) initFailure(err error) error {
	r.metricsMu.Lock()
	r.metrics.LastError = err.Error()
	r.metricsMu.Unlock()
	r.log("error", "TSDK initialization failed: "+err.Error())
	return err
}

func (r *Runtime) loadWASM() ([]byte, error) {
	if len(r.wasmData) > 0 {
		return append([]byte(nil), r.wasmData...), nil
	}
	if r.wasmPath != "" {
		return os.ReadFile(r.wasmPath)
	}
	return assets.ReadWASM(r.wasmName)
}

// Destroy releases the module and compiler resources. It is safe to call more
// than once and invalidates all subsequent crypto calls.
func (r *Runtime) Destroy() error {
	r.initMu.Lock()
	defer r.initMu.Unlock()
	if r.destroyed.Swap(true) {
		return nil
	}
	r.ready.Store(false)
	r.serverTimeGeneration.Add(1)
	ctx := context.Background()
	var firstErr error
	if r.module != nil {
		if err := r.module.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if r.compiled != nil {
		if err := r.compiled.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if r.wazeroRuntime != nil {
		if err := r.wazeroRuntime.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	r.module = nil
	r.memory = nil
	r.exports = nil
	r.compiled = nil
	r.wazeroRuntime = nil
	return firstErr
}

// Close implements the conventional resource lifecycle API.
func (r *Runtime) Close(_ context.Context) error { return r.Destroy() }

func (r *Runtime) assertReady() error {
	if r.destroyed.Load() || !r.ready.Load() || r.module == nil {
		return errors.New("TSDK runtime is not ready")
	}
	return nil
}

func (r *Runtime) alloc(ctx context.Context, length uint32) (uint32, error) {
	if length == 0 {
		length = 1
	}
	result, err := r.exports["A"].Call(ctx, uint64(length))
	if err != nil {
		return 0, fmt.Errorf("allocate TSDK buffer: %w", err)
	}
	if len(result) == 0 || uint32(result[0]) == 0 {
		return 0, fmt.Errorf("TSDK failed to allocate %d bytes", length)
	}
	ptr := uint32(result[0])
	if !r.inBounds(ptr, length) {
		return 0, fmt.Errorf("TSDK allocation out of bounds: ptr=%d length=%d", ptr, length)
	}
	return ptr, nil
}

func (r *Runtime) allocBytes(ctx context.Context, value []byte) (uint32, uint32, error) {
	length := uint32(len(value))
	ptr, err := r.alloc(ctx, length)
	if err != nil {
		return 0, 0, err
	}
	if len(value) > 0 && !r.memory.Write(ptr, value) {
		r.free(ctx, ptr)
		return 0, 0, fmt.Errorf("write TSDK buffer out of bounds: ptr=%d length=%d", ptr, length)
	}
	return ptr, length, nil
}

func (r *Runtime) allocCString(ctx context.Context, value string) (uint32, error) {
	data := append([]byte(value), 0)
	ptr, _, err := r.allocBytes(ctx, data)
	return ptr, err
}

func (r *Runtime) free(ctx context.Context, ptr uint32) {
	if ptr == 0 || r.exports == nil || r.exports["B"] == nil {
		return
	}
	_, _ = r.exports["B"].Call(ctx, uint64(ptr))
}

func (r *Runtime) call(ctx context.Context, symbol string, args ...uint64) error {
	function := r.exports[symbol]
	if function == nil {
		return fmt.Errorf("TSDK export %s is unavailable", symbol)
	}
	if _, err := function.Call(ctx, args...); err != nil {
		return fmt.Errorf("call TSDK export %s: %w", symbol, err)
	}
	return nil
}

// Encrypt applies the official in-place ba export.
func (r *Runtime) Encrypt(value []byte) ([]byte, error) {
	return r.transformInPlace(context.Background(), value, "ba")
}

// Decrypt applies the official in-place ca export.
func (r *Runtime) Decrypt(value []byte) ([]byte, error) {
	return r.transformInPlace(context.Background(), value, "ca")
}

func (r *Runtime) transformInPlace(ctx context.Context, value []byte, symbol string) ([]byte, error) {
	if err := r.assertReady(); err != nil {
		return nil, err
	}
	ptr, length, err := r.allocBytes(ctx, value)
	if err != nil {
		return nil, err
	}
	defer r.free(ctx, ptr)
	if err := r.call(ctx, symbol, uint64(ptr), uint64(length)); err != nil {
		return nil, err
	}
	data, ok := r.memory.Read(ptr, length)
	if !ok {
		return nil, fmt.Errorf("read TSDK transformed buffer out of bounds: ptr=%d length=%d", ptr, length)
	}
	return append([]byte(nil), data...), nil
}

// EncryptV2 applies the caller-buffer encrypt v2 export.
func (r *Runtime) EncryptV2(value []byte) ([]byte, error) {
	return r.transformV2(context.Background(), value, "da")
}

// DecryptV2 applies the caller-buffer decrypt v2 export.
func (r *Runtime) DecryptV2(value []byte) ([]byte, error) {
	return r.transformV2(context.Background(), value, "ea")
}

func (r *Runtime) transformV2(ctx context.Context, value []byte, symbol string) ([]byte, error) {
	if err := r.assertReady(); err != nil {
		return nil, err
	}
	inputPtr, inputLength, err := r.allocBytes(ctx, value)
	if err != nil {
		return nil, err
	}
	defer r.free(ctx, inputPtr)
	capacity := uint32(len(value)*2 + 64)
	outputPtr, err := r.alloc(ctx, capacity)
	if err != nil {
		return nil, err
	}
	defer r.free(ctx, outputPtr)
	result, err := r.exports[symbol].Call(ctx, uint64(inputPtr), uint64(inputLength), uint64(outputPtr), uint64(capacity))
	if err != nil {
		return nil, fmt.Errorf("call TSDK export %s: %w", symbol, err)
	}
	if len(result) == 0 || uint32(result[0]) > capacity {
		return nil, fmt.Errorf("TSDK export %s returned invalid length", symbol)
	}
	length := uint32(result[0])
	data, ok := r.memory.Read(outputPtr, length)
	if !ok {
		return nil, fmt.Errorf("read TSDK v2 output out of bounds")
	}
	return append([]byte(nil), data...), nil
}

// GenerateToken invokes aa and copies the returned WASM-owned string before
// releasing both the input and return buffers.
func (r *Runtime) GenerateToken(value []byte) (TokenResult, error) {
	if err := r.assertReady(); err != nil {
		return TokenResult{}, err
	}
	ctx := context.Background()
	inputPtr, inputLength, err := r.allocBytes(ctx, value)
	if err != nil {
		return TokenResult{}, err
	}
	defer r.free(ctx, inputPtr)
	result, err := r.exports["aa"].Call(ctx, uint64(inputPtr), uint64(inputLength))
	if err != nil {
		return TokenResult{}, fmt.Errorf("generate TSDK token: %w", err)
	}
	if len(result) == 0 || uint32(result[0]) == 0 {
		return TokenResult{}, errors.New("TSDK returned an empty token pointer")
	}
	resultPtr := uint32(result[0])
	defer r.free(ctx, resultPtr)
	text, ok := r.readCStringOK(r.module, resultPtr)
	if !ok {
		return TokenResult{}, errors.New("TSDK returned an invalid token string")
	}
	token := TokenResult{Token: text}
	if strings.HasPrefix(text, "{") {
		var parsed struct {
			Token string `json:"token"`
			Nonce string `json:"nonce"`
		}
		if err := json.Unmarshal([]byte(text), &parsed); err == nil {
			token = TokenResult{Token: parsed.Token, Nonce: parsed.Nonce}
		}
	}
	if token.Token == "" {
		return TokenResult{}, errors.New("TSDK returned an invalid token result")
	}
	r.metricsMu.Lock()
	r.metrics.TokenCount++
	r.metrics.LastTokenLength = len(token.Token)
	r.metrics.LastTokenAt = time.Now().UnixMilli()
	r.metricsMu.Unlock()
	return token, nil
}

// HeartbeatTick runs the M export.
func (r *Runtime) HeartbeatTick() error {
	if err := r.assertReady(); err != nil {
		return err
	}
	if err := r.call(context.Background(), "M"); err != nil {
		return err
	}
	r.metricsMu.Lock()
	r.metrics.HeartbeatTicks++
	r.metricsMu.Unlock()
	return nil
}

// ProcessReceivedData runs the P export.
func (r *Runtime) ProcessReceivedData() error {
	if err := r.assertReady(); err != nil {
		return err
	}
	if err := r.call(context.Background(), "P"); err != nil {
		return err
	}
	r.metricsMu.Lock()
	r.metrics.ProcessTicks++
	r.metricsMu.Unlock()
	return nil
}

// SendStatus runs the E export used by the status lifecycle.
func (r *Runtime) SendStatus() error {
	if err := r.assertReady(); err != nil {
		return err
	}
	if err := r.call(context.Background(), "E"); err != nil {
		return err
	}
	r.metricsMu.Lock()
	r.metrics.StatusReports++
	r.metricsMu.Unlock()
	return nil
}

// DetectSpeedHack runs the fa export when present in the official module.
func (r *Runtime) DetectSpeedHack(elapsed time.Duration) error {
	if err := r.assertReady(); err != nil {
		return err
	}
	function := r.module.ExportedFunction("fa")
	if function == nil {
		return nil
	}
	_, err := function.Call(context.Background(), uint64(maxDurationMillis(elapsed)))
	return err
}

// CheckFunctionArray mirrors the Node wrapper's string-pointer array layout.
func (r *Runtime) CheckFunctionArray(functions []string, functionType uint32) error {
	if err := r.assertReady(); err != nil {
		return err
	}
	values := make([]string, 0, len(functions))
	for _, value := range functions {
		if strings.TrimSpace(value) != "" {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return nil
	}
	ctx := context.Background()
	pointers := make([]uint32, len(values))
	for i, value := range values {
		ptr, err := r.allocCString(ctx, value)
		if err != nil {
			for _, allocated := range pointers[:i] {
				r.free(ctx, allocated)
			}
			return err
		}
		pointers[i] = ptr
	}
	defer func() {
		for _, ptr := range pointers {
			r.free(ctx, ptr)
		}
	}()
	arrayPtr, err := r.alloc(ctx, uint32(len(pointers)*4))
	if err != nil {
		return err
	}
	defer r.free(ctx, arrayPtr)
	for i, ptr := range pointers {
		if !r.memory.WriteUint32Le(arrayPtr+uint32(i*4), ptr) {
			return errors.New("TSDK function pointer array write out of bounds")
		}
	}
	if err := r.call(ctx, "K", uint64(arrayPtr), uint64(len(pointers)), uint64(functionType)); err != nil {
		return err
	}
	r.metricsMu.Lock()
	r.metrics.FunctionChecks++
	r.metricsMu.Unlock()
	return nil
}

// BindUser performs the official AnoUserLogin-compatible runtime refresh.
func (r *Runtime) BindUser(openID string) error {
	if err := r.assertReady(); err != nil {
		return err
	}
	openID = strings.TrimSpace(openID)
	if openID == "" {
		return nil
	}
	ctx := context.Background()
	ptr, err := r.allocCString(ctx, openID)
	if err != nil {
		return err
	}
	defer r.free(ctx, ptr)
	if _, err := r.exports["G"].Call(ctx, uint64(r.gameID), uint64(ptr)); err != nil {
		return err
	}
	r.metricsMu.Lock()
	r.metrics.UserBound = true
	r.metricsMu.Unlock()
	return nil
}

// GetEncryptedInitInfo copies the H export's C string. The returned pointer is
// owned by WASM according to the official ABI and is therefore not freed here.
func (r *Runtime) GetEncryptedInitInfo() (string, error) {
	if err := r.assertReady(); err != nil {
		return "", err
	}
	function := r.exports["H"]
	if function == nil {
		return "", errors.New("TSDK export H is unavailable")
	}
	result, err := function.Call(context.Background())
	if err != nil {
		return "", err
	}
	if len(result) == 0 || uint32(result[0]) == 0 {
		return "", nil
	}
	value, ok := r.readCStringOK(r.module, uint32(result[0]))
	if !ok {
		return "", errors.New("TSDK encrypted init info pointer is invalid")
	}
	return value, nil
}

// GetDataToServer copies the N export's WASM-owned payload. The length output
// buffer is caller-owned and is released after copying the payload.
func (r *Runtime) GetDataToServer() ([]byte, error) {
	if err := r.assertReady(); err != nil {
		return nil, err
	}
	ctx := context.Background()
	lengthPtr, err := r.alloc(ctx, 4)
	if err != nil {
		return nil, err
	}
	defer r.free(ctx, lengthPtr)
	if !r.memory.WriteUint32Le(lengthPtr, 0) {
		return nil, errors.New("TSDK data length write out of bounds")
	}
	result, err := r.exports["N"].Call(ctx, uint64(lengthPtr))
	if err != nil {
		return nil, err
	}
	length, ok := r.memory.ReadUint32Le(lengthPtr)
	if !ok || len(result) == 0 || uint32(result[0]) == 0 || length == 0 {
		return []byte{}, nil
	}
	dataPtr := uint32(result[0])
	data, ok := r.memory.Read(dataPtr, length)
	if !ok {
		return nil, fmt.Errorf("TSDK data-to-server pointer out of bounds: ptr=%d length=%d", dataPtr, length)
	}
	return append([]byte(nil), data...), nil
}

// SendDataFromServer passes a copied server payload to O and releases its
// caller-owned input buffer after the call returns.
func (r *Runtime) SendDataFromServer(value []byte) error {
	if err := r.assertReady(); err != nil {
		return err
	}
	ctx := context.Background()
	ptr, length, err := r.allocBytes(ctx, value)
	if err != nil {
		return err
	}
	defer r.free(ctx, ptr)
	return r.call(ctx, "O", uint64(ptr), uint64(length))
}

// GetResult copies the fixed 64-byte C result without freeing WASM-owned
// memory.
func (r *Runtime) GetResult() ([]byte, error) {
	if err := r.assertReady(); err != nil {
		return nil, err
	}
	result, err := r.exports["C"].Call(context.Background())
	if err != nil {
		return nil, err
	}
	if len(result) == 0 || uint32(result[0]) == 0 {
		return []byte{}, nil
	}
	data, ok := r.memory.Read(uint32(result[0]), 64)
	if !ok {
		return nil, errors.New("TSDK result pointer out of bounds")
	}
	return append([]byte(nil), data...), nil
}

// GetStatus returns a copy of the runtime state and counters.
func (r *Runtime) GetStatus() Status {
	r.metricsMu.Lock()
	metrics := r.metrics
	r.metricsMu.Unlock()
	return Status{
		AccountID:  r.accountID,
		Version:    OfficialVersion,
		WASMSHA256: OfficialSHA256,
		Enabled:    r.enabled,
		Ready:      r.ready.Load(),
		Destroyed:  r.destroyed.Load(),
		Metrics:    metrics,
	}
}

func (r *Runtime) inBounds(ptr, length uint32) bool {
	return r.memory != nil && ptr <= r.memory.Size() && length <= r.memory.Size()-ptr
}

func (r *Runtime) log(level, message string) {
	if r.logger != nil {
		r.logger(level, message)
	}
}

func maxDurationMillis(value time.Duration) uint32 {
	if value <= 0 {
		return 0
	}
	value /= time.Millisecond
	if value > time.Duration(^uint32(0)) {
		return ^uint32(0)
	}
	return uint32(value)
}

func envBool(name string, fallback bool) bool {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func envUint32(name string, fallback uint32) uint32 {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 32)
	if err != nil || parsed == 0 {
		return fallback
	}
	return uint32(parsed)
}
