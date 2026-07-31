package logger

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"

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
		switch target {
		case config.TargetConsole:
			adapter := &ConsoleAdapter{}
			w, err := adapter.GetWriter()
			if err != nil {
				return nil, fmt.Errorf("failed to initialize console adapter: %w", err)
			}
			writers = append(writers, w)
			closers = append(closers, adapter)

		case config.TargetFile:
			return nil, fmt.Errorf("log target 'FILE' is declared but not yet implemented")

		default:
			// Fallback to console for unknown/unhandled target types
			adapter := &ConsoleAdapter{}
			w, _ := adapter.GetWriter()
			writers = append(writers, w)
			closers = append(closers, adapter)
		}
	}

	// GUARD: If LogTargets was empty or failed to parse, fallback to os.Stdout directly
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	combinedWriter := io.MultiWriter(writers...)

	// Configure log level based on environment
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if strings.EqualFold(cfg.AppEnv, "development") {
		opts.Level = slog.LevelDebug
		opts.AddSource = true // Helps trace log origin during dev
	}

	var handler slog.Handler
	if strings.EqualFold(cfg.AppEnv, "production") {
		handler = slog.NewJSONHandler(combinedWriter, opts)
	} else {
		handler = slog.NewTextHandler(combinedWriter, opts)
	}

	logger := slog.New(handler)

	// 1. Set global slog default
	slog.SetDefault(logger)

	// 2. Redirect standard log package output (used by third-party packages / driver internals) to slog
	log.SetFlags(0)
	log.SetOutput(slog.NewLogLogger(handler, slog.LevelInfo).Writer())

	slog.Info("logger pipeline initialized successfully",
		"env", cfg.AppEnv,
		"targets_count", len(writers),
	)

	return closers, nil
}
