package app

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	"github.com/humble-mun/chassis/pkg/constants"
)

func TestPrepareFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	var customRegistered bool
	init := PrepareFlags("test", cmd, func(*pflag.FlagSet) {
		customRegistered = true
	})

	if init == nil {
		t.Fatal("expected non-nil init func")
	}
	if !customRegistered {
		t.Fatal("expected custom register func to be invoked")
	}

	pfs := cmd.PersistentFlags()
	for _, name := range []string{
		constants.FlagNodeName,
		constants.FlagEnablePProf,
		constants.FlagInfraAPIToken,
		constants.FlagEnableDebugCORS,
	} {
		if pfs.Lookup(name) == nil {
			t.Errorf("expected flag %q to be registered", name)
		}
	}
	if pfs.Lookup("vklog") == nil {
		t.Error("expected logging --vklog flag to be registered")
	}
}

func TestOptions(t *testing.T) {
	o := &options{}
	WithInit(func() error { return nil })(o)
	WithoutHTTPServer()(o)
	WithGRPCServer(grpc.NewServer())(o)
	WithTCPListener()(o)
	WithUnixListener()(o)

	if o.init == nil {
		t.Error("expected init to be set")
	}
	if !o.withoutHTTPServer {
		t.Error("expected withoutHTTPServer to be true")
	}
	if len(o.serverOptions) != 3 {
		t.Errorf("expected 3 server options, got %d", len(o.serverOptions))
	}
}

// TestBaseContext exercises the full bootstrap once. ctrl.SetupSignalHandler
// panics if called more than once per process, so BaseContext must only be
// invoked in a single test within this package.
func TestBaseContext(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set(constants.FlagNodeName, "node-1")

	base, err := BaseContext()
	if err != nil {
		t.Fatalf("BaseContext error: %v", err)
	}
	if base.HTTPGin == nil {
		t.Error("expected HTTPGin to be constructed")
	}
	if base.Ctx == nil {
		t.Error("expected signal-aware context to be set")
	}
	if base.NodeName != "node-1" {
		t.Errorf("NodeName = %q, want node-1", base.NodeName)
	}
	if base.RootLogger.GetSink() == nil {
		t.Error("expected root logger sink to be set")
	}
	if base.Logger.GetSink() == nil {
		t.Error("expected named logger sink to be set")
	}
}
