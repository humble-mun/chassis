package manager

import (
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/humble-mun/chassis/pkg/service"
)

const (
	flagClientQPS             = "client-qps"
	flagClientBurst           = "client-burst"
	flagDisableLeaderElection = "disable-leader-election"
	flagLeaderElectNamespace  = "leader-elect-namespace"
	flagLeaderElectID         = "leader-elect-id"
	flagResyncPeriod          = "resync-period"

	defaultClientQPS     = 5.0
	defaultClientBurst   = 10
	defaultLeaderElectID = "operator"
	defaultResyncPeriod  = 10 * time.Hour
)

// RegisterFlags registers manager configuration flags.
func RegisterFlags(pfs *pflag.FlagSet) {
	pfs.Float32(flagClientQPS, defaultClientQPS, "The maximum QPS to the master from this client.")
	pfs.Int(flagClientBurst, defaultClientBurst, "The maximum burst for throttle.")
	pfs.Duration(flagResyncPeriod, defaultResyncPeriod, "The base frequency the informers are resynced.")
	pfs.Bool(flagDisableLeaderElection, false, "Disable leader election and let every replica run controllers concurrently.")
	pfs.String(flagLeaderElectNamespace, service.DefaultGlobalResourceNamespace, "The namespace in which the leader election resource will be created.")
	pfs.String(flagLeaderElectID, defaultLeaderElectID, "The name of the resource that leader election will use for holding the leader lock.")
}

// NewManager creates and configures a new controller-runtime manager.
func NewManager(logger logr.Logger, selectors map[client.Object]cache.ByObject,
	addToSchemes ...func(*runtime.Scheme) error) (
	mgr manager.Manager, cfg *rest.Config, scheme *runtime.Scheme, err error) {

	logger = logger.WithName("manager")
	if cfg, err = ctrl.GetConfig(); err != nil {
		logger.Error(err, "get kubeconfig failed")
		return
	}
	qps := viper.GetFloat64(flagClientQPS)
	cfg.QPS = float32(qps)
	cfg.Burst = viper.GetInt(flagClientBurst)
	scheme = runtime.NewScheme()
	for _, addToScheme := range addToSchemes {
		if err = addToScheme(scheme); err != nil {
			logger.Error(err, "add to scheme failed")
			return
		}
	}
	if mgr, err = ctrl.NewManager(cfg, controllerOptions(logger, scheme, selectors)); err != nil {
		logger.Error(err, "create controller manager failed")
		return
	}
	log.SetLogger(logger)
	return
}

func controllerOptions(logger logr.Logger, scheme *runtime.Scheme,
	selectors map[client.Object]cache.ByObject) (managerOptions ctrl.Options) {

	managerOptions = ctrl.Options{
		Logger:                 logger,
		Scheme:                 scheme,
		LeaderElection:         !viper.GetBool(flagDisableLeaderElection),
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		Cache: cache.Options{
			SyncPeriod: ptr.To(viper.GetDuration(flagResyncPeriod)),
			ByObject:   selectors,
		},
	}
	if !managerOptions.LeaderElection {
		return
	}
	managerOptions.LeaderElectionID = viper.GetString(flagLeaderElectID)
	managerOptions.LeaderElectionNamespace = viper.GetString(flagLeaderElectNamespace)
	managerOptions.LeaderElectionReleaseOnCancel = true
	return
}
