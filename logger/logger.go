package logger

import (
	"fmt"
	"log/slog"
	"os"
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

// New returns a SlogLogger reference that satisfies the Logger interface.
func New() *SlogLogger {
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	})
	logger := slog.New(handler)
	return &SlogLogger{logger: logger}
}

// SlogLogger is the default implementation using slog structured logs
type SlogLogger struct {
	logger *slog.Logger
}

// Info starts a new message at the info level in the logger
func (s *SlogLogger) Info(v ...any) {
	s.logger.Info(fmt.Sprint(v...))
}

// Fatal starts a new message at the fatal level in the logger, exits with status code 1
func (s *SlogLogger) Fatal(v ...any) {
	s.logger.Error(fmt.Sprint(v...), slog.String("fatal", "true"))
	os.Exit(1) // Need to explicitly exit as slog doesn't have a built-in Fatal method
}
