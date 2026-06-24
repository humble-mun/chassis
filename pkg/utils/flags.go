package utils

import (
	"errors"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const configFileType = "yaml"

// envPrefix and configRoot are package-level variables so they can be
// overridden at build time via the linker (-X), e.g.
// -X 'github.com/humble-mun/chassis/pkg/utils.envPrefix=ACME'.
// They are deliberately not const: the prefix and config root are the very
// mechanism that resolves env vars, flags, and config files, so they cannot
// be configured through those same sources without a chicken-and-egg problem.
// Runtime overrides supplied via ViperOption take precedence over these.
var (
	envPrefix  = "HM"
	configRoot = "/etc/humble-mun"
)

// ViperOption overrides the env prefix or config root resolved by
// RegisterToViper. A non-empty value takes precedence over the package-level
// default (which may itself be injected at build time via ldflags).
type ViperOption func(*viperConfig)

type viperConfig struct {
	envPrefix  string
	configRoot string
}

// WithEnvPrefix overrides the environment variable prefix. An empty string is
// ignored, leaving the package-level default (or ldflags-injected value) in
// effect.
func WithEnvPrefix(prefix string) ViperOption {
	return func(c *viperConfig) {
		if prefix != "" {
			c.envPrefix = prefix
		}
	}
}

// WithConfigRoot overrides the primary config search directory. An empty
// string is ignored, leaving the package-level default (or ldflags-injected
// value) in effect.
func WithConfigRoot(root string) ViperOption {
	return func(c *viperConfig) {
		if root != "" {
			c.configRoot = root
		}
	}
}

// RegisterToViper registers flags to viper and returns a function to load config.
// The env prefix and config root default to the package-level values (which may
// be injected at build time via ldflags); ViperOptions override them at runtime.
func RegisterToViper(pfs *pflag.FlagSet, configName string, opts ...ViperOption) func() error {
	cfg := viperConfig{envPrefix: envPrefix, configRoot: configRoot}
	for _, opt := range opts {
		opt(&cfg)
	}
	return func() (err error) {
		// Apply the config name/type/search paths here in the loader rather
		// than eagerly at registration time. A binary with multiple cobra
		// subcommands calls RegisterToViper once per command, all sharing the
		// global viper; setting the config name eagerly lets the last
		// registration clobber the others, so a command's loader would read
		// the wrong file. Deferring to the loader means each command applies
		// its own config name immediately before ReadInConfig.
		viper.SetConfigName(configName)
		viper.SetConfigType(configFileType)
		viper.AddConfigPath(cfg.configRoot)
		for _, configPath := range extraConfigPaths {
			viper.AddConfigPath(configPath)
		}
		if err = viper.ReadInConfig(); err != nil {
			if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); !ok {
				return
			}
		} else {
			viper.WatchConfig()
		}
		viper.SetEnvPrefix(cfg.envPrefix)
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
		viper.AutomaticEnv()
		err = viper.BindPFlags(pfs)
		return
	}
}
