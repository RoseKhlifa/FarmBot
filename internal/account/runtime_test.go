package account

import (
	"context"
	"errors"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/game/session"
	"github.com/RoseKhlifa/FarmBot/internal/game/tsdk"
)

func TestNormalizeTSDKDeviceBeforeRuntimeCreation(t *testing.T) {
	options := &session.Options{OS: "iOS"}
	normalizeTSDKDevice(options)
	got := options.TSDK.Device
	if got.Model != "iPhone 15 Pro Max" || got.Brand != "Apple" || got.Platform != "iOS" {
		t.Fatalf("normalized device = %+v, want iPhone/Apple/iOS", got)
	}
}

func TestNormalizeTSDKDevicePreservesCustomValues(t *testing.T) {
	options := &session.Options{OS: "Android"}
	options.TSDK.Device.Model = "Mate 60 Pro"
	options.TSDK.Device.Brand = "Huawei"
	normalizeTSDKDevice(options)
	got := options.TSDK.Device
	if got.Model != "Mate 60 Pro" || got.Brand != "Huawei" || got.Platform != "Android" {
		t.Fatalf("normalized device = %+v, want custom model/brand/Android", got)
	}
}

func TestRuntimeStartNormalizesDeviceBeforeTSDKFactory(t *testing.T) {
	var captured tsdk.Options
	runtime := NewRuntime(Config{
		AccountID: "account-test",
		LoginCode: "login-code",
		Session:   session.Options{OS: "iOS"},
	}, Dependencies{
		TSDK: func(ctx context.Context, options tsdk.Options) *tsdk.Runtime {
			captured = options
			return tsdk.New(ctx, options)
		},
		Login: func(context.Context, string, session.Options) (*session.Session, error) {
			return nil, errors.New("stop after TSDK construction")
		},
	})

	if err := runtime.Start(context.Background()); err == nil {
		t.Fatal("Start() unexpectedly succeeded")
	}
	if got := captured.Device; got.Model != "iPhone 15 Pro Max" || got.Brand != "Apple" || got.Platform != "iOS" {
		t.Fatalf("TSDK factory received device = %+v, want iPhone/Apple/iOS", got)
	}
}
