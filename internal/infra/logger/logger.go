package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/thec1oud/billing/internal/config"
)

type LogWriterAdapter interface {
	GetWriter() (io.Writer, error)
	Close() error
}

type ConsoleAdapter struct{}

func (c *ConsoleAdapter) GetWriter() (io.Writer, error) {
	return os.Stdout, nil
}

func (c *ConsoleAdapter) Close() error {
	return nil // No-op for stdout
}

func InitGlobalLogger(cfg *config.Config) ([]io.Closer, error) {
	var writers []io.Writer
	var closers []io.Closer

	for _, target := range cfg.LogTargets {
		var adapter LogWriterAdapter

		switch target {
		case config.TargetConsole:
			adapter = &ConsoleAdapter{}
		case config.TargetFile:
			return nil, fmt.Errorf("log target 'FILE' is declared but not yet implemented. Extend LogWriterAdapter to use it")
		default:
			adapter = &ConsoleAdapter{}
		}

		writer, err := adapter.GetWriter()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize logger adapter for %s: %w", target, err)
		}

		writers = append(writers, writer)
		closers = append(closers, adapter)
	}

	// Dynamic Strategy: Merge all active writers into a single pipeline stream
	combinedWriter := io.MultiWriter(writers...)

	// Configure structure rules based on environment
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if cfg.AppEnv == "development" {
		opts.Level = slog.LevelDebug
	}

	var handler slog.Handler
	if cfg.AppEnv == "production" {
		handler = slog.NewJSONHandler(combinedWriter, opts)
	} else {
		handler = slog.NewTextHandler(combinedWriter, opts)
	}

	slog.SetDefault(slog.New(handler))

	// Return active resources back to main to handle defer tracking cleanly
	return closers, nil
}
