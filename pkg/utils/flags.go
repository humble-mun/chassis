package utils

import (
	"errors"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	envPrefix      = "HM"
	configFileType = "yaml"
)

// RegisterToViper registers flags to viper and returns a function to load config
func RegisterToViper(pfs *pflag.FlagSet, configName string) func() error {
	viper.SetConfigName(configName)
	viper.SetConfigType(configFileType)
	viper.AddConfigPath("/etc/humble-mun")
	for _, configPath := range extraConfigPaths {
		viper.AddConfigPath(configPath)
	}
	return func() (err error) {
		if err = viper.ReadInConfig(); err != nil {
			if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); !ok {
				return
			}
		} else {
			viper.WatchConfig()
		}
		viper.SetEnvPrefix(envPrefix)
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
		viper.AutomaticEnv()
		err = viper.BindPFlags(pfs)
		return
	}
}
