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
