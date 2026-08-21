package httpapi

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/config"
	"github.com/gin-gonic/gin"
)

const (
	defaultCDNBaseURL        = "https://cdn-resource.nqf.qq.com"
	defaultShutdownTimeout   = 5 * time.Second
	defaultReadHeaderTimeout = 10 * time.Second
	defaultIdleTimeout       = 120 * time.Second
	maxCDNAssetBytes         = 32 << 20
)

var (
	// ErrServerRunning prevents accidentally replacing a live listener owned by
	// the same Server instance.
	ErrServerRunning = errors.New("http server is already running")
	// ErrNilListener is returned when Serve is called without a listener.
	ErrNilListener = errors.New("http listener is nil")

	seedManifestName = regexp.MustCompile(`^Crop_(\d+)_Seed$`)
)

// ServerOptions supplies resources and route registration to the HTTP
// skeleton. WebFS and GameConfigFS are expected to be rooted at their
// respective directories; WebRoot and GameConfigRoot allow callers to pass an
// embed.FS rooted at the repository instead. P7 can therefore inject its
// embedded web/dist without changing this package.
type ServerOptions struct {
	WebFS          fs.FS
	WebRoot        string
	WebDir         string
	GameConfigFS   fs.FS
	GameConfigRoot string
	GameConfigDir  string
	LoginAssetsDir string

	CDNBaseURL string
	HTTPClient *http.Client

	ShutdownTimeout   time.Duration
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration
	Ready             func(context.Context) error

	// RegisterRoutes is called after the infrastructure routes are installed
	// and before serving starts. Domain handlers can register grouped routes
	// here without making server.go depend on future handler packages.
	RegisterRoutes func(*gin.Engine)
}

// Options is a short alias retained for callers that prefer a generic name.
type Options = ServerOptions

// Server owns the Gin engine and its net/http lifecycle. It contains only
// transport and resource concerns; authentication and domain handlers belong
// to later HTTP cards and are injected through RegisterRoutes.
type Server struct {
	engine       *gin.Engine
	httpServer   *http.Server
	addr         string
	startedAt    time.Time
	shutdownWait time.Duration

	webFS        fs.FS
	gameConfigFS fs.FS
	loginAssets  string
	cdnBase      *url.URL
	httpClient   *http.Client
	seedCDN      map[int]string
	readyCheck   func(context.Context) error
	draining     atomic.Bool

	mu      sync.Mutex
	running bool
}

// NewServer creates the Gin server skeleton using cfg's ADMIN_PORT-compatible
// port and resource paths. The listener is not opened until Run or Serve is
// called.
func NewServer(cfg config.Config, options ...ServerOptions) (*Server, error) {
	opts := ServerOptions{}
	if len(options) > 0 {
		opts = options[0]
	}

	port := cfg.AdminPort
	if port <= 0 || port > 65535 {
		port = config.DefaultAdminPort
	}

	webFS, err := resolveWebFS(cfg, opts)
	if err != nil {
		return nil, err
	}
	gameConfigFS, err := resolveGameConfigFS(cfg, opts)
	if err != nil {
		return nil, err
	}
	loginAssetsDir, err := resolveLoginAssetsDir(cfg, opts)
	if err != nil {
		return nil, err
	}

	cdnBase := strings.TrimSpace(opts.CDNBaseURL)
	if cdnBase == "" {
		cdnBase = defaultCDNBaseURL
	}
	parsedCDN, err := url.Parse(cdnBase)
	if err != nil || parsedCDN.Scheme != "https" && parsedCDN.Scheme != "http" || parsedCDN.Host == "" {
		return nil, fmt.Errorf("invalid CDN base URL %q", cdnBase)
	}

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	shutdownTimeout := opts.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = defaultShutdownTimeout
	}
	readHeaderTimeout := opts.ReadHeaderTimeout
	if readHeaderTimeout <= 0 {
		readHeaderTimeout = defaultReadHeaderTimeout
	}
	idleTimeout := opts.IdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = defaultIdleTimeout
	}

	server := &Server{
		engine:       gin.New(),
		addr:         net.JoinHostPort("0.0.0.0", strconv.Itoa(port)),
		startedAt:    time.Now(),
		shutdownWait: shutdownTimeout,
		webFS:        webFS,
		gameConfigFS: gameConfigFS,
		loginAssets:  loginAssetsDir,
		cdnBase:      parsedCDN,
		httpClient:   client,
		seedCDN:      loadSeedCDNMap(gameConfigFS),
		readyCheck:   opts.Ready,
	}
	server.httpServer = &http.Server{
		Addr:              server.addr,
		Handler:           server.engine,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}
	server.engine.Use(gin.Recovery())
	server.registerInfrastructureRoutes()
	if opts.RegisterRoutes != nil {
		opts.RegisterRoutes(server.engine)
	}
	server.registerFallback()
	return server, nil
}

