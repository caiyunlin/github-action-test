package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appcontainers/armappcontainers/v3"
)

func TestValidateContainerApp(t *testing.T) {
	cfg := testConfig()
	identity, properties := validContainerApp(cfg)

	revisionName, err := validateContainerApp(identity, properties, cfg)
	if err != nil {
		t.Fatalf("validateContainerApp() error = %v", err)
	}
	if revisionName != "aca-hello--0000003" {
		t.Errorf("revisionName = %q", revisionName)
	}
}

func TestLoadConfigBuildsManagedIdentityResourceID(t *testing.T) {
	t.Setenv("AZURE_SUBSCRIPTION_ID", "subscription")
	t.Setenv("ACA_RESOURCE_GROUP", "aca-rg")
	t.Setenv("ACA_NAME", "aca-hello")
	t.Setenv("ACA_CONTAINER_NAME", "simple-hello-world-container")
	t.Setenv("ACA_SECRET_NAME", "temporal-api-key")
	t.Setenv("ACA_ENVIRONMENT_VARIABLE", "TEMPORAL_API_KEY")
	t.Setenv("KEY_VAULT_NAME", "mykv0224")
	t.Setenv("TARGET_SECRET_NAME", "temporal-api-key")
	t.Setenv("ACA_KEY_VAULT_IDENTITY_NAME", "aca-hello-keyvault-identity")
	t.Setenv("GITHUB_ENV", filepath.Join(t.TempDir(), "github-env"))

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	expected := "/subscriptions/subscription/resourceGroups/aca-rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/aca-hello-keyvault-identity"
	if cfg.managedIdentityID != expected {
		t.Errorf("managedIdentityID = %q, want %q", cfg.managedIdentityID, expected)
	}
	expectedSecretURL := "https://mykv0224.vault.azure.net/secrets/temporal-api-key"
	if cfg.keyVaultSecretURL != expectedSecretURL {
		t.Errorf("keyVaultSecretURL = %q, want %q", cfg.keyVaultSecretURL, expectedSecretURL)
	}
}

func TestValidateContainerAppRejectsWrongKeyVaultURL(t *testing.T) {
	cfg := testConfig()
	identity, properties := validContainerApp(cfg)
	wrongURL := "https://other.vault.azure.net/secrets/temporal-api-key"
	properties.Configuration.Secrets[0].KeyVaultURL = &wrongURL

	_, err := validateContainerApp(identity, properties, cfg)
	if err == nil || !strings.Contains(err.Error(), "Key Vault URL") {
		t.Fatalf("validateContainerApp() error = %v, want Key Vault URL error", err)
	}
}

func TestWriteGitHubEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "github-env")
	if err := writeGitHubEnvironment(path, "ACA_REVISION_NAME", "aca-hello--0000003"); err != nil {
		t.Fatalf("writeGitHubEnvironment() error = %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(contents) != "ACA_REVISION_NAME=aca-hello--0000003\n" {
		t.Errorf("environment file = %q", contents)
	}
}

func testConfig() config {
	return config{
		containerName:           "simple-hello-world-container",
		secretName:              "temporal-api-key",
		environmentVariableName: "TEMPORAL_API_KEY",
		keyVaultSecretURL:       "https://mykv0224.vault.azure.net/secrets/temporal-api-key",
		managedIdentityName:     "aca-identity",
		managedIdentityID:       "/subscriptions/subscription/resourceGroups/aca-rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/aca-identity",
	}
}

func validContainerApp(cfg config) (*armappcontainers.ManagedServiceIdentity, *armappcontainers.ContainerAppProperties) {
	revisionName := "aca-hello--0000003"
	secretName := cfg.secretName
	secretURL := cfg.keyVaultSecretURL
	identityID := cfg.managedIdentityID
	containerName := cfg.containerName
	environmentVariableName := cfg.environmentVariableName

	identity := &armappcontainers.ManagedServiceIdentity{
		UserAssignedIdentities: map[string]*armappcontainers.UserAssignedIdentity{
			strings.ToUpper(identityID): {},
		},
	}
	properties := &armappcontainers.ContainerAppProperties{
		LatestRevisionName: &revisionName,
		Configuration: &armappcontainers.Configuration{
			Secrets: []*armappcontainers.Secret{{
				Name:        &secretName,
				KeyVaultURL: &secretURL,
				Identity:    &identityID,
			}},
		},
		Template: &armappcontainers.Template{
			Containers: []*armappcontainers.Container{{
				Name: &containerName,
				Env: []*armappcontainers.EnvironmentVar{{
					Name:      &environmentVariableName,
					SecretRef: &secretName,
				}},
			}},
		},
	}
	return identity, properties
}
