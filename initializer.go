// This is an example initializer that uses env variables to configure the runtime. You
// can implement custom initializers to support additional providers by implementing the
// runtime.Initializer interface.
package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/unweave/unweave/api/types"
	"github.com/unweave/unweave/providers/lambdalabs"
	"github.com/unweave/unweave/runtime"
	"github.com/unweave/unweave/tools/gonfig"
)

// EnvInitializer is only used in development or if you're self-hosting Unweave.
type EnvInitializer struct{}

type providerConfig struct {
	LambdaLabsAPIKey string `env:"LAMBDALABS_API_KEY"`
}

func (i *EnvInitializer) Initialize(ctx context.Context, accountID uuid.UUID, provider types.RuntimeProvider) (*runtime.Runtime, error) {
	var cfg providerConfig
	gonfig.GetFromEnvVariables(&cfg)

	switch provider {
	case types.LambdaLabsProvider:
		if cfg.LambdaLabsAPIKey == "" {
			return nil, fmt.Errorf("missing LambdaLabs API key in runtime config file")
		}
		sess, err := lambdalabs.NewSessionProvider(cfg.LambdaLabsAPIKey)
		if err != nil {
			return nil, err
		}
		return &runtime.Runtime{Session: sess}, nil

	default:
		return nil, fmt.Errorf("%q provider not supported in the env initializer", provider)
	}
}