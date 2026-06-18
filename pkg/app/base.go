package app

import (
	"context"
	"flag"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/humble-mun/chassis/pkg/constants"
	"github.com/humble-mun/chassis/pkg/logging"
	"github.com/humble-mun/chassis/pkg/metrics"
	"github.com/humble-mun/chassis/pkg/server"
	"github.com/humble-mun/chassis/pkg/utils"
	"github.com/humble-mun/chassis/pkg/version"
)

// PrepareFlags register various parameters to command line flags, config files, and environment variables
func PrepareFlags(name string, cmd *cobra.Command, register ...func(*pflag.FlagSet)) (init func() error) {
	return PrepareFlagsWithViper(name, cmd, nil, register...)
}

// PrepareFlagsWithViper behaves like PrepareFlags but additionally forwards
// runtime viper overrides (env prefix, config root) to utils.RegisterToViper.
// A non-empty override takes precedence over the package-level default (which
// may itself be injected at build time via ldflags).
func PrepareFlagsWithViper(name string, cmd *cobra.Command, viperOpts []utils.ViperOption, register ...func(*pflag.FlagSet)) (init func() error) {
	pfs := cmd.PersistentFlags()
	ffs := flag.NewFlagSet(name, flag.PanicOnError)
	logging.Init(ffs, pfs)
	cmd.Flags().AddGoFlagSet(ffs)
	pfs.String(constants.FlagNodeName, "", "The node name that current agent serving on.")
	pfs.Bool(constants.FlagEnablePProf, false, "Whether to enable pprof api.")
	pfs.String(constants.FlagInfraAPIToken, "", "Token guarding infrastructure endpoints (/logging, /debug/pprof). Empty disables authentication.")
	pfs.Bool(constants.FlagEnableDebugCORS, false, "Whether to enable a permissive CORS policy (allow all origins and credentials) for development and debugging.")
	server.RegisterFlags(pfs)
	metrics.RegisterFlags(pfs)
	for _, f := range register {
		f(pfs)
	}
	init = utils.RegisterToViper(pfs, name, viperOpts...)
	return
}

// Base holds the initial application state produced by BaseContext.
type Base struct {
	// RootLogger is the unnamed root logger; pass it to components that add
	// their own name/values.
	RootLogger logr.Logger
	// Logger is the root logger named "cmd" with build and node metadata.
	Logger logr.Logger
	// HTTPGin is the shared HTTP/gRPC server. It is nil when the WithoutHTTPServer
	// option is set.
	HTTPGin *server.HTTPServer
	// Ctx is the signal-aware root context.
	Ctx context.Context
	// NodeName is the resolved node name flag value.
	NodeName string
}

// BaseContext create the initial state including a gin server
func BaseContext(opts ...Option) (base Base, err error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	rootLogger := logging.NewLogger()
	logger := rootLogger.WithName("cmd").WithValues(
		"builtAt", version.BuiltAt, "commitId", version.CommitID,
		"arch", version.Architecture)
	base.RootLogger = rootLogger

	if o.init != nil {
		if err = o.init(); err != nil {
			logger.Error(err, "init viper failed")
			return
		}
	}
	nodeName := viper.GetString(constants.FlagNodeName)
	logger = logger.WithValues("nodeName", nodeName)
	base.Logger = logger
	base.NodeName = nodeName
	base.Ctx = ctrl.SetupSignalHandler()

	if o.withoutHTTPServer {
		return
	}

	var corsOpts []server.CORSOption
	if viper.GetBool(constants.FlagEnableDebugCORS) {
		corsOpts = append(corsOpts, server.WithCORSAllowAllOrigins())
	}
	serverOpts := append([]server.Option{server.WithDefaultListener(), server.WithDefaultCORSConfig(corsOpts...)}, o.serverOptions...)
	httpGin := server.NewHTTPServer(rootLogger, serverOpts...)
	base.HTTPGin = httpGin
	httpGin.RegisterRoute(func(mux *gin.Engine) {
		mux.GET("/version", utils.NoLog, func(ctx *gin.Context) {
			ctx.Writer.Header().Set("Content-Type", "text/plain")
			version.WriteVersionToStream(ctx.Writer)
		})
	})
	infraAPIToken := viper.GetString(constants.FlagInfraAPIToken)
	httpGin.RegisterRoute(func(mux *gin.Engine) {
		mux.GET("/logging", utils.RequireAPIToken(infraAPIToken), gin.WrapF(klog.Configure))
		mux.PUT("/logging", utils.RequireAPIToken(infraAPIToken), gin.WrapF(klog.Configure))
	})
	httpGin.RegisterRoute(metrics.RegisterRoute)
	httpGin.RegisterRoute(utils.RegisterProbeRoute)
	enablePProfAPI := viper.GetBool(constants.FlagEnablePProf)
	if enablePProfAPI {
		httpGin.RegisterRoute(func(mux *gin.Engine) {
			group := mux.Group("/debug", utils.NoLog, utils.RequireAPIToken(infraAPIToken))
			pprof.RouteRegister(group, "/pprof")
		})
	}

	return
}
