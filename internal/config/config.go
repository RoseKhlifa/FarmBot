package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultAdminPort     = 3007
	DefaultServerURL     = "wss://gate-obt.nqf.qq.com/prod/ws"
	DefaultClientVersion = "1.13.0.4_20260723"
	DefaultTSDKGameID    = 3167
	DefaultTSDKAppKey    = "0"
	DefaultYYBPort       = 8450
)

// TSDKConfig contains the values used to initialize the game security SDK.
type TSDKConfig struct {
	GameID     int
	AppKey     string
	AceEnabled bool
}

// YYBConfig contains the legacy yyb-go settings that remain useful while the
// service is being folded into the main process. APIURL, APIKey and Port are
// deprecated after the in-process integration, but are still read for a
// backwards-compatible configuration transition.
type YYBConfig struct {
	Enabled      bool
	Host         string
	Port         int
	ResourceRoot string
	TokenFile    string
	APIURL       string
	APIKey       string
	APIToken     string
	AdminUser    string
	AdminPass    string
}

// WXProxyConfig contains the optional upstream proxy credentials used by the
// public proxy routes.
type WXProxyConfig struct {
	APIURL string
	APIKey string
	AppID  string
}

// IntervalConfig keeps the scheduler defaults from the Node runtime in one
// typed value. Durations are represented in Go's native time.Duration unit.
type IntervalConfig struct {
	Heartbeat      time.Duration
	FarmCheck      time.Duration
	FriendCheck    time.Duration
	FarmCheckMin   time.Duration
	FarmCheckMax   time.Duration
	FriendCheckMin time.Duration
	FriendCheckMax time.Duration
	HelpCheckMin   time.Duration
	HelpCheckMax   time.Duration
	StealCheckMin  time.Duration
	StealCheckMax  time.Duration
}

// Config is the process-wide configuration loaded from environment variables
// and the documented defaults. It intentionally has no dependency on a
// configuration framework so it can be used by the composition root directly.
type Config struct {
	AdminPort      int
	AdminPassword  string
	DataDir        string
	ResourceDir    string
	LogDir         string
	LogLevel       string
	ServerURL      string
	ClientVersion  string
	Platform       string
	OS             string
	LicenseEnabled bool
	MasterKey      string
	TSDK           TSDKConfig
	Yyb            YYBConfig
	WxProxy        WXProxyConfig
	Intervals      IntervalConfig
	Paths          Paths

	// The flattened interval fields ease migration for callers that previously
	// read the Node CONFIG object directly. Intervals is the canonical grouping.
	HeartbeatInterval      time.Duration
	FarmCheckInterval      time.Duration
	FriendCheckInterval    time.Duration
	FarmCheckIntervalMin   time.Duration
	FarmCheckIntervalMax   time.Duration
	FriendCheckIntervalMin time.Duration
	FriendCheckIntervalMax time.Duration
	HelpCheckIntervalMin   time.Duration
	HelpCheckIntervalMax   time.Duration
	StealCheckIntervalMin  time.Duration
	StealCheckIntervalMax  time.Duration

	// Warnings contains non-fatal configuration warnings emitted while loading.
	Warnings []string
}

// DeprecatedEnvVars documents variables from the old process-per-account
// runtime. Go uses goroutines per account, so these variables are intentionally
// not consumed by Load. YYB_API_URL/KEY/PORT are likewise retained only as
// compatibility inputs during the yyb in-process migration.
var DeprecatedEnvVars = []string{
	"FARM_WORKER",
	"FARM_RUNTIME_MODE",
	"FARM_ACCOUNT_ID",
	"YYB_API_URL",
	"YYB_API_KEY",
	"YYB_PORT",
}

