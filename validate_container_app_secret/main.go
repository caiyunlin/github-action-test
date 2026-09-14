package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appcontainers/armappcontainers/v3"
)

const (
	defaultValidationTimeout  = 5 * time.Minute
	defaultValidationInterval = 10 * time.Second
)

type config struct {
	environmentName     string
	subscriptionID      string
	resourceGroupName   string
	containerAppName    string
	revisionName        string
	secretName          string
	expectedSecretValue string
	validationTimeout   time.Duration
	validationInterval  time.Duration
	stepSummaryPath     string
}

type validationState struct {
	secretMatches bool
	health        string
	provisioning  string
	running       string
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "validate Container App secret: %v\n", err)
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
	containerAppsClient, err := armappcontainers.NewContainerAppsClient(cfg.subscriptionID, credential, nil)
	if err != nil {
		return fmt.Errorf("create Container Apps client: %w", err)
	}
	revisionsClient, err := armappcontainers.NewContainerAppsRevisionsClient(cfg.subscriptionID, credential, nil)
	if err != nil {
		return fmt.Errorf("create Container Apps revisions client: %w", err)
	}

	validationCtx, cancel := context.WithTimeout(ctx, cfg.validationTimeout)
	defer cancel()

	var lastState validationState
	for {
		lastState, err = getValidationState(validationCtx, containerAppsClient, revisionsClient, cfg)
		if err != nil {
			return err
		}
		if isSuccessful(lastState) {
			if err := writeStepSummary(cfg, lastState); err != nil {
				return err
			}
			fmt.Println("Container App resolved secret matches Key Vault and revision is healthy")
			return nil
		}
		if isFailed(lastState) {
			return fmt.Errorf("revision %q entered a failed state: health=%s provisioning=%s running=%s", cfg.revisionName, lastState.health, lastState.provisioning, lastState.running)
		}

		fmt.Printf("Waiting for ACA secret refresh: health=%s provisioning=%s running=%s\n", lastState.health, lastState.provisioning, lastState.running)
		select {
		case <-validationCtx.Done():
			return fmt.Errorf("validation timed out: health=%s provisioning=%s running=%s: %w", lastState.health, lastState.provisioning, lastState.running, validationCtx.Err())
		case <-time.After(cfg.validationInterval):
		}
	}
}

func getValidationState(ctx context.Context, containerAppsClient *armappcontainers.ContainerAppsClient, revisionsClient *armappcontainers.ContainerAppsRevisionsClient, cfg config) (validationState, error) {
	resolvedSecrets, err := containerAppsClient.ListSecrets(ctx, cfg.resourceGroupName, cfg.containerAppName, nil)
	if err != nil {
		return validationState{}, fmt.Errorf("list resolved Container App secrets: %w", err)
	}

	revision, err := revisionsClient.GetRevision(ctx, cfg.resourceGroupName, cfg.containerAppName, cfg.revisionName, nil)
	if err != nil {
		return validationState{}, fmt.Errorf("get revision %q: %w", cfg.revisionName, err)
	}

	state := validationState{secretMatches: resolvedSecretMatches(resolvedSecrets.Value, cfg.secretName, cfg.expectedSecretValue)}
	if revision.Properties != nil {
		state.health = stringValue(revision.Properties.HealthState)
		state.provisioning = stringValue(revision.Properties.ProvisioningState)
		state.running = stringValue(revision.Properties.RunningState)
	}
	return state, nil
}

func loadConfig() (config, error) {
	cfg := config{
		environmentName:     strings.TrimSpace(os.Getenv("DEPLOY_ENV")),
		subscriptionID:      strings.TrimSpace(os.Getenv("AZURE_SUBSCRIPTION_ID")),
		resourceGroupName:   strings.TrimSpace(os.Getenv("ACA_RESOURCE_GROUP")),
		containerAppName:    strings.TrimSpace(os.Getenv("ACA_NAME")),
		revisionName:        strings.TrimSpace(os.Getenv("ACA_REVISION_NAME")),
		secretName:          strings.TrimSpace(os.Getenv("ACA_SECRET_NAME")),
		expectedSecretValue: os.Getenv("TEMPORAL_API_KEY"),
		validationTimeout:   defaultValidationTimeout,
		validationInterval:  defaultValidationInterval,
		stepSummaryPath:     strings.TrimSpace(os.Getenv("GITHUB_STEP_SUMMARY")),
	}

	required := map[string]string{
		"AZURE_SUBSCRIPTION_ID": cfg.subscriptionID,
		"ACA_RESOURCE_GROUP":    cfg.resourceGroupName,
		"ACA_NAME":              cfg.containerAppName,
		"ACA_REVISION_NAME":     cfg.revisionName,
		"ACA_SECRET_NAME":       cfg.secretName,
		"TEMPORAL_API_KEY":      cfg.expectedSecretValue,
	}
	for name, value := range required {
		if value == "" {
			return config{}, fmt.Errorf("environment variable %s is required", name)
		}
	}
	if strings.ContainsAny(cfg.expectedSecretValue, "\r\n") {
		return config{}, fmt.Errorf("environment variable TEMPORAL_API_KEY must be a single-line value")
	}

	var err error
	cfg.validationTimeout, err = durationFromEnvironment("ACA_VALIDATION_TIMEOUT", cfg.validationTimeout)
	if err != nil {
		return config{}, err
	}
	cfg.validationInterval, err = durationFromEnvironment("ACA_VALIDATION_INTERVAL", cfg.validationInterval)
	if err != nil {
		return config{}, err
	}
	return cfg, nil
}

func durationFromEnvironment(name string, defaultValue time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return defaultValue, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("environment variable %s must be a positive duration", name)
	}
	return duration, nil
}

func resolvedSecretMatches(secrets []*armappcontainers.ContainerAppSecret, name, expectedValue string) bool {
	for _, secret := range secrets {
		if secret != nil && secret.Name != nil && secret.Value != nil && strings.EqualFold(*secret.Name, name) {
			return *secret.Value == expectedValue
		}
	}
	return false
}

func isSuccessful(state validationState) bool {
	running := state.running == "Running" || state.running == "RunningAtMaxScale" || state.running == "ScaledToZero"
	return state.secretMatches && state.health == "Healthy" && state.provisioning == "Provisioned" && running
}

func isFailed(state validationState) bool {
	return state.health == "Unhealthy" || state.provisioning == "Failed" || state.running == "Failed"
}

func writeStepSummary(cfg config, state validationState) error {
	if cfg.stepSummaryPath == "" {
		return nil
	}
	summary := fmt.Sprintf("## ACA Temporal API Key Update\n\n- Environment: %s\n- Container App: %s\n- Revision: %s\n- Secret comparison: matched\n- Health: %s\n- Provisioning: %s\n- Running: %s\n", cfg.environmentName, cfg.containerAppName, cfg.revisionName, state.health, state.provisioning, state.running)
	file, err := os.OpenFile(cfg.stepSummaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open GitHub step summary: %w", err)
	}
	defer file.Close()
	if _, err := file.WriteString(summary); err != nil {
		return fmt.Errorf("write GitHub step summary: %w", err)
	}
	return nil
}

func stringValue[T ~string](value *T) string {
	if value == nil {
		return ""
	}
	return string(*value)
}
