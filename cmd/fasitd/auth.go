package main

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
)

var _ credentials.PerRPCCredentials = ksaPerRPCCredentials{}

const tokenPath = "/var/run/secrets/fasit.nais.io/serviceaccount/token" // #nosec G101 -- filesystem path to the projected token, not a credential

func KSAPerRPCCredentials() (credentials.PerRPCCredentials, error) {
	if _, err := os.ReadFile(tokenPath); err != nil {
		return nil, fmt.Errorf("could not read %s: %w", tokenPath, err)
	}
	return ksaPerRPCCredentials{
		tokenPath: tokenPath,
	}, nil
}

type ksaPerRPCCredentials struct {
	tokenPath string
}

func (k ksaPerRPCCredentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	token, err := os.ReadFile(k.tokenPath)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", k.tokenPath, err)
	}
	return map[string]string{
		"authorization": fmt.Sprintf("bearer %s", token),
	}, nil
}

func (k ksaPerRPCCredentials) RequireTransportSecurity() bool {
	return true
}
