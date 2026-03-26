package config

import (
	"fmt"
)

func ServerConfigValidator(settings map[string]string) error {
	if _, ok := settings["grpc.address"]; !ok {
		return fmt.Errorf("required setting 'grpc.address' not set")
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

func validateDebugUiConfig(settings map[string]string) error {
	if settings["debug-ui.enabled"] == "true" {
		if _, ok := settings["debug-ui.address"]; !ok {
			return fmt.Errorf("required setting 'debug-ui.address' not set")
		}
	}
	return nil
}

func validateWebConfig(settings map[string]string) error {
	if settings["web.enabled"] == "true" {
		if _, ok := settings["web.address"]; !ok {
			return fmt.Errorf("required setting 'web.address' not set")
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
