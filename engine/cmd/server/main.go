package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mustafaselman/frameval/engine/internal/api"
	"github.com/mustafaselman/frameval/engine/internal/executor"
	"github.com/mustafaselman/frameval/engine/internal/experiment"
	"github.com/mustafaselman/frameval/engine/internal/sandbox"
	"github.com/mustafaselman/frameval/engine/internal/storage"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dbPath := getenv("FRAMEVAL_DB_PATH", "./frameval.db")
	graderAddr := getenv("FRAMEVAL_GRADER_ADDR", "localhost:50051")
	port := getenv("FRAMEVAL_PORT", "8080")
	sandboxImage := getenv("FRAMEVAL_SANDBOX_IMAGE", "frameval-sandbox:local")
	tasksRoot := getenv("FRAMEVAL_TASKS_ROOT", "../tasks")

	store, err := storage.Open(ctx, dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.SeedBuiltinTasks(ctx, tasksRoot); err != nil {
		log.Printf("seed tasks: %v", err)
	}
	if err := store.SeedModelConfigs(ctx); err != nil {
		log.Printf("seed model configs: %v", err)
	}

	hub := api.NewHub()
	go hub.Run(ctx)
	manager := sandbox.NewManager(ctx, sandboxImage)
	queue := experiment.NewQueue(ctx, 3)
	defer queue.Close()
	registry := executor.NewRegistry(manager)
	graderClient := experiment.NewGraderClient(graderAddr)
	orchestrator := experiment.NewOrchestrator(store, queue, manager, registry, graderClient, hub)
	service := api.NewService(store, orchestrator, hub)
	server := &http.Server{Addr: ":" + port, Handler: api.NewRouter(service)}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("frameval engine listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen and serve: %v", err)
	}
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
