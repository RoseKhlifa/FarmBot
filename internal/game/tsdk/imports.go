package tsdk

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

const tqosURL = "https://api.anticheatexpert.com/tqos"
const serverTimeURL = "https://api.anticheatexpert.com/test"

// DeviceInfo supplies the values used by the official device string import.
type DeviceInfo struct {
	Model    string
	Platform string
	System   string
	Brand    string
}

func (r *Runtime) instantiateImports(ctx context.Context, wazeroRuntime wazero.Runtime) error {
	builder := wazeroRuntime.NewHostModuleBuilder("a")
	builder.NewFunctionBuilder().WithFunc(r.importAssertion).Export("a")
	builder.NewFunctionBuilder().WithFunc(r.importWriteFile).Export("b")
	builder.NewFunctionBuilder().WithFunc(r.importStack).Export("c")
	builder.NewFunctionBuilder().WithFunc(r.importVersion).Export("d")
	builder.NewFunctionBuilder().WithFunc(r.importACEVM).Export("e")
	builder.NewFunctionBuilder().WithFunc(r.importSensors).Export("f")
	builder.NewFunctionBuilder().WithFunc(r.importReadFile).Export("g")
	builder.NewFunctionBuilder().WithFunc(r.importClock).Export("h")
	builder.NewFunctionBuilder().WithFunc(r.importDataPath).Export("i")
	builder.NewFunctionBuilder().WithFunc(r.importDevice).Export("j")
	builder.NewFunctionBuilder().WithFunc(r.importRuntimeTable).Export("k")
	builder.NewFunctionBuilder().WithFunc(r.importDebugState).Export("l")
	builder.NewFunctionBuilder().WithFunc(r.importAppID).Export("m")
	builder.NewFunctionBuilder().WithFunc(r.importGameID).Export("n")
	builder.NewFunctionBuilder().WithFunc(r.importFunctionIntegrity).Export("o")
	builder.NewFunctionBuilder().WithFunc(r.importFileStat).Export("p")
	builder.NewFunctionBuilder().WithFunc(r.importServerTime).Export("q")
	builder.NewFunctionBuilder().WithFunc(r.importMemoryGrowthFailure).Export("r")
	builder.NewFunctionBuilder().WithFunc(r.importEpochMillis).Export("s")
	builder.NewFunctionBuilder().WithFunc(r.importAppendFile).Export("t")
	builder.NewFunctionBuilder().WithFunc(r.importAbort).Export("u")
	builder.NewFunctionBuilder().WithFunc(r.importTQOS).Export("v")
	_, err := builder.Instantiate(ctx)
	return err
}

func (r *Runtime) importAssertion(_ context.Context, module api.Module, expression, file, line, function uint32) {
	expr := r.readCString(module, expression)
	fileName := r.readOptionalCString(module, file)
	functionName := r.readOptionalCString(module, function)
	panic(fmt.Sprintf("TSDK assertion: %s at %s:%d %s", expr, fileName, line, functionName))
}

func (r *Runtime) importWriteFile(_ context.Context, module api.Module, filePtr, dataPtr, encodingPtr uint32) uint32 {
	fileName, ok := r.tryReadCString(module, filePtr)
	if !ok {
		return 0
	}
	data, ok := r.tryReadCString(module, dataPtr)
	if !ok {
		return 0
	}
	encodingName, ok := r.tryReadCString(module, encodingPtr)
	if !ok {
		return 0
	}
	decoded, err := decodeText(data, encodingName)
	if err != nil {
		r.warnOnce("file-write-encoding", "TSDK file write encoding rejected: "+err.Error())
		return 0
	}
	target, err := r.resolveDataPath(fileName)
	if err != nil {
		r.warnOnce("file-write", "TSDK file path rejected: "+err.Error())
		return 0
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return 0
	}
	if err := os.WriteFile(target, decoded, 0o600); err != nil {
		r.warnOnce("file-write", "TSDK file write unavailable: "+err.Error())
		return 0
	}
	return 1
}

func (r *Runtime) importStack(_ context.Context, module api.Module, ptr, capacity uint32) uint32 {
	stack := nodeStackTrace()
	if !r.writeCString(module, stack, ptr, capacity) {
		return 0
	}
	return uint32(len([]byte(stack)) + 1)
}

