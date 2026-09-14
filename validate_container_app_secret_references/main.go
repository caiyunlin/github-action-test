package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appcontainers/armappcontainers/v3"
)

type config struct {
	subscriptionID          string
	resourceGroupName       string
	containerAppName        string
	containerName           string
	secretName              string
	environmentVariableName string
	keyVaultSecretURL       string
	managedIdentityID       string
	githubEnvironmentPath   string
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "validate Container App secret references: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return fmt.Errorf("create Azure credential: %w", err)
	}
	client, err := armappcontainers.NewContainerAppsClient(cfg.subscriptionID, credential, nil)
	if err != nil {
		return fmt.Errorf("create Container Apps client: %w", err)
	}

	response, err := client.Get(ctx, cfg.resourceGroupName, cfg.containerAppName, nil)
	if err != nil {
		return fmt.Errorf("get Container App: %w", err)
	}
	revisionName, err := validateContainerApp(response.Identity, response.Properties, cfg)
	if err != nil {
		return err
	}
	if err := writeGitHubEnvironment(cfg.githubEnvironmentPath, "ACA_REVISION_NAME", revisionName); err != nil {
		return err
	}

	fmt.Println("Validated ACA Key Vault and environment-variable references")
	return nil
}

func loadConfig() (config, error) {
	cfg := config{
		subscriptionID:          strings.TrimSpace(os.Getenv("AZURE_SUBSCRIPTION_ID")),
		resourceGroupName:       strings.TrimSpace(os.Getenv("ACA_RESOURCE_GROUP")),
		containerAppName:        strings.TrimSpace(os.Getenv("ACA_NAME")),
		containerName:           strings.TrimSpace(os.Getenv("ACA_CONTAINER_NAME")),
		secretName:              strings.TrimSpace(os.Getenv("ACA_SECRET_NAME")),
		environmentVariableName: strings.TrimSpace(os.Getenv("ACA_ENVIRONMENT_VARIABLE")),
		keyVaultSecretURL:       strings.TrimRight(strings.TrimSpace(os.Getenv("TARGET_SECRET_URL")), "/"),
		managedIdentityID:       strings.TrimRight(strings.TrimSpace(os.Getenv("ACA_KEY_VAULT_IDENTITY")), "/"),
		githubEnvironmentPath:   strings.TrimSpace(os.Getenv("GITHUB_ENV")),
	}

	required := []struct {
		name  string
		value string
	}{
		{name: "AZURE_SUBSCRIPTION_ID", value: cfg.subscriptionID},
		{name: "ACA_RESOURCE_GROUP", value: cfg.resourceGroupName},
		{name: "ACA_NAME", value: cfg.containerAppName},
		{name: "ACA_CONTAINER_NAME", value: cfg.containerName},
		{name: "ACA_SECRET_NAME", value: cfg.secretName},
		{name: "ACA_ENVIRONMENT_VARIABLE", value: cfg.environmentVariableName},
		{name: "TARGET_SECRET_URL", value: cfg.keyVaultSecretURL},
		{name: "ACA_KEY_VAULT_IDENTITY", value: cfg.managedIdentityID},
		{name: "GITHUB_ENV", value: cfg.githubEnvironmentPath},
	}
	for _, item := range required {
		if item.value == "" {
			return config{}, fmt.Errorf("environment variable %s is required", item.name)
		}
	}
	return cfg, nil
}

func validateContainerApp(identity *armappcontainers.ManagedServiceIdentity, properties *armappcontainers.ContainerAppProperties, cfg config) (string, error) {
	if properties == nil || properties.Configuration == nil || properties.Template == nil {
		return "", fmt.Errorf("Container App configuration is incomplete")
	}
	if properties.LatestRevisionName == nil || strings.TrimSpace(*properties.LatestRevisionName) == "" {
		return "", fmt.Errorf("Container App has no latest revision")
	}
	if !hasManagedIdentity(identity, cfg.managedIdentityID) {
		return "", fmt.Errorf("managed identity %q is not assigned to the Container App", cfg.managedIdentityID)
	}
	if !hasKeyVaultSecretReference(properties.Configuration.Secrets, cfg) {
		return "", fmt.Errorf("ACA secret %q does not reference the expected Key Vault URL and managed identity", cfg.secretName)
	}
	if !hasEnvironmentVariableReference(properties.Template.Containers, cfg) {
		return "", fmt.Errorf("environment variable %q does not reference ACA secret %q in container %q", cfg.environmentVariableName, cfg.secretName, cfg.containerName)
	}
	return strings.TrimSpace(*properties.LatestRevisionName), nil
}

func hasManagedIdentity(identity *armappcontainers.ManagedServiceIdentity, identityID string) bool {
	if identity == nil {
		return false
	}
	for resourceID := range identity.UserAssignedIdentities {
		if strings.EqualFold(strings.TrimRight(resourceID, "/"), identityID) {
			return true
		}
	}
	return false
}

func hasKeyVaultSecretReference(secrets []*armappcontainers.Secret, cfg config) bool {
	for _, secret := range secrets {
		if secret == nil || secret.Name == nil || secret.KeyVaultURL == nil || secret.Identity == nil {
			continue
		}
		if strings.EqualFold(*secret.Name, cfg.secretName) &&
			strings.EqualFold(strings.TrimRight(*secret.KeyVaultURL, "/"), cfg.keyVaultSecretURL) &&
			strings.EqualFold(strings.TrimRight(*secret.Identity, "/"), cfg.managedIdentityID) {
			return true
		}
	}
	return false
}

func hasEnvironmentVariableReference(containers []*armappcontainers.Container, cfg config) bool {
	for _, container := range containers {
		if container == nil || container.Name == nil || !strings.EqualFold(*container.Name, cfg.containerName) {
			continue
		}
		for _, environmentVariable := range container.Env {
			if environmentVariable != nil && environmentVariable.Name != nil && environmentVariable.SecretRef != nil &&
				strings.EqualFold(*environmentVariable.Name, cfg.environmentVariableName) &&
				strings.EqualFold(*environmentVariable.SecretRef, cfg.secretName) {
				return true
			}
		}
	}
	return false
}

func writeGitHubEnvironment(path, name, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("environment value %s must be a single line", name)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open GitHub environment file: %w", err)
	}
	defer file.Close()
	if _, err := fmt.Fprintf(file, "%s=%s\n", name, value); err != nil {
		return fmt.Errorf("write GitHub environment file: %w", err)
	}
	return nil
}
