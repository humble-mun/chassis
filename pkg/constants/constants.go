package constants

import "time"

const (
	// FlagNodeName is the flag name for node name
	FlagNodeName = "node-name"
	// FlagEnablePProf is the flag name to enable pprof
	FlagEnablePProf = "enable-pprof"
	// FlagControllerRequeueDuration is the flag name for controller requeue duration
	FlagControllerRequeueDuration = "requeue-duration"
	// FlagGlobalResourceNamespace is the flag name for global resource namespace
	FlagGlobalResourceNamespace = "global-resource-namespace"
	// FlagInfraAPIToken is the flag name for the infrastructure API token guarding
	// built-in endpoints such as /logging and /debug/pprof
	//nolint:gosec // G101: this is the flag name, not a credential
	FlagInfraAPIToken = "infra-api-token"
	// FlagEnableDebugCORS is the flag name to enable a permissive CORS policy
	// (allow all origins and credentials) for development and debugging
	FlagEnableDebugCORS = "enable-debug-cors"

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
