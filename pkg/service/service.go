package service

import "time"

var (
	// ConfigName is the default configuration file name, should be set by consumers
	ConfigName string
)

const (
	// FlagNodeName is the flag name for node name
	FlagNodeName = "node-name"
	// FlagEnablePProf is the flag name to enable pprof
	FlagEnablePProf = "enable-pprof"
	// FlagControllerRequeueDuration is the flag name for controller requeue duration
	FlagControllerRequeueDuration = "requeue-duration"
	// FlagGlobalResourceNamespace is the flag name for global resource namespace
	FlagGlobalResourceNamespace = "global-resource-namespace"

	// DefaultHTTPServerBind is the default HTTP server bind address
	DefaultHTTPServerBind = "0.0.0.0:8080"
	// DefaultTLSCertPath is the default TLS certificate path
	DefaultTLSCertPath = "/etc/humble-mun/pki/tls.crt"
	// DefaultTLSKeyPath is the default TLS key path
	DefaultTLSKeyPath = "/etc/humble-mun/pki/tls.key"
	// DefaultControllerRequeueDuration is the default controller requeue duration
	DefaultControllerRequeueDuration = 1 * time.Second
	// DefaultGlobalResourceNamespace is the default global resource namespace
	DefaultGlobalResourceNamespace = "hm-system"
)
