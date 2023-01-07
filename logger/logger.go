package logger

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
)

// Logger is an interface to make swapping out loggers simple
type Logger interface {
	Fatal(v ...any)
	// Fatalf(format string, v ...any)
	// Error(v ...any)
	// Errorf(format string, v ...any)
	Info(v ...any)
	// Infof(format string, v ...any)
	// Debug(v ...any)
	// Debugf(format string, v ...any)
}

// New returns a ZeroLogger reference that satisfies the Logger interface.
func New() *ZeroLogger {
	logger := zerolog.New(os.Stderr).
		With().
		Timestamp().
		Logger()
	return &ZeroLogger{logger: logger}
}

// ZeroLogger is the default implementation using zerolog structured logs
type ZeroLogger struct {
	logger zerolog.Logger
}

// Info starts a new message at the info level in the logger
func (z *ZeroLogger) Info(v ...any) {
	z.logger.Info().Msg(fmt.Sprint(v...))
}

// Fatal starts a new message at the fatal level in the logger, exits with status code 1
func (z *ZeroLogger) Fatal(v ...any) {
	z.logger.Fatal().Msg(fmt.Sprint(v...))
}
