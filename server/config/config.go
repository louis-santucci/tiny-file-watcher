package config

import (
	"fmt"

	"github.com/ridgelines/go-config"
)

const (
	defaultApplicationPath = "/Users/louissantucci/.tfw"
	tfwConfigFileName      = "tfw.yml"
	DefaultDBPath          = defaultApplicationPath
)

func InitConfig() *config.Config {
	yamlFile := config.NewYAMLFile(defaultApplicationPath + "/" + tfwConfigFileName)
	yamlFileLoader := config.NewOnceLoader(yamlFile)
	providers := []config.Provider{yamlFileLoader}
	cfg := config.NewConfig(providers)

	cfg.Validate = func(settings map[string]string) error {
		if _, ok := settings["grpc.address"]; !ok {
			return fmt.Errorf("required setting 'grpc.address' not set")
		}
		if _, ok := settings["db.name"]; !ok {
			return fmt.Errorf("required setting 'db.name' not set")
		}

		err := validateDebugUiConfig(settings)
		if err != nil {
			return err
		}

		err = validateWebConfig(settings)
		if err != nil {
			return err
		}

		return nil
	}

	return cfg
}

func validateDebugUiConfig(settings map[string]string) error {
	if _, ok := settings["debug-ui.address"]; !ok {
		return fmt.Errorf("required setting 'debug-ui.address' not set")
	}
	if _, ok := settings["debug-ui.enabled"]; !ok {
		return fmt.Errorf("required setting 'debug-ui.enabled' not set")
	}
	return nil
}

func validateWebConfig(settings map[string]string) error {
	if settings["web.enabled"] == "true" {
		if _, ok := settings["web.address"]; !ok {
			return fmt.Errorf("required setting 'web.address' not set")
		}
		if _, ok := settings["web.enabled"]; !ok {
			return fmt.Errorf("required setting 'web.enabled' not set")
		}
		for _, key := range []string{
			"oidc.enabled",
			"oidc.issuer",
			"oidc.client-id",
			"oidc.client-secret",
			"oidc.redirect-uri",
		} {
			if _, ok := settings[key]; !ok {
				return fmt.Errorf("required setting %q not set (required when web.enabled=true)", key)
			}
		}
	}
	return nil
}
