package observability

const (
	// EnvLogLevel overrides the default log level (debug, info, warn, error).
	EnvLogLevel = "VIBE_LOG_LEVEL"
	// EnvLogDir overrides the default log directory.
	EnvLogDir = "VIBE_LOG_DIR"
	// DefaultLogLevel is used when EnvLogLevel is unset.
	DefaultLogLevel = "info"
)