func nodeStackTrace() string {
	// The original Node host exposes Error().stack. Passing Go's raw
	// "goroutine ..." format into the SDK makes its Linux device path parser
	// walk the wrong frame layout and trap in WASM, so normalize it to the
	// familiar V8 form before handing it over.
	raw := strings.Split(strings.TrimSpace(string(debug.Stack())), "\n")
	lines := []string{"Error: TSDK stack capture"}
	for i := 1; i+1 < len(raw); i += 2 {
		function := strings.TrimSpace(raw[i])
		location := strings.TrimSpace(raw[i+1])
		if function == "" || location == "" {
			continue
		}
		lines = append(lines, "    at "+function+" ("+location+")")
		if len(strings.Join(lines, "\n"))+1 >= 768 {
			break
		}
	}
	if len(lines) == 1 {
		lines = append(lines, "    at TsdkRuntime.init (tsdk-runtime.js:1:1)")
	}
	return strings.Join(lines, "\n") + "\n"
}

func (r *Runtime) importVersion(_ context.Context, module api.Module, ptr, capacity uint32) uint32 {
	return r.writeStringResult(module, OfficialVersion, ptr, capacity)
}

func (r *Runtime) importACEVM(_ context.Context, _ api.Module, _ uint32) uint32 {
	r.warnOnce("acevm", "TSDK JS integrity VM is unavailable in Go; using the official empty-result downgrade.")
	return 0
}

func (r *Runtime) importSensors(_ context.Context, _ api.Module) {
	r.warnOnce("sensors", "TSDK touch and gyroscope input is unavailable in Go.")
}

func (r *Runtime) importReadFile(_ context.Context, module api.Module, filePtr, outPtr, capacity, encodingPtr uint32) uint32 {
	fileName, ok := r.tryReadCString(module, filePtr)
	if !ok {
		return 0
	}
	encodingName, ok := r.tryReadCString(module, encodingPtr)
	if !ok {
		return 0
	}
	target, err := r.resolveDataPath(fileName)
	if err != nil {
		return 0
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return 0
	}
	value := string(data)
	if strings.EqualFold(encodingName, "base64") {
		value = base64.StdEncoding.EncodeToString(data)
	} else if strings.EqualFold(encodingName, "hex") {
		value = hex.EncodeToString(data)
	}
	if !r.writeCString(module, value, outPtr, capacity) {
		return 0
	}
	return outPtr
}

func (r *Runtime) importClock(_ context.Context, module api.Module, clockID, low, high, outPtr uint32) uint32 {
	_ = low
	_ = high
	if clockID > 3 {
		return 28
	}
	var nanos int64
	if clockID == 0 {
		nanos = r.wallTime().UnixNano()
	} else {
		nanos = r.monotonicTime().Nanoseconds()
	}
	if !r.writeUint64(module, outPtr, uint64(nanos)) {
		return 28
	}
	return 0
}

func (r *Runtime) importDataPath(_ context.Context, module api.Module, ptr, capacity uint32) uint32 {
	return r.writeStringResult(module, r.dataDir+string(filepath.Separator), ptr, capacity)
}

func (r *Runtime) importDevice(_ context.Context, module api.Module, ptr, capacity uint32) uint32 {
	return r.writeStringResult(module, r.deviceString(), ptr, capacity)
}

func (r *Runtime) importRuntimeTable(_ context.Context, module api.Module, ptr, capacity uint32) uint32 {
	if len(officialRuntimeTable) > int(capacity) || !r.writeBytes(module, ptr, officialRuntimeTable) {
		return 0
	}
	return uint32(len(officialRuntimeTable))
}

func (r *Runtime) importDebugState(_ context.Context, _ api.Module) uint32 { return 2 }

func (r *Runtime) importAppID(_ context.Context, module api.Module, ptr, capacity uint32) uint32 {
	return r.writeStringResult(module, r.appID, ptr, capacity)
}

func (r *Runtime) importGameID(_ context.Context, module api.Module, ptr, capacity uint32) uint32 {
	return r.writeStringResult(module, DefaultAppID, ptr, capacity)
}

func (r *Runtime) importFunctionIntegrity(_ context.Context, _ api.Module, _, _, _, _ uint32) {
	r.warnOnce("integrity-functions", "TSDK mini-program function integrity checks are unavailable in Go.")
}