// New is an alias for NewServer.
func New(cfg config.Config, options ...ServerOptions) (*Server, error) {
	return NewServer(cfg, options...)
}

// Engine exposes the Gin engine for the composition root and route handlers.
// Infrastructure routes are already installed by NewServer.
func (s *Server) Engine() *gin.Engine {
	if s == nil {
		return nil
	}
	return s.engine
}

// Handler exposes the server as a standard net/http handler for tests and
// embedding in another listener.
func (s *Server) Handler() http.Handler {
	if s == nil {
		return nil
	}
	return s.engine
}

// HTTPServer returns the underlying net/http server for composition-root
// integrations that need to inspect transport timeouts.
func (s *Server) HTTPServer() *http.Server {
	if s == nil {
		return nil
	}
	return s.httpServer
}

// Addr returns the configured public listen address. The server deliberately
// binds all interfaces to match the existing ADMIN_PORT deployment contract.
func (s *Server) Addr() string {
	if s == nil {
		return ""
	}
	return s.addr
}

// RegisterRoutes allows a composition root to add grouped routes after
// construction. No business logic is implemented here.
func (s *Server) RegisterRoutes(register func(*gin.Engine)) {
	if s == nil || register == nil {
		return
	}
	register(s.engine)
}

func (s *Server) registerInfrastructureRoutes() {
	s.engine.GET("/api/health", s.handleHealth)
	s.engine.GET("/api/ready", s.handleReady)
	s.engine.GET("/api/game-asset", s.handleGameAsset)
	s.engine.GET("/game-config", s.notFoundJSON)
	s.engine.GET("/game-config/*path", s.handleGameConfig)
	s.engine.GET("/login-assets", s.handleLoginAsset)
	s.engine.GET("/login-assets/*path", s.handleLoginAsset)
}

func (s *Server) registerFallback() {
	s.engine.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Status(http.StatusNotFound)
			return
		}
		requestPath := c.Request.URL.Path
		if isPathUnder(requestPath, "/api") || isPathUnder(requestPath, "/game-config") {
			s.notFoundJSON(c)
			return
		}
		if s.serveWebPath(c, requestPath) {
			return
		}
		c.String(http.StatusNotFound, "web build not found. Please build the web project.")
	})
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ok":        true,
		"status":    "ok",
		"uptime":    time.Since(s.startedAt).Seconds(),
		"timestamp": time.Now().UnixMilli(),
	})
}

func (s *Server) handleReady(c *gin.Context) {
	if s.IsDraining() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "ready": false, "draining": true})
		return
	}
	if s.readyCheck != nil {
		if err := s.readyCheck(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "ready": false, "error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "ready": true})
}

func (s *Server) BeginDrain() {
	if s != nil {
		s.draining.Store(true)
	}
}

func (s *Server) IsDraining() bool { return s != nil && s.draining.Load() }

func (s *Server) handleGameConfig(c *gin.Context) {
	name, ok := cleanFSName(c.Param("path"))
	if !ok || s.gameConfigFS == nil {
		s.notFoundJSON(c)
		return
	}
	if s.serveFSFile(c, s.gameConfigFS, name) {
		return
	}
	// Go module paths reject full-width punctuation in embedded filenames.
	// The two affected seed images are stored with ASCII parentheses and keep
	// their original URL working through this narrow fallback.
	compatName := strings.NewReplacer("（", "(", "）", ")").Replace(name)
	if compatName == name || !s.serveFSFile(c, s.gameConfigFS, compatName) {
		s.notFoundJSON(c)
	}
}

func (s *Server) handleLoginAsset(c *gin.Context) {
	setLoginAssetHeaders(c, c.Param("path"))
	name, ok := cleanDiskName(c.Param("path"))
	if !ok || s.loginAssets == "" {
		c.Status(http.StatusNotFound)
		return
	}
	root, err := filepath.Abs(s.loginAssets)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	filename := filepath.Join(root, filepath.FromSlash(name))
	relative, err := filepath.Rel(root, filename)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		c.Status(http.StatusNotFound)
		return
	}
	file, err := os.Open(filename)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}
	data, err := io.ReadAll(file)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	http.ServeContent(c.Writer, c.Request, filepath.Base(filename), info.ModTime(), bytes.NewReader(data))
}

