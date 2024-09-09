package logger

import "github.com/sirupsen/logrus"

// loggerer is an interface for logging.
type loggerer interface {
	Info(...interface{})
	Warn(...interface{})
	Error(...interface{})
	Debug(...interface{})
	Infof(string, ...interface{})
	Warnf(string, ...interface{})
	Errorf(string, ...interface{})
	Debugf(string, ...interface{})
}

// noopLogger is a no-op logger.
type noopLogger struct{}

func (l *noopLogger) Info(...interface{})           {}
func (l *noopLogger) Warn(...interface{})           {}
func (l *noopLogger) Error(...interface{})          {}
func (l *noopLogger) Debug(...interface{})          {}
func (l *noopLogger) Infof(string, ...interface{})  {}
func (l *noopLogger) Warnf(string, ...interface{})  {}
func (l *noopLogger) Errorf(string, ...interface{}) {}
func (l *noopLogger) Debugf(string, ...interface{}) {}

// Logger is a logger.
type Logger struct {
	internal loggerer
}

// LogLevel represents the log level.
type LogLevel string

// Log levels.
const (
	Debug LogLevel = "debug"
	Info  LogLevel = "info"
	Warn  LogLevel = "warn"
	Error LogLevel = "error"
)

// _ is a compile-time check to ensure that logrus.Logger
// fully implements loggerer.
var _ loggerer = &logrus.Logger{}

// NOOPLogger is a logger that discards all log messages.
var NOOPLogger = &Logger{
	internal: &noopLogger{},
}

// New creates a new logger with the specified log level.
func New(level LogLevel) *Logger {
	lr := logrus.New()

	switch level {
	case Debug:
		lr.SetLevel(logrus.DebugLevel)
	case Info:
		lr.SetLevel(logrus.InfoLevel)
	case Warn:
		lr.SetLevel(logrus.WarnLevel)
	case Error:
		lr.SetLevel(logrus.ErrorLevel)
	default:
		lr.SetLevel(logrus.InfoLevel)
	}

	lr.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	return &Logger{
		internal: logrus.New(),
	}
}

// Info logs an info message.
func (l *Logger) Info(args ...interface{}) {
	l.internal.Info(args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(args ...interface{}) {
	l.internal.Warn(args...)
}

// Error logs an error message.
func (l *Logger) Error(args ...interface{}) {
	l.internal.Error(args...)
}

// Debug logs a debug message.
func (l *Logger) Debug(args ...interface{}) {
	l.internal.Debug(args...)
}

// Infof logs a formatted info message.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.internal.Infof(format, args...)
}

// Warnf logs a formatted warning message.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.internal.Warnf(format, args...)
}

// Errorf logs a formatted error message.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.internal.Errorf(format, args...)
}

// Debugf logs a formatted debug message.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.internal.Debugf(format, args...)
}
