package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// TestRegisterToViperSubcommandDoesNotClobberConfigName guards against a
// regression where RegisterToViper applied the config name to the global viper
// eagerly at registration time. A binary with multiple cobra subcommands calls
// RegisterToViper once per command on the shared global viper, so the last
// registration's config name would clobber the others and a command's loader
// would read the wrong file (silently falling back to flag defaults).
func TestRegisterToViperSubcommandDoesNotClobberConfigName(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "daemon.yaml"),
		[]byte("http-bind-address: 0.0.0.0:9443\n"), 0o600); err != nil {
		t.Fatalf("write daemon.yaml: %v", err)
	}

	rootPfs := pflag.NewFlagSet("daemon", pflag.ContinueOnError)
	rootPfs.String("http-bind-address", "0.0.0.0:8080", "")
	rootLoad := RegisterToViper(rootPfs, "daemon", WithConfigRoot(dir))

	// Registering a second command (mimicking a cobra subcommand) must not
	// change which config file the root command's loader reads.
	subPfs := pflag.NewFlagSet("config", pflag.ContinueOnError)
	_ = RegisterToViper(subPfs, "config", WithConfigRoot(dir))

	if err := rootLoad(); err != nil {
		t.Fatalf("rootLoad: %v", err)
	}
	if got := viper.GetString("http-bind-address"); got != "0.0.0.0:9443" {
		t.Fatalf("config name clobbered: http-bind-address = %q, want 0.0.0.0:9443 "+
			"(daemon.yaml was not read)", got)
	}
	if viper.ConfigFileUsed() == "" {
		t.Fatal("no config file loaded; expected daemon.yaml")
	}
}