func (r *Runtime) importFileStat(ctx context.Context, module api.Module, filePtr uint32) uint32 {
	fileName, ok := r.tryReadCString(module, filePtr)
	if !ok {
		return 0
	}
	target, err := r.resolveDataPath(fileName)
	if err != nil {
		return 0
	}
	info, err := os.Stat(target)
	if err != nil || r.module == nil {
		return 0
	}
	// Node's fs.Stats.mode includes the POSIX file-type bits in addition to
	// permissions. Preserve those bits so the WASM stat object has the same
	// shape when it reads an existing TSDK state file.
	mode := uint32(info.Mode().Perm())
	switch {
	case info.IsDir():
		mode |= 0o040000
	case info.Mode().IsRegular():
		mode |= 0o100000
	case info.Mode()&os.ModeSymlink != 0:
		mode |= 0o120000
	}
	size := uint32(info.Size())
	if info.Size() > 0x7fffffff {
		size = 0x7fffffff
	}
	modified := uint32(info.ModTime().UnixMilli())
	result, err := module.ExportedFunction("y").Call(ctx, uint64(mode), uint64(size), uint64(modified), uint64(modified))
	if err != nil || len(result) == 0 {
		return 0
	}
	return uint32(result[0])
}

func (r *Runtime) importServerTime(_ context.Context, module api.Module, outPtr uint32) uint32 {
	if !r.writeUint32(module, outPtr, uint32(r.wallTime().Unix())) {
		return 0
	}
	generation := r.serverTimeGeneration.Add(1)
	go r.calibrateServerTime(generation, module, outPtr)
	return 1
}

func (r *Runtime) importMemoryGrowthFailure(_ context.Context, _ api.Module, size uint32) uint32 {
	panic(fmt.Sprintf("TSDK cannot grow memory to %d bytes", size))
}

func (r *Runtime) importEpochMillis(_ context.Context, _ api.Module) float64 {
	return float64(r.wallTime().UnixMilli())
}

