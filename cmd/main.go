package main

import (
	"aws-prj/internal/config"
	"aws-prj/internal/db"
	"aws-prj/internal/handler"
	"aws-prj/internal/logger"
	redisclient "aws-prj/internal/redis_client"
	"aws-prj/internal/repository"
	"aws-prj/internal/service"
	sqlcq "aws-prj/pgsql"
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	middleware "github.com/go-chi/chi/v5/middleware"

	customMiddleware "aws-prj/internal/middleware"
)

func main() {
	cfg, err := config.Load()

	log := logger.Init()

	if err != nil {
		log.Error("load config", "error", err)
		os.Exit(1)
	}

	pool, err := db.InitDB(context.Background(), cfg.DatabaseURL, log)

	if err != nil {
		log.Error("database connection failed", "error", err)
		os.Exit(1)
	}

	redisClient, err := redisclient.Init(context.Background(), cfg.RedisAddr, log)

	if err != nil {
		log.Error("redis connection failed", "error", err)
		os.Exit(1)
	}

	queries := sqlcq.New(pool)

	repo := repository.Init(queries)

	ser := service.Init(repo, redisClient)

	hdl := handler.Init(handler.CreateHandlerReq{
		Service: ser,
		Log:     log,
		Pool:    pool,
	})

	app := chi.NewRouter()

	app.Use(middleware.RequestID)
	app.Use(customMiddleware.RequestLogger(log))
	app.Use(middleware.Recoverer)
	app.Use(middleware.Timeout(5 * time.Second))

	app.Get("/healthz", hdl.Health)

	app.Get("/readyz", hdl.Readyz)

	app.Get("/api/tasks/{id}", hdl.GetTaskById)

	app.Delete("/api/tasks/{id}", hdl.DeleteTaskById)

	app.Get("/api/tasks", hdl.ListTasks)

	app.Post("/api/tasks", hdl.CreateTask)

	srv := &http.Server{
		Addr:    cfg.Port,
		Handler: app,
	}

	serverErr := make(chan error, 1)

	go func() {
		log.Info("server starting", "addr", srv.Addr)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)

	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		log.Error("server failed", "error", err)
		os.Exit(1)
	case <-stop:
		log.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}

	pool.Close()

	if err := redisClient.Close(); err != nil {
		log.Error("redis close failed", "error", err)
	}

	log.Info("server stopped")
}
