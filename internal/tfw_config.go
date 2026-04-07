package internal

import (
	"github.com/ridgelines/go-config"
)

func InitConfig(configPath string, validator *func(map[string]string) error) *config.Config {
	yamlFile := config.NewYAMLFile(configPath)
	yamlFileLoader := config.NewOnceLoader(yamlFile)
	providers := []config.Provider{yamlFileLoader}
	cfg := config.NewConfig(providers)

	if validator != nil {
		cfg.Validate = *validator
	}

	return cfg
}
