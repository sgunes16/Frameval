package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/mustafaselman/frameval/engine/internal/api"
	builtinharness "github.com/mustafaselman/frameval/engine/internal/builtin/harness"
	"github.com/mustafaselman/frameval/engine/internal/executor"
	"github.com/mustafaselman/frameval/engine/internal/experiment"
	"github.com/mustafaselman/frameval/engine/internal/logging"
	"github.com/mustafaselman/frameval/engine/internal/sandbox"
	"github.com/mustafaselman/frameval/engine/internal/storage"
)

func main() {
	if err := run(); err != nil {
		// run() has already invoked every deferred close — we are safe to
		// terminate the process here without skipping cleanup. The error
		// is already logged via the structured logger; this line is the
		// final readable signal when something is hand-tailing the binary.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run is the engine's actual lifecycle. It returns nil on a clean signal-
// driven shutdown (SIGINT / SIGTERM with the server gracefully closing)
// and an error otherwise. By keeping main() a thin wrapper around run()
// we guarantee every defer (store.Close, queue.Close, graderClient.Close,
// signal stop) fires before os.Exit kills the process — the previous
// log.Fatalf path skipped all defers, leaving sandbox containers and
// gRPC connections orphaned on startup failure.
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := logging.New(logging.Config{
		Level:  parseLogLevel(getenv("FRAMEVAL_LOG_LEVEL", "info")),
		Format: getenv("FRAMEVAL_LOG_FORMAT", "json"),
	})

	dbPath := getenv("FRAMEVAL_DB_PATH", "./frameval.db")
	graderAddr := getenv("FRAMEVAL_GRADER_ADDR", "localhost:50051")
	port := getenv("FRAMEVAL_PORT", "8080")
	sandboxImage := getenv("FRAMEVAL_SANDBOX_IMAGE", "frameval-sandbox:local")
	tasksRoot := getenv("FRAMEVAL_TASKS_ROOT", "../tasks")
	maxConcurrent := getenvInt("FRAMEVAL_MAX_CONCURRENT", 1)

	store, err := storage.Open(ctx, dbPath)
	if err != nil {
		logger.Error("open store failed", "err", err)
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()
	if err := store.SeedBuiltinTasks(ctx, tasksRoot); err != nil {
		logger.Warn("seed tasks", "err", err)
	}
	if err := store.SeedModelConfigs(ctx); err != nil {
		logger.Warn("seed model configs", "err", err)
	}
	if count, err := store.ReconcileCompletedExperiments(ctx); err != nil {
		logger.Warn("reconcile completed experiments", "err", err)
	} else if count > 0 {
		logger.Info("reconciled completed experiments", "count", count)
	}

	hub := api.NewHub()
	go hub.Run(ctx)
	manager := sandbox.NewManager(ctx, sandboxImage)
	// Fan out from the static seed (in SeedModelConfigs) to whatever
	// opencode's bundled cloud provider currently exposes. Best-effort —
	// SeedOpenCodeModels swallows transient errors so a missing daemon or
	// slow image pull never blocks engine startup.
	if err := store.SeedOpenCodeModels(ctx, manager); err != nil {
		logger.Warn("seed opencode models failed", "err", err)
	}
	queue := experiment.NewQueue(ctx, maxConcurrent)
	defer queue.Close()
	registry := executor.NewRegistry(manager)
	harnessRegistry := builtinharness.NewRegistry()
	graderClient := experiment.NewGraderClient(graderAddr, logger)
	defer func() { _ = graderClient.Close() }()
	orchestrator := experiment.NewOrchestrator(store, queue, manager, registry, harnessRegistry, graderClient, hub)
	service := api.NewService(store, orchestrator, harnessRegistry, registry, hub)
	server := &http.Server{Addr: ":" + port, Handler: api.NewRouter(service, logger)}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	logger.Info("frameval engine listening", "port", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("listen and serve", "err", err)
		return fmt.Errorf("listen and serve: %w", err)
	}
	return nil
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
