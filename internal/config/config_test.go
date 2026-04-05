package config

import "testing"

func TestDefaultRemoteConnectBindingsAreNormalized(t *testing.T) {
	cfg := Default()

	if len(cfg.Keys.RemoteConnect) != 2 {
		t.Fatalf("len(RemoteConnect) = %d, want 2", len(cfg.Keys.RemoteConnect))
	}
	if cfg.Keys.RemoteConnect[0] != "ctrl+k" {
		t.Fatalf("RemoteConnect[0] = %q, want ctrl+k", cfg.Keys.RemoteConnect[0])
	}
	if cfg.Keys.RemoteConnect[1] != "f14" {
		t.Fatalf("RemoteConnect[1] = %q, want f14", cfg.Keys.RemoteConnect[1])
	}
}
