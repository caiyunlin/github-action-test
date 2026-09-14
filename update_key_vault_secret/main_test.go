package main

import (
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("KEY_VAULT_NAME", "mykv0224")
	t.Setenv("TARGET_SECRET_NAME", "temporal-api-key")
	t.Setenv("TEMPORAL_API_KEY", "expected-value")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.vaultURL != "https://mykv0224.vault.azure.net" {
		t.Errorf("vaultURL = %q", cfg.vaultURL)
	}
	if cfg.secretName != "temporal-api-key" {
		t.Errorf("secretName = %q", cfg.secretName)
	}
	if cfg.secretValue != "expected-value" {
		t.Error("secretValue does not match")
	}
}

func TestLoadConfigRejectsMultilineSecret(t *testing.T) {
	t.Setenv("KEY_VAULT_NAME", "mykv0224")
	t.Setenv("TARGET_SECRET_NAME", "temporal-api-key")
	t.Setenv("TEMPORAL_API_KEY", "first\nsecond")

	_, err := loadConfig()
	if err == nil || !strings.Contains(err.Error(), "single-line") {
		t.Fatalf("loadConfig() error = %v, want single-line validation error", err)
	}
}
