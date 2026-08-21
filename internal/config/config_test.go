package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	for _, name := range []string{
		"ADMIN_PORT", "ADMIN_PASSWORD", "FARM_DATA_DIR", "FARM_SERVER_URL", "FARM_CLIENT_VERSION",
		"FARM_RESOURCE_DIR", "FARM_LOG_DIR", "LOG_LEVEL", "WX_PROXY_API_URL", "WX_PROXY_API_KEY", "WX_PROXY_APP_ID",
		"FARM_PLATFORM", "FARM_OS", "FARM_LICENSE_ENABLED", "FARM_MASTER_KEY", "FARM_TSDK_GAME_ID",
		"FARM_TSDK_APP_KEY", "FARM_TSDK_ACE_ENABLED", "FARM_YYB_ENABLED",
	} {
		t.Setenv(name, "")
	}

	cfg := Load()
	if cfg.AdminPort != DefaultAdminPort {
		t.Fatalf("AdminPort = %d, want %d", cfg.AdminPort, DefaultAdminPort)
	}
	if cfg.ServerURL != DefaultServerURL || cfg.ClientVersion != DefaultClientVersion {
		t.Fatalf("unexpected game defaults: %#v", cfg)
	}
	if cfg.TSDK.GameID != DefaultTSDKGameID || cfg.TSDK.AppKey != DefaultTSDKAppKey || !cfg.TSDK.AceEnabled {
		t.Fatalf("unexpected TSDK defaults: %#v", cfg.TSDK)
	}
	if !cfg.Yyb.Enabled {
		t.Fatalf("unexpected yyb defaults: %#v", cfg.Yyb)
	}
	if cfg.Intervals.Heartbeat != 25*time.Second || cfg.FarmCheckIntervalMin != 3*time.Second {
		t.Fatalf("unexpected interval defaults: %#v", cfg.Intervals)
	}
	if len(cfg.Warnings) != 1 {
		t.Fatalf("expected one non-fatal master key warning, got %v", cfg.Warnings)
	}
}

func TestLoadEnvironmentOverrides(t *testing.T) {
	t.Setenv("ADMIN_PORT", "9090")
	t.Setenv("ADMIN_PASSWORD", "secret")
	t.Setenv("FARM_LOG_DIR", "logs")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("WX_PROXY_API_URL", "https://proxy.example.test")
	t.Setenv("WX_PROXY_API_KEY", "proxy-key")
	t.Setenv("WX_PROXY_APP_ID", "proxy-app")
	t.Setenv("FARM_SERVER_URL", "wss://example.test/ws")
	t.Setenv("FARM_CLIENT_VERSION", "test-version")
	t.Setenv("FARM_LICENSE_ENABLED", "true")
	t.Setenv("FARM_MASTER_KEY", "master")
	t.Setenv("FARM_TSDK_GAME_ID", "42")
	t.Setenv("FARM_TSDK_APP_KEY", "key")
	t.Setenv("FARM_TSDK_ACE_ENABLED", "false")
	t.Setenv("FARM_YYB_ENABLED", "false")

	cfg := Load()
	if cfg.AdminPort != 9090 || cfg.AdminPassword != "secret" || !cfg.LicenseEnabled {
		t.Fatalf("admin/license overrides not applied: %#v", cfg)
	}
	if cfg.LogDir != "logs" || cfg.LogLevel != "debug" || cfg.WxProxy.APIURL != "https://proxy.example.test" || cfg.WxProxy.APIKey != "proxy-key" || cfg.WxProxy.AppID != "proxy-app" {
		t.Fatalf("logging/proxy overrides not applied: %#v", cfg)
	}
	if cfg.ServerURL != "wss://example.test/ws" || cfg.ClientVersion != "test-version" {
		t.Fatalf("game overrides not applied: %#v", cfg)
	}
	if cfg.TSDK.GameID != 42 || cfg.TSDK.AppKey != "key" || cfg.TSDK.AceEnabled {
		t.Fatalf("TSDK overrides not applied: %#v", cfg.TSDK)
	}
	if cfg.Yyb.Enabled {
		t.Fatalf("yyb overrides not applied: %#v", cfg.Yyb)
	}
	if len(cfg.Warnings) != 0 {
		t.Fatalf("master key warning should be absent when configured: %v", cfg.Warnings)
	}
}

func TestInvalidNumericEnvironmentUsesDefaults(t *testing.T) {
	t.Setenv("ADMIN_PORT", "not-a-port")
	t.Setenv("FARM_TSDK_GAME_ID", "-1")
	cfg := Load()
	if cfg.AdminPort != DefaultAdminPort || cfg.TSDK.GameID != DefaultTSDKGameID {
		t.Fatalf("invalid values should use defaults: %#v", cfg)
	}
}
