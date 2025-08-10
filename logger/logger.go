package logger

import (
	"fmt"
	"log/slog"
	"os"
)

// Logger is an interface to make swapping out loggers simple
type Logger interface {
	Fatal(v ...any)
	Fatalf(format string, v ...any)
	Error(v ...any)
	Errorf(format string, v ...any)
	Info(v ...any)
	Infof(format string, v ...any)
	Debug(v ...any)
	Debugf(format string, v ...any)
}

// New returns a SlogLogger reference that satisfies the Logger interface.
func New() *SlogLogger {
	return NewWithLevel("info")
}

// NewWithLevel returns a SlogLogger with the specified log level.
// Valid levels: "debug", "info", "error"
func NewWithLevel(level string) *SlogLogger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "error":
		logLevel = slog.LevelError
	case "info":
		fallthrough
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: false,
		Level:     logLevel,
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

// Fatalf starts a formatted message at the fatal level in the logger, exits with status code 1
func (s *SlogLogger) Fatalf(format string, v ...any) {
	s.logger.Error(fmt.Sprintf(format, v...), slog.String("fatal", "true"))
	os.Exit(1)
}

// Error starts a new message at the error level in the logger
func (s *SlogLogger) Error(v ...any) {
	s.logger.Error(fmt.Sprint(v...))
}

// Errorf starts a formatted message at the error level in the logger
func (s *SlogLogger) Errorf(format string, v ...any) {
	s.logger.Error(fmt.Sprintf(format, v...))
}

// Infof starts a formatted message at the info level in the logger
func (s *SlogLogger) Infof(format string, v ...any) {
	s.logger.Info(fmt.Sprintf(format, v...))
}

// Debug starts a new message at the debug level in the logger
func (s *SlogLogger) Debug(v ...any) {
	s.logger.Debug(fmt.Sprint(v...))
}

// Debugf starts a formatted message at the debug level in the logger
func (s *SlogLogger) Debugf(format string, v ...any) {
	s.logger.Debug(fmt.Sprintf(format, v...))
}

// NoOpLogger is a logger that does nothing - useful for silent mode
type NoOpLogger struct{}

// NewNoOp returns a logger that discards all output
func NewNoOp() *NoOpLogger {
	return &NoOpLogger{}
}

// Fatal does nothing in NoOp mode (but still exits for consistency)
func (n *NoOpLogger) Fatal(v ...any) {
	os.Exit(1)
}

// Fatalf does nothing in NoOp mode (but still exits for consistency)
func (n *NoOpLogger) Fatalf(format string, v ...any) {
	os.Exit(1)
}

// Error does nothing in NoOp mode
func (n *NoOpLogger) Error(v ...any) {}

// Errorf does nothing in NoOp mode
func (n *NoOpLogger) Errorf(format string, v ...any) {}

// Info does nothing in NoOp mode
func (n *NoOpLogger) Info(v ...any) {}

// Infof does nothing in NoOp mode
func (n *NoOpLogger) Infof(format string, v ...any) {}

// Debug does nothing in NoOp mode
func (n *NoOpLogger) Debug(v ...any) {}

// Debugf does nothing in NoOp mode
func (n *NoOpLogger) Debugf(format string, v ...any) {}