func (r *Runtime) importAppendFile(_ context.Context, module api.Module, filePtr, dataPtr, encodingPtr uint32) uint32 {
	fileName, ok := r.tryReadCString(module, filePtr)
	if !ok {
		return 0
	}
	data, ok := r.tryReadCString(module, dataPtr)
	if !ok {
		return 0
	}
	encodingName, ok := r.tryReadCString(module, encodingPtr)
	if !ok {
		return 0
	}
	decoded, err := decodeText(data, encodingName)
	if err != nil {
		return 0
	}
	target, err := r.resolveDataPath(fileName)
	if err != nil || os.MkdirAll(filepath.Dir(target), 0o700) != nil {
		return 0
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Write(decoded); err != nil {
		return 0
	}
	return 1
}

func (r *Runtime) importAbort(_ context.Context, _ api.Module) {
	panic("TSDK aborted")
}

func (r *Runtime) importTQOS(_ context.Context, module api.Module, ptr, length uint32) uint32 {
	data, ok := r.readBytes(module, ptr, length)
	if !ok {
		return 0
	}
	var report struct {
		Message any               `json:"message"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		r.warnOnce("tqos", "TSDK TQOS report rejected: "+err.Error())
		return 0
	}
	body, err := json.Marshal(report.Message)
	if message, ok := report.Message.(string); ok {
		body = []byte(message)
	}
	if err != nil {
		return 0
	}
	go r.postTQOS(body, report.Headers)
	return 1
}

func (r *Runtime) calibrateServerTime(generation uint64, module api.Module, outPtr uint32) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, serverTimeURL, nil)
	if err != nil {
		return
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return
	}
	defer func() { _ = response.Body.Close() }()
	dateHeader := response.Header.Get("Date")
	parsed, err := http.ParseTime(dateHeader)
	if err != nil || generation != r.serverTimeGeneration.Load() || r.destroyed.Load() {
		return
	}
	_ = r.writeUint32(module, outPtr, uint32(parsed.Unix()))
}

func (r *Runtime) postTQOS(body []byte, headers map[string]string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tqosURL, strings.NewReader(string(body)))
	if err != nil {
		return
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		r.warnOnce("tqos", "TSDK TQOS report failed: "+err.Error())
		return
	}
	_ = response.Body.Close()
}

func decodeText(value, encodingName string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encodingName)) {
	case "", "utf8", "utf-8":
		return []byte(value), nil
	case "base64":
		return base64.StdEncoding.DecodeString(value)
	case "hex":
		return hex.DecodeString(value)
	default:
		return nil, fmt.Errorf("unsupported encoding %q", encodingName)
	}
}

func (r *Runtime) deviceString() string {
	model := r.device.Model
	if model == "" {
		model = nodeOSName() + " " + nodeArchName()
	}
	platform := r.device.Platform
	if platform == "" {
		platform = nodePlatformName()
	}
	system := r.device.System
	if system == "" {
		system = nodeSystemName()
	}
	brand := r.device.Brand
	if brand == "" {
		// Keep the host identity compatible with the original Node wrapper. The
		// SDK only consumes this as a semicolon-delimited device descriptor.
		brand = "Node.js"
	}
	return model + ";" + platform + ";" + system + ";" + brand + ";"
}

func nodeOSName() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows_NT"
	case "darwin":
		return "Darwin"
	case "linux":
		return "Linux"
	default:
		return runtime.GOOS
	}
}

func nodePlatformName() string {
	if runtime.GOOS == "windows" {
		return "win32"
	}
	return runtime.GOOS
}

func nodeArchName() string {
	if runtime.GOARCH == "amd64" {
		return "x64"
	}
	return runtime.GOARCH
}

func nodeSystemName() string {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
			if value := strings.TrimSpace(string(data)); value != "" {
				return value
			}
		}
	}
	return runtime.Version()
}

func (r *Runtime) resolveDataPath(input string) (string, error) {
	value := strings.ReplaceAll(input, "\\", "/")
	value = strings.TrimLeft(value, "/")
	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("TSDK file path escaped its account data directory")
	}
	root, err := filepath.Abs(r.dataDir)
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, clean)
	rootWithSep := root + string(filepath.Separator)
	if target != root && !strings.HasPrefix(target, rootWithSep) {
		return "", fmt.Errorf("TSDK file path escaped its account data directory")
	}
	return target, nil
}

func (r *Runtime) warnOnce(key, message string) {
	r.warnMu.Lock()
	defer r.warnMu.Unlock()
	if r.warned == nil {
		r.warned = make(map[string]struct{})
	}
	if _, exists := r.warned[key]; exists {
		return
	}
	r.warned[key] = struct{}{}
	if r.logger != nil {
		r.logger("warn", message)
	}
}

func (r *Runtime) writeStringResult(module api.Module, value string, ptr, capacity uint32) uint32 {
	if !r.writeCString(module, value, ptr, capacity) {
		return 0
	}
	return ptr
}

func (r *Runtime) readOptionalCString(module api.Module, ptr uint32) string {
	if ptr == 0 {
		return "unknown"
	}
	return r.readCString(module, ptr)
}

func (r *Runtime) tryReadCString(module api.Module, ptr uint32) (string, bool) {
	if ptr == 0 {
		return "", false
	}
	value, ok := r.readCStringOK(module, ptr)
	return value, ok
}

func (r *Runtime) readCString(module api.Module, ptr uint32) string {
	value, ok := r.readCStringOK(module, ptr)
	if !ok {
		panic(fmt.Sprintf("TSDK string pointer out of bounds: %d", ptr))
	}
	return value
}

func (r *Runtime) readCStringOK(module api.Module, ptr uint32) (string, bool) {
	memory := module.Memory()
	if memory == nil || ptr >= memory.Size() {
		return "", false
	}
	max := memory.Size() - ptr
	if max > 1024*1024 {
		max = 1024 * 1024
	}
	data, ok := memory.Read(ptr, max)
	if !ok {
		return "", false
	}
	for i, value := range data {
		if value == 0 {
			return string(data[:i]), true
		}
	}
	return "", false
}

func (r *Runtime) readBytes(module api.Module, ptr, length uint32) ([]byte, bool) {
	memory := module.Memory()
	if memory == nil || length > memory.Size() || ptr > memory.Size()-length {
		return nil, false
	}
	data, ok := memory.Read(ptr, length)
	if !ok {
		return nil, false
	}
	copyOfData := make([]byte, len(data))
	copy(copyOfData, data)
	return copyOfData, true
}

func (r *Runtime) writeBytes(module api.Module, ptr uint32, data []byte) bool {
	memory := module.Memory()
	if memory == nil || uint32(len(data)) > memory.Size() || ptr > memory.Size()-uint32(len(data)) {
		return false
	}
	return memory.Write(ptr, data)
}

func (r *Runtime) writeCString(module api.Module, value string, ptr, capacity uint32) bool {
	data := append([]byte(value), 0)
	return uint32(len(data)) <= capacity && r.writeBytes(module, ptr, data)
}

func (r *Runtime) writeUint32(module api.Module, ptr, value uint32) bool {
	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], value)
	return r.writeBytes(module, ptr, data[:])
}

func (r *Runtime) writeUint64(module api.Module, ptr uint32, value uint64) bool {
	var data [8]byte
	binary.LittleEndian.PutUint64(data[:], value)
	return r.writeBytes(module, ptr, data[:])
}
