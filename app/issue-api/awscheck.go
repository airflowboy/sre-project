package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// awsChecker proves IRSA works end-to-end: the Pod's ServiceAccount token
// is exchanged for AWS credentials, which fetch a Secrets Manager entry.
// The endpoint NEVER returns the secret value — only its length, so a curl
// against the public ALB cannot leak it.
type awsChecker struct {
	sm        *secretsmanager.Client
	secretARN string
}

func newAWSChecker(ctx context.Context, secretARN string) (*awsChecker, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return &awsChecker{
		sm:        secretsmanager.NewFromConfig(cfg),
		secretARN: secretARN,
	}, nil
}

// GET /aws-check
//
// On success: 200 with {"ok":true,"secret_arn":"...","value_length":24}
// On failure: 500 with the AWS error message (e.g., AccessDenied, NotFound).
// The secret value is never serialized.
func (c *awsChecker) handle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	out, err := c.sm.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &c.secretARN,
	})
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	n := 0
	if out.SecretString != nil {
		n = len(*out.SecretString)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":           true,
		"secret_arn":   c.secretARN,
		"value_length": n,
	})
}
