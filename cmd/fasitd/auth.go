package main

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
)

var _ credentials.PerRPCCredentials = ksaPerRPCCredentials{}

const tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token" // #nosec G101 -- filesystem path to the projected token, not a credential

func KSAPerRPCCredentials() (credentials.PerRPCCredentials, error) {
	token, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %v", tokenPath, err)
	}
	return ksaPerRPCCredentials{
		token: token,
	}, nil
}

type ksaPerRPCCredentials struct {
	token []byte
}

func (k ksaPerRPCCredentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": fmt.Sprintf("bearer %s", k.token),
	}, nil
}

func (k ksaPerRPCCredentials) RequireTransportSecurity() bool {
	return true
}
