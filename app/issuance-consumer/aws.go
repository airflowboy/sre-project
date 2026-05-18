package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// fetchDSN pulls the postgres connection string out of Secrets Manager.
// The Pod's IRSA-derived credentials are picked up automatically by the
// default config loader from AWS_ROLE_ARN / AWS_WEB_IDENTITY_TOKEN_FILE
// env vars that EKS injects.
func fetchDSN(ctx context.Context, secretARN string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(cctx)
	if err != nil {
		return "", fmt.Errorf("load aws config: %w", err)
	}
	sm := secretsmanager.NewFromConfig(cfg)
	out, err := sm.GetSecretValue(cctx, &secretsmanager.GetSecretValueInput{
		SecretId: &secretARN,
	})
	if err != nil {
		return "", fmt.Errorf("get secret: %w", err)
	}
	if out.SecretString == nil {
		return "", fmt.Errorf("secret %s has no string value", secretARN)
	}
	return *out.SecretString, nil
}
