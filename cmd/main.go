package main

import (
	"aws-prj/internal/config"
	"aws-prj/internal/db"
	"aws-prj/internal/logger"
	redisclient "aws-prj/internal/redis_client"
	"aws-prj/internal/util"
	sqlcq "aws-prj/pgsql"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func toPgUUID(s string) (pgtype.UUID, error) {
	id, err := uuid.Parse(s)

	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{
		Bytes: id,
		Valid: true,
	}, nil
}

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

	app := chi.NewRouter()

	app.Use(middleware.RequestID)
	app.Use(middleware.Recoverer)
	app.Use(middleware.Timeout(5 * time.Second))

	app.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		util.WriteJSON(w, http.StatusOK, "ok")
	})

	app.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			log.Error("db is not ready", "error", err)
			http.Error(w, "db is not ready", http.StatusInternalServerError)
			return
		}

		if err := redisClient.Ping(r.Context()); err != nil {
			log.Error("redis is not ready", "error", err)
			http.Error(w, "redis is not ready", http.StatusInternalServerError)
			return
		}

		util.WriteJSON(w, http.StatusOK, "db and redis ready")
	})

	app.Get("/api/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		uuid, err := toPgUUID(id)

		if err != nil {
			log.Error("parse uuid failed", "error", err)
			http.Error(w, fmt.Sprintf("parse uuid failed: %v", err), http.StatusBadRequest)
			return
		}

		var redisTask sqlcq.Task

		val, err := redisClient.Get(r.Context(), fmt.Sprintf("task:%s", id)).Result()

		if err != nil {
			task, err := queries.GetTaskById(r.Context(), uuid)

			if err != nil {
				log.Error("get task by id failed by db", "error", err)
				http.Error(w, fmt.Sprintf("(db) get task by id failed: %v", err), http.StatusInternalServerError)
				return
			}

			b, err := json.Marshal(task)
			if err != nil {
				log.Error("json marshal failed", "error", err)
				http.Error(w, fmt.Sprintf("json marshal failed: %v", err), http.StatusInternalServerError)
				return
			}

			err = redisClient.Set(r.Context(), fmt.Sprintf("task:%s", id), string(b), time.Minute*5).Err()

			if err != nil {
				log.Error("redis store task failed", "task_id", uuid, "error", err)
				http.Error(w, fmt.Sprintf("redis store task failed: %v", err), http.StatusInternalServerError)
				return
			}

			log.Info("fetch task by pgdb", "task_id", id)

			util.WriteJSON(w, http.StatusOK, task)
			return
		}

		err = json.Unmarshal([]byte(val), &redisTask)

		if err != nil {
			log.Error("json unmarshal failed", "error", err)
			http.Error(w, fmt.Sprintf("json unmarshal failed: %v", err), http.StatusInternalServerError)
			return
		}

		log.Info("fetch task by redis", "task_id", id)

		util.WriteJSON(w, http.StatusOK, redisTask)
	})

	app.Delete("/api/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		uuid, err := toPgUUID(id)

		if err != nil {
			log.Error("parse uuid failed", "uuid", uuid)
			http.Error(w, fmt.Sprintf("parse uuid failed: %v", uuid), http.StatusInternalServerError)
			return
		}

		err = queries.DeleteTaskById(r.Context(), uuid)

		if err != nil {
			log.Error("delete task by id", "id", id, "error", err)
			http.Error(w, fmt.Sprintf("delete task by id %s: %v", id, err), http.StatusInternalServerError)
			return
		}

		util.WriteJSON(w, http.StatusOK, fmt.Sprintf("delete task by id %s success", id))
	})

	app.Get("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		type listTasksResp struct {
			List   []sqlcq.Task `json:"list"`
			Offset int          `json:"offset"`
			Limit  int          `json:"limit"`
			Page   int          `json:"page"`
		}
		pageStr := r.URL.Query().Get("page")
		limitStr := r.URL.Query().Get("limit")

		page, err := strconv.Atoi(pageStr)

		if err != nil {
			log.Error("page str parse failed", "error", err)
			http.Error(w, "page str parse failed", http.StatusBadRequest)
			return
		}

		limit, err := strconv.Atoi(limitStr)

		if err != nil {
			log.Error("limit str parse failed", "error", err)
			http.Error(w, "limit str parse failed", http.StatusBadRequest)
			return
		}

		offset := (page - 1) * limit

		tasks, err := queries.ListTasks(r.Context(), sqlcq.ListTasksParams{
			Offset: int32(offset),
			Limit:  int32(limit),
		})

		if err != nil {
			log.Info("list tasks response empty", "error", err)
			util.WriteJSON(w, http.StatusOK, listTasksResp{
				List:   []sqlcq.Task{},
				Offset: offset,
				Limit:  limit,
				Page:   page,
			})
		} else {
			util.WriteJSON(w, http.StatusOK, listTasksResp{
				List:   tasks,
				Offset: offset,
				Limit:  limit,
				Page:   page,
			})
		}
	})

	app.Post("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		type createTaskReq struct {
			Context string `json:"context"`
		}

		var body createTaskReq

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			log.Error("create task request body error", "error", err)
			http.Error(w, fmt.Sprintf("create task request body error: %v", err), http.StatusBadRequest)
			return
		}

		task, err := queries.CreateTask(r.Context(), body.Context)

		if err != nil {
			log.Error("create task error", "error", err)
			http.Error(w, fmt.Sprintf("create task error: %v", err), http.StatusInternalServerError)
			return
		}

		util.WriteJSON(w, http.StatusOK, task)
	})

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
