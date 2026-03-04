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
		if _, ok := settings["debug-ui.address"]; !ok {
			return fmt.Errorf("required setting 'debug-ui.address' not set")
		}
		if _, ok := settings["debug-ui.enabled"]; !ok {
			return fmt.Errorf("required setting 'debug-ui.enabled' not set")
		}
		if _, ok := settings["db.name"]; !ok {
			return fmt.Errorf("required setting 'db.name' not set")
		}

		return nil
	}

	return cfg
}
