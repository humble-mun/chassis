package manager

import (
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestRegisterFlags(t *testing.T) {
	pfs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(pfs)

	for _, name := range []string{
		flagClientQPS,
		flagClientBurst,
		flagResyncPeriod,
		flagDisableLeaderElection,
		flagLeaderElectNamespace,
		flagLeaderElectID,
	} {
		if pfs.Lookup(name) == nil {
			t.Fatalf("expected flag %q to be registered", name)
		}
	}
}

func TestControllerOptionsDisablesLeaderElection(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set(flagResyncPeriod, time.Minute)
	viper.Set(flagDisableLeaderElection, true)

	options := controllerOptions(logr.Discard(), runtime.NewScheme(), nil)
	if options.LeaderElection {
		t.Fatal("expected leader election to be disabled")
	}
	if options.LeaderElectionID != "" {
		t.Fatalf("expected empty leader election id, got %q", options.LeaderElectionID)
	}
	if options.LeaderElectionNamespace != "" {
		t.Fatalf("expected empty leader election namespace, got %q", options.LeaderElectionNamespace)
	}
	if options.LeaderElectionReleaseOnCancel {
		t.Fatal("expected release-on-cancel to remain false when leader election is disabled")
	}
	if options.Cache.SyncPeriod == nil || *options.Cache.SyncPeriod != time.Minute {
		t.Fatalf("unexpected sync period: %#v", options.Cache.SyncPeriod)
	}
}

func TestControllerOptionsEnablesLeaderElection(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set(flagResyncPeriod, time.Minute)
	viper.Set(flagDisableLeaderElection, false)
	viper.Set(flagLeaderElectID, "test-manager")
	viper.Set(flagLeaderElectNamespace, "chassis")

	options := controllerOptions(logr.Discard(), runtime.NewScheme(), nil)
	if !options.LeaderElection {
		t.Fatal("expected leader election to be enabled")
	}
	if options.LeaderElectionID != "test-manager" {
		t.Fatalf("unexpected leader election id: %q", options.LeaderElectionID)
	}
	if options.LeaderElectionNamespace != "chassis" {
		t.Fatalf("unexpected leader election namespace: %q", options.LeaderElectionNamespace)
	}
	if !options.LeaderElectionReleaseOnCancel {
		t.Fatal("expected release-on-cancel to be enabled")
	}
}
