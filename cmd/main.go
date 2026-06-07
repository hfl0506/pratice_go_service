package main

import (
	"aws-prj/internal/config"
	"aws-prj/internal/db"
	redisclient "aws-prj/internal/redis_client"
	"aws-prj/internal/util"
	sqlcq "aws-prj/pgsql"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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

	if err != nil {
		log.Fatalf("load config error: %v", err)
	}

	pool, err := db.InitDB(context.Background(), cfg.DatabaseURL)

	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	redisClient, err := redisclient.Init(context.Background(), cfg.RedisAddr)

	if err != nil {
		log.Fatalf("redis connection failed: %v", err)
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
			log.Println("db is not ready")
			http.Error(w, "db is not ready", http.StatusInternalServerError)
			return
		}

		if err := redisClient.Ping(r.Context()); err != nil {
			log.Println("redis is not ready")
			http.Error(w, "redis is not ready", http.StatusInternalServerError)
			return
		}

		util.WriteJSON(w, http.StatusOK, "db and redis ready")
	})

	app.Get("/api/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		uuid, err := toPgUUID(id)

		if err != nil {
			http.Error(w, fmt.Sprintf("parse uuid failed: %v", err), http.StatusInternalServerError)
			return
		}

		var redisTask sqlcq.Task

		val, err := redisClient.Get(r.Context(), fmt.Sprintf("task:%s", id)).Result()

		if err != nil {
			task, err := queries.GetTaskById(r.Context(), uuid)

			if err != nil {
				http.Error(w, fmt.Sprintf("(db) get task by id failed: %v", err), http.StatusInternalServerError)
				return
			}

			b, err := json.Marshal(task)
			if err != nil {
				log.Printf("json marshal failed: %v", err)
			}

			err = redisClient.Set(r.Context(), fmt.Sprintf("task:%s", id), string(b), time.Minute*5).Err()

			if err != nil {
				log.Printf("redis store task %s failed: %v", uuid, err)
			}

			log.Printf("fetch task %s by pgdb", id)

			util.WriteJSON(w, http.StatusOK, task)
			return
		}

		err = json.Unmarshal([]byte(val), &redisTask)

		if err != nil {
			http.Error(w, fmt.Sprintf("json unmarshal failed: %v", err), http.StatusInternalServerError)
			return
		}

		log.Printf("fetch task %s by redis", id)

		util.WriteJSON(w, http.StatusOK, redisTask)
	})

	app.Delete("/api/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		uuid, err := toPgUUID(id)

		if err != nil {
			http.Error(w, fmt.Sprintf("parse uuid failed: %v", uuid), http.StatusInternalServerError)
			return
		}

		err = queries.DeleteTaskById(r.Context(), uuid)

		if err != nil {
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
			http.Error(w, "page str parse failed", http.StatusBadRequest)
			return
		}

		limit, err := strconv.Atoi(limitStr)

		if err != nil {
			http.Error(w, "limit str parse failed", http.StatusBadRequest)
			return
		}

		offset := (page - 1) * limit

		tasks, err := queries.ListTasks(r.Context(), sqlcq.ListTasksParams{
			Offset: int32(offset),
			Limit:  int32(limit),
		})

		if err != nil {
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
			http.Error(w, fmt.Sprintf("create task request body error: %v", err), http.StatusBadRequest)
			return
		}

		task, err := queries.CreateTask(r.Context(), body.Context)

		if err != nil {
			http.Error(w, fmt.Sprintf("create task error: %v", err), http.StatusInternalServerError)
			return
		}

		util.WriteJSON(w, http.StatusOK, task)
	})

	log.Println("server listen to port 8080")
	log.Fatalf("server error: %v", http.ListenAndServe(":8080", app))
}
