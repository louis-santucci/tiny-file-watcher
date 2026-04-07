package config

import (
	"fmt"
	"sync"
	"tiny-file-watcher/internal"

	"github.com/ridgelines/go-config"
)

var (
	clientCfg     *config.Config
	clientCfgOnce sync.Once
	clientCfgErr  error
)

func LoadClientConfig() (*config.Config, error) {
	clientCfgOnce.Do(func() {
		clientCfgValidator := clientConfigValidator
		clientCfg = internal.InitConfig(internal.ClientConfigPath(), &clientCfgValidator)
		clientCfgErr = clientCfg.Load()
	})
	return clientCfg, clientCfgErr
}

func clientConfigValidator(settings map[string]string) error {
	if _, ok := settings["grpc.address"]; !ok {
		return fmt.Errorf("required setting 'grpc.address' not set")
	}
	err := validateOidcConfig(settings)
	if err != nil {
		return err
	}

	return nil
}

func validateOidcConfig(settings map[string]string) error {
	if settings["oidc.enabled"] == "true" {
		_, ok := settings["oidc.issuer"]
		if !ok {
			return fmt.Errorf("required setting 'oidc.issuer' not set (required when oidc.enabled=true)")
		}
		_, ok = settings["oidc.device-client-id"]
		if !ok {
			return fmt.Errorf("required setting 'oidc.device-client-id' not set (required when oidc.enabled=true)")
		}
	}
	return nil
}