func (s *Server) handleGameAsset(c *gin.Context) {
	seedID, err := strconv.Atoi(strings.TrimSpace(c.Query("seedId")))
	if err != nil || seedID <= 0 {
		c.Status(http.StatusNotFound)
		return
	}
	sourcePath, ok := s.seedCDN[seedID]
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	target := *s.cdnBase
	target.Path = strings.TrimRight(target.Path, "/") + "/" + strings.TrimLeft(sourcePath, "/")
	target.RawPath = ""
	request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		c.Status(http.StatusNotFound)
		return
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxCDNAssetBytes+1))
	if err != nil || len(data) > maxCDNAssetBytes {
		c.Status(http.StatusBadGateway)
		return
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(http.StatusOK, "image/png", data)
}

func (s *Server) notFoundJSON(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "Not Found"})
}

func (s *Server) serveWebPath(c *gin.Context, requestPath string) bool {
	if s.webFS == nil {
		return false
	}
	name, ok := cleanFSName(requestPath)
	if !ok || name == "" {
		name = "index.html"
	}
	if s.serveFSFile(c, s.webFS, name) {
		return true
	}
	if name != "index.html" {
		return s.serveFSFile(c, s.webFS, "index.html")
	}
	return false
}

func (s *Server) serveFSFile(c *gin.Context, root fs.FS, name string) bool {
	if root == nil || name == "" {
		return false
	}
	file, err := root.Open(name)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return false
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return false
	}
	http.ServeContent(c.Writer, c.Request, path.Base(name), info.ModTime(), bytes.NewReader(data))
	return true
}

// Serve runs the HTTP server on an already-created listener and returns when
// the listener stops. It is useful for tests and for composition roots that
// reserve a port before handing it to Gin.
func (s *Server) Serve(listener net.Listener) error {
	if s == nil || s.httpServer == nil {
		return errors.New("http server is nil")
	}
	if listener == nil {
		return ErrNilListener
	}
	if !s.markRunning() {
		_ = listener.Close()
		return ErrServerRunning
	}
	defer s.markStopped()
	err := s.httpServer.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// ListenAndServe binds 0.0.0.0:ADMIN_PORT and blocks until the server stops.
// Prefer Run or RunWithSignals for context-aware lifecycle management.
func (s *Server) ListenAndServe() error {
	if s == nil || s.httpServer == nil {
		return errors.New("http server is nil")
	}
	if !s.markRunning() {
		return ErrServerRunning
	}
	defer s.markStopped()
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Run starts the listener and shuts it down gracefully when ctx is canceled.
// Normal context cancellation returns nil after active requests drain.
func (s *Server) Run(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return errors.New("http server is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.addr, err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- s.Serve(listener) }()
	select {
	case err := <-serveDone:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownWait)
		shutdownErr := s.Shutdown(shutdownCtx)
		cancel()
		serveErr := <-serveDone
		if shutdownErr != nil {
			return shutdownErr
		}
		return serveErr
	}
}

// RunWithSignals runs until ctx is canceled or SIGINT/SIGTERM is received.
func (s *Server) RunWithSignals(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return s.Run(signalCtx)
}

// Shutdown gracefully drains active HTTP requests. A nil context receives the
// configured shutdown timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}
	s.BeginDrain()
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), s.shutdownWait)
		defer cancel()
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) markRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return false
	}
	s.running = true
	return true
}

func (s *Server) markStopped() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

func resolveWebFS(cfg config.Config, options ServerOptions) (fs.FS, error) {
	if options.WebFS != nil {
		return subFS(options.WebFS, options.WebRoot)
	}
	if options.WebDir != "" {
		return diskFS(options.WebDir, true)
	}
	candidates := []string{"web/dist"}
	if cfg.Paths.ResourceDir != "" {
		candidates = append(candidates, filepath.Join(cfg.Paths.ResourceDir, "web", "dist"))
	}
	if disk := firstDiskFS(candidates); disk != nil {
		return disk, nil
	}
	return embeddedSubFS(cfg.Paths.Embedded, "web/dist", "assets/web/dist"), nil
}

func resolveGameConfigFS(cfg config.Config, options ServerOptions) (fs.FS, error) {
	if options.GameConfigFS != nil {
		return subFS(options.GameConfigFS, options.GameConfigRoot)
	}
	if options.GameConfigDir != "" {
		return diskFS(options.GameConfigDir, true)
	}
	candidates := []string{"gameConfig"}
	if cfg.Paths.ResourceDir != "" {
		candidates = append(candidates, filepath.Join(cfg.Paths.ResourceDir, "gameConfig"))
	}
	if disk := firstDiskFS(candidates); disk != nil {
		return disk, nil
	}
	return embeddedSubFS(cfg.Paths.Embedded, "gameConfig", "assets/gameConfig"), nil
}

func resolveLoginAssetsDir(cfg config.Config, options ServerOptions) (string, error) {
	if options.LoginAssetsDir != "" {
		absolute, err := filepath.Abs(options.LoginAssetsDir)
		if err != nil {
			return "", fmt.Errorf("resolve login assets directory: %w", err)
		}
		if err := os.MkdirAll(absolute, 0o755); err != nil {
			return "", fmt.Errorf("create login assets directory: %w", err)
		}
		return absolute, nil
	}
	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = cfg.Paths.DataDir
	}
	if dataDir == "" {
		dataDir = config.ResolvePaths().DataDir
	}
	absolute := filepath.Join(dataDir, "login-assets")
	if err := os.MkdirAll(absolute, 0o755); err != nil {
		return "", fmt.Errorf("create login assets directory: %w", err)
	}
	return absolute, nil
}

func firstDiskFS(candidates []string) fs.FS {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return os.DirFS(candidate)
		}
	}
	return nil
}

func embeddedSubFS(root fs.FS, candidates ...string) fs.FS {
	if root == nil {
		return nil
	}
	for _, candidate := range candidates {
		if candidate == "" {
			return root
		}
		resource, err := fs.Sub(root, candidate)
		if err == nil {
			return resource
		}
	}
	return nil
}

func diskFS(directory string, required bool) (fs.FS, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve resource directory: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		if !required && os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat resource directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("resource path %q is not a directory", absolute)
	}
	return os.DirFS(absolute), nil
}

func subFS(root fs.FS, subdir string) (fs.FS, error) {
	if strings.TrimSpace(subdir) == "" {
		return root, nil
	}
	clean, ok := cleanFSName(subdir)
	if !ok {
		return nil, fmt.Errorf("invalid embedded resource root %q", subdir)
	}
	result, err := fs.Sub(root, clean)
	if err != nil {
		return nil, fmt.Errorf("open embedded resource root %q: %w", subdir, err)
	}
	return result, nil
}

func cleanFSName(raw string) (string, bool) {
	value := strings.TrimPrefix(raw, "/")
	if value == "" {
		return "", true
	}
	if strings.ContainsRune(value, 0) {
		return "", false
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return "", false
	}
	return clean, true
}

func cleanDiskName(raw string) (string, bool) {
	clean, ok := cleanFSName(raw)
	if !ok || clean == "" {
		return "", false
	}
	for _, part := range strings.Split(clean, "/") {
		if part == ".." || strings.HasPrefix(part, ".") {
			return "", false
		}
	}
	return clean, true
}

func isPathUnder(requestPath, prefix string) bool {
	return requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/")
}

func setLoginAssetHeaders(c *gin.Context, requestPath string) {
	c.Header("X-Content-Type-Options", "nosniff")
	if strings.EqualFold(filepath.Ext(path.Base(requestPath)), ".svg") {
		c.Header("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'; img-src data:")
	}
}

func loadSeedCDNMap(root fs.FS) map[int]string {
	result := make(map[int]string)
	if root == nil {
		return result
	}
	data, err := fs.ReadFile(root, "manifest.csv")
	if err != nil {
		return result
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	for rowIndex := 0; ; rowIndex++ {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || rowIndex == 0 || len(row) < 5 {
			continue
		}
		spriteName := strings.TrimSpace(row[4])
		match := seedManifestName.FindStringSubmatch(spriteName)
		if len(match) != 2 {
			continue
		}
		seedID, err := strconv.Atoi(match[1])
		if err != nil || seedID <= 0 {
			continue
		}
		sourcePath, ok := normalizeCDNPath(row[1])
		if ok {
			result[seedID] = sourcePath
		}
	}
	return result
}

func normalizeCDNPath(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return "", false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return "", false
	}
	clean := strings.TrimPrefix(path.Clean("/"+value), "/")
	if clean == "" || clean == "." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	for _, part := range strings.Split(clean, "/") {
		if part == ".." {
			return "", false
		}
	}
	return clean, true
}
