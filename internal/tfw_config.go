package internal

import (
	"fmt"
	"os"

	"github.com/ridgelines/go-config"
)

func InitConfig(validator *func(map[string]string) error) *config.Config {
	var configPath = ConfigPath()
	providers := []config.Provider{}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Errorf("config file does not exist at path: %s", configPath)
	} else {
		yamlFile := config.NewYAMLFile(ConfigPath())
		yamlFileLoader := config.NewOnceLoader(yamlFile)
		providers = append(providers, yamlFileLoader)
	}
	environmentVariableLoader := config.NewEnvironment(initEnvVariables())
	providers = append(providers, environmentVariableLoader)
	cfg := config.NewConfig(providers)

	if validator != nil {
		cfg.Validate = *validator
	}

	return cfg
}

func initEnvVariables() map[string]string {
	mappings := map[string]string{
		"GRPC_ADDRESS":       "grpc.address",
		"LOG_LEVEL":          "log.level",
		"DEBUG_UI_ENABLED":   "debug-ui.enabled",
		"DEBUG_UI_ADDRESS":   "debug-ui.address",
		"WEB_ENABLED":        "web.enabled",
		"WEB_ADDRESS":        "web.address",
		"OIDC_ENABLED":       "oidc.enabled",
		"OIDC_ISSUER":        "oidc.issuer",
		"OIDC_CLIENT_ID":     "oidc.client-id",
		"OIDC_CLIENT_SECRET": "oidc.client-secret",
		"OIDC_REDIRECT_URI":  "oidc.redirect-uri",
		"DATABASE_PATH":      "database.path",
	}
	return mappings
}
