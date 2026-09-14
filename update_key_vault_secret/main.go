package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets"
)

const vaultURLSuffix = ".vault.azure.net"

type config struct {
	vaultURL    string
	secretName  string
	secretValue string
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "update Key Vault secret: %v\n", err)
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
	client, err := azsecrets.NewClient(cfg.vaultURL, credential, nil)
	if err != nil {
		return fmt.Errorf("create Key Vault client: %w", err)
	}

	if _, err := client.SetSecret(ctx, cfg.secretName, azsecrets.SetSecretParameters{Value: &cfg.secretValue}, nil); err != nil {
		return fmt.Errorf("set secret %q: %w", cfg.secretName, err)
	}

	response, err := client.GetSecret(ctx, cfg.secretName, "", nil)
	if err != nil {
		return fmt.Errorf("read secret %q for validation: %w", cfg.secretName, err)
	}
	if response.Value == nil || *response.Value != cfg.secretValue {
		return fmt.Errorf("secret %q validation failed", cfg.secretName)
	}

	fmt.Printf("Key Vault secret %q updated and validated\n", cfg.secretName)
	return nil
}

func loadConfig() (config, error) {
	vaultName := strings.TrimSpace(os.Getenv("KEY_VAULT_NAME"))
	if vaultName == "" {
		return config{}, fmt.Errorf("environment variable KEY_VAULT_NAME is required")
	}
	secretName := strings.TrimSpace(os.Getenv("TARGET_SECRET_NAME"))
	if secretName == "" {
		return config{}, fmt.Errorf("environment variable TARGET_SECRET_NAME is required")
	}
	secretValue, exists := os.LookupEnv("TEMPORAL_API_KEY")
	if !exists || secretValue == "" {
		return config{}, fmt.Errorf("environment variable TEMPORAL_API_KEY is required")
	}
	if strings.ContainsAny(secretValue, "\r\n") {
		return config{}, fmt.Errorf("environment variable TEMPORAL_API_KEY must be a single-line value")
	}

	return config{
		vaultURL:    "https://" + vaultName + vaultURLSuffix,
		secretName:  secretName,
		secretValue: secretValue,
	}, nil
}