// Load reads the process environment and returns a complete configuration.
// Invalid numeric values fall back to their documented defaults; loading never
// panics because configuration is user-controlled input.
func Load() Config {
	paths := ResolvePaths()
	cfg := Config{
		AdminPort:      envInt("ADMIN_PORT", DefaultAdminPort, 1, 65535),
		AdminPassword:  os.Getenv("ADMIN_PASSWORD"),
		DataDir:        paths.DataDir,
		ResourceDir:    paths.ResourceDir,
		LogDir:         os.Getenv("FARM_LOG_DIR"),
		LogLevel:       envString("LOG_LEVEL", "info"),
		ServerURL:      envString("FARM_SERVER_URL", DefaultServerURL),
		ClientVersion:  envString("FARM_CLIENT_VERSION", DefaultClientVersion),
		Platform:       envString("FARM_PLATFORM", "qq"),
		OS:             envString("FARM_OS", "iOS"),
		LicenseEnabled: strings.TrimSpace(os.Getenv("FARM_LICENSE_ENABLED")) == "true",
		MasterKey:      strings.TrimSpace(os.Getenv("FARM_MASTER_KEY")),
		TSDK: TSDKConfig{
			GameID:     envInt("FARM_TSDK_GAME_ID", DefaultTSDKGameID, 1, 0),
			AppKey:     envString("FARM_TSDK_APP_KEY", DefaultTSDKAppKey),
			AceEnabled: envNotFalse("FARM_TSDK_ACE_ENABLED"),
		},
		Yyb: YYBConfig{
			Enabled:      envBool("YYB_ENABLED", true),
			Host:         envString("YYB_HOST", "127.0.0.1"),
			Port:         envInt("YYB_PORT", DefaultYYBPort, 1, 65535),
			ResourceRoot: os.Getenv("YYB_RESOURCE_ROOT"),
			TokenFile:    os.Getenv("YYB_TOKEN_FILE"),
			APIURL:       os.Getenv("YYB_API_URL"),
			APIKey:       os.Getenv("YYB_API_KEY"),
			APIToken:     os.Getenv("YYB_API_TOKEN"),
			AdminUser:    os.Getenv("YYB_ADMIN_USER"),
			AdminPass:    os.Getenv("YYB_ADMIN_PASS"),
		},
		WxProxy: WXProxyConfig{
			APIURL: os.Getenv("WX_PROXY_API_URL"),
			APIKey: os.Getenv("WX_PROXY_API_KEY"),
			AppID:  os.Getenv("WX_PROXY_APP_ID"),
		},
		Intervals: defaultIntervals(),
		Paths:     paths,
	}
	if cfg.Yyb.APIToken == "" {
		// The old deployment convention used the same value for both names.
		cfg.Yyb.APIToken = cfg.Yyb.APIKey
	}
	if cfg.MasterKey == "" {
		const warning = "FARM_MASTER_KEY is not set; credential encryption is not protected by a master key"
		cfg.Warnings = append(cfg.Warnings, warning)
		slog.Warn(warning)
	}
	copyFlattenedIntervals(&cfg)
	return cfg
}

// LoadConfig is an explicit alias for callers that prefer a descriptive name.
func LoadConfig() Config { return Load() }

// Default returns the same defaults as Load without relying on mutable process
// state. It is useful for documentation and callers constructing a baseline.
func Default() Config {
	paths := ResolvePaths()
	cfg := Config{
		AdminPort:     DefaultAdminPort,
		DataDir:       paths.DataDir,
		ResourceDir:   paths.ResourceDir,
		LogLevel:      "info",
		ServerURL:     DefaultServerURL,
		ClientVersion: DefaultClientVersion,
		Platform:      "qq",
		OS:            "iOS",
		TSDK:          TSDKConfig{GameID: DefaultTSDKGameID, AppKey: DefaultTSDKAppKey, AceEnabled: true},
		Yyb:           YYBConfig{Enabled: true, Host: "127.0.0.1", Port: DefaultYYBPort},
		Intervals:     defaultIntervals(),
		Paths:         paths,
	}
	copyFlattenedIntervals(&cfg)
	return cfg
}

func defaultIntervals() IntervalConfig {
	return IntervalConfig{
		Heartbeat:      25 * time.Second,
		FarmCheck:      3 * time.Second,
		FriendCheck:    8 * time.Second,
		FarmCheckMin:   3 * time.Second,
		FarmCheckMax:   5 * time.Second,
		FriendCheckMin: 8 * time.Second,
		FriendCheckMax: 10 * time.Second,
		HelpCheckMin:   15 * time.Second,
		HelpCheckMax:   20 * time.Second,
		StealCheckMin:  10 * time.Second,
		StealCheckMax:  15 * time.Second,
	}
}

func copyFlattenedIntervals(cfg *Config) {
	i := cfg.Intervals
	cfg.HeartbeatInterval = i.Heartbeat
	cfg.FarmCheckInterval = i.FarmCheck
	cfg.FriendCheckInterval = i.FriendCheck
	cfg.FarmCheckIntervalMin = i.FarmCheckMin
	cfg.FarmCheckIntervalMax = i.FarmCheckMax
	cfg.FriendCheckIntervalMin = i.FriendCheckMin
	cfg.FriendCheckIntervalMax = i.FriendCheckMax
	cfg.HelpCheckIntervalMin = i.HelpCheckMin
	cfg.HelpCheckIntervalMax = i.HelpCheckMax
	cfg.StealCheckIntervalMin = i.StealCheckMin
	cfg.StealCheckIntervalMax = i.StealCheckMax
}

func envString(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
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

// envNotFalse preserves the Node runtime's opt-out semantics: only the exact
// string "false" disables the feature, while an unset or malformed value keeps
// it enabled.
func envNotFalse(name string) bool {
	return strings.TrimSpace(os.Getenv(name)) != "false"
}

func envInt(name string, fallback, min, max int) int {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < min || (max > 0 && parsed > max) {
		return fallback
	}
	return parsed
}
