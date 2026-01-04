package main

import (
	"log/slog"

	"github.com/go-logr/logr"
)

// slogLogr adapts slog.Logger to logr.Logger interface
type slogLogr struct {
	logger *slog.Logger
	name   string
}

// NewSlogLogr creates a new logr.Logger that uses slog
func NewSlogLogr(logger *slog.Logger) logr.Logger {
	return logr.New(&slogLogr{logger: logger})
}

func (l *slogLogr) Init(info logr.RuntimeInfo) {}

func (l *slogLogr) Enabled(level int) bool {
	// controller-runtime uses V(0) for info, V(1) for debug
	// Map to slog levels
	return true
}

func (l *slogLogr) Info(level int, msg string, keysAndValues ...any) {
	logger := l.logger
	if l.name != "" {
		logger = logger.With("logger", l.name)
	}
	if level > 0 {
		logger.Debug(msg, keysAndValues...)
	} else {
		logger.Info(msg, keysAndValues...)
	}
}

func (l *slogLogr) Error(err error, msg string, keysAndValues ...any) {
	logger := l.logger
	if l.name != "" {
		logger = logger.With("logger", l.name)
	}
	args := append([]any{"error", err}, keysAndValues...)
	logger.Error(msg, args...)
}

func (l *slogLogr) WithValues(keysAndValues ...any) logr.LogSink {
	return &slogLogr{
		logger: l.logger.With(keysAndValues...),
		name:   l.name,
	}
}

func (l *slogLogr) WithName(name string) logr.LogSink {
	newName := name
	if l.name != "" {
		newName = l.name + "." + name
	}
	return &slogLogr{
		logger: l.logger,
		name:   newName,
	}
}
