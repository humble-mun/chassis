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

	"github.com/humble-mun/chassis/pkg/logging"
	"github.com/humble-mun/chassis/pkg/metrics"
	"github.com/humble-mun/chassis/pkg/server"
	"github.com/humble-mun/chassis/pkg/service"
	"github.com/humble-mun/chassis/pkg/utils"
	"github.com/humble-mun/chassis/pkg/version"
)

// PrepareFlags register various parameters to command line flags, config files, and environment variables
func PrepareFlags(name string, cmd *cobra.Command, register ...func(*pflag.FlagSet)) (init func() error) {
	pfs := cmd.PersistentFlags()
	ffs := flag.NewFlagSet(name, flag.PanicOnError)
	logging.Init(ffs, pfs)
	cmd.Flags().AddGoFlagSet(ffs)
	pfs.String(service.FlagNodeName, "", "The node name that current agent serving on.")
	pfs.Bool(service.FlagEnablePProf, false, "Whether to enable pprof api.")
	server.RegisterFlags(pfs)
	metrics.RegisterFlags(pfs)
	for _, f := range register {
		f(pfs)
	}
	init = utils.RegisterToViper(pfs, name)
	return
}

// BaseContext create the initial state and optionally a gin server.
func BaseContext(opts ...Option) (
	rootLogger, logger logr.Logger, httpGin *server.HTTPServer, ctx context.Context, nodeName string, err error) {

	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	rootLogger = logging.NewLogger()
	logger = rootLogger.WithName("cmd").WithValues(
		"builtAt", version.BuiltAt, "commitId", version.CommitID,
		"arch", version.Architecture)

	if o.init != nil {
		if err = o.init(); err != nil {
			logger.Error(err, "init viper failed")
			return
		}
	}

	nodeName = viper.GetString(service.FlagNodeName)
	logger = logger.WithValues("nodeName", nodeName)
	ctx = ctrl.SetupSignalHandler()

	if o.withoutHTTPServer {
		return
	}

	serverOpts := append([]server.Option{server.WithDefaultListener(), server.WithDefaultCORSConfig()}, o.serverOptions...)
	httpGin = server.NewHTTPServer(rootLogger, serverOpts...)
	httpGin.RegisterRoute(func(mux *gin.Engine) {
		mux.GET("/version", utils.NoLog, func(ctx *gin.Context) {
			ctx.Writer.Header().Set("Content-Type", "text/plain")
			version.WriteVersionToStream(ctx.Writer)
		})
	})
	httpGin.RegisterRoute(func(mux *gin.Engine) {
		mux.GET("/logging", gin.WrapF(klog.Configure))
		mux.PUT("/logging", gin.WrapF(klog.Configure))
	})
	httpGin.RegisterRoute(metrics.RegisterRoute)
	httpGin.RegisterRoute(utils.RegisterProbeRoute)
	enablePProfAPI := viper.GetBool(service.FlagEnablePProf)
	if enablePProfAPI {
		httpGin.RegisterRoute(utils.APIVersion("/debug", utils.NoLog)(func(group *gin.RouterGroup) {
			pprof.RouteRegister(group, "/pprof")
		}))
	}

	return
}
