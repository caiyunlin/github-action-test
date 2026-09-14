package main

import (
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appcontainers/armappcontainers/v3"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("AZURE_SUBSCRIPTION_ID", "subscription")
	t.Setenv("ACA_RESOURCE_GROUP", "aca-rg")
	t.Setenv("ACA_NAME", "aca-hello")
	t.Setenv("ACA_REVISION_NAME", "aca-hello--0000003")
	t.Setenv("ACA_SECRET_NAME", "temporal-api-key")
	t.Setenv("TEMPORAL_API_KEY", "expected-value")
	t.Setenv("ACA_VALIDATION_TIMEOUT", "2m")
	t.Setenv("ACA_VALIDATION_INTERVAL", "5s")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.validationTimeout != 2*time.Minute {
		t.Errorf("validationTimeout = %s", cfg.validationTimeout)
	}
	if cfg.validationInterval != 5*time.Second {
		t.Errorf("validationInterval = %s", cfg.validationInterval)
	}
}

func TestLoadConfigRejectsMultilineSecret(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("TEMPORAL_API_KEY", "first\nsecond")

	_, err := loadConfig()
	if err == nil || !strings.Contains(err.Error(), "single-line") {
		t.Fatalf("loadConfig() error = %v, want single-line validation error", err)
	}
}

func TestResolvedSecretMatches(t *testing.T) {
	name := "TEMPORAL-API-KEY"
	value := "expected-value"
	secrets := []*armappcontainers.ContainerAppSecret{{Name: &name, Value: &value}}

	if !resolvedSecretMatches(secrets, "temporal-api-key", value) {
		t.Fatal("resolvedSecretMatches() = false, want true")
	}
	if resolvedSecretMatches(secrets, "temporal-api-key", "different-value") {
		t.Fatal("resolvedSecretMatches() = true for a different value")
	}
}

func TestValidationStates(t *testing.T) {
	healthy := validationState{secretMatches: true, health: "Healthy", provisioning: "Provisioned", running: "Running"}
	if !isSuccessful(healthy) {
		t.Fatal("isSuccessful() = false for healthy matching state")
	}

	unhealthy := validationState{health: "Unhealthy", provisioning: "Provisioned", running: "Running"}
	if !isFailed(unhealthy) {
		t.Fatal("isFailed() = false for unhealthy state")
	}
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("AZURE_SUBSCRIPTION_ID", "subscription")
	t.Setenv("ACA_RESOURCE_GROUP", "aca-rg")
	t.Setenv("ACA_NAME", "aca-hello")
	t.Setenv("ACA_REVISION_NAME", "aca-hello--0000003")
	t.Setenv("ACA_SECRET_NAME", "temporal-api-key")
	t.Setenv("TEMPORAL_API_KEY", "expected-value")
}
