package handler

import (
	"aws-prj/internal/middleware"
	"aws-prj/internal/service"
	"aws-prj/internal/util"
	sqlcq "aws-prj/pgsql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	service *service.Service
	log     *slog.Logger
	pool    *pgxpool.Pool
}

type CreateHandlerReq struct {
	Service *service.Service
	Log     *slog.Logger
	Pool    *pgxpool.Pool
}

func Init(r CreateHandlerReq) *Handler {
	return &Handler{
		service: r.Service,
		log:     r.Log,
		pool:    r.Pool,
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	log := middleware.LoggerFromContext(r.Context())
	log.Info("server is health")
	util.WriteJSON(w, http.StatusOK, "ok")
}

func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	log := middleware.LoggerFromContext(r.Context())
	if err := h.pool.Ping(r.Context()); err != nil {
		log.Error("db is not ready", "error", err)
		http.Error(w, "db is not ready", http.StatusInternalServerError)
		return
	}

	if err := h.service.Ping(r.Context()); err != nil {
		log.Error("redis is not ready", "error", err)
		http.Error(w, "redis is not ready", http.StatusInternalServerError)
		return
	}

	util.WriteJSON(w, http.StatusOK, "db and redis ready")
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	log := middleware.LoggerFromContext(r.Context())
	type createTaskReq struct {
		Context string `json:"context"`
	}

	var body createTaskReq

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Error("create task request body error", "error", err)
		http.Error(w, fmt.Sprintf("create task request body error: %v", err), http.StatusBadRequest)
		return
	}

	task, err := h.service.CreateTask(r.Context(), body.Context)

	if err != nil {
		log.Error("create task error", "error", err)
		http.Error(w, fmt.Sprintf("create task error: %v", err), http.StatusInternalServerError)
		return
	}

	util.WriteJSON(w, http.StatusOK, task)
}

func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	log := middleware.LoggerFromContext(r.Context())
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

	tasks, err := h.service.ListTasks(r.Context(), int32(offset), int32(limit))

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
}

func (h *Handler) GetTaskById(w http.ResponseWriter, r *http.Request) {
	log := middleware.LoggerFromContext(r.Context())
	id := chi.URLParam(r, "id")

	uuid, err := toPgUUID(id)

	if err != nil {
		log.Error("parse uuid failed", "error", err)
		http.Error(w, fmt.Sprintf("parse uuid failed: %v", err), http.StatusBadRequest)
		return
	}

	var redisTask sqlcq.Task

	val, err := h.service.RetrieveCacheTask(r.Context(), fmt.Sprintf("task:%s", id))

	if err != nil {
		task, err := h.service.GetTaskById(r.Context(), uuid)

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

		err = h.service.CacheTask(r.Context(), fmt.Sprintf("task:%s", id), string(b))

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
}

func (h *Handler) DeleteTaskById(w http.ResponseWriter, r *http.Request) {
	log := middleware.LoggerFromContext(r.Context())
	id := chi.URLParam(r, "id")

	uuid, err := toPgUUID(id)

	if err != nil {
		log.Error("parse uuid failed", "uuid", uuid)
		http.Error(w, fmt.Sprintf("parse uuid failed: %v", uuid), http.StatusInternalServerError)
		return
	}

	err = h.service.DeleteTaskById(r.Context(), uuid)

	if err != nil {
		log.Error("delete task by id", "id", id, "error", err)
		http.Error(w, fmt.Sprintf("delete task by id %s: %v", id, err), http.StatusInternalServerError)
		return
	}

	err = h.service.DecacheTask(r.Context(), fmt.Sprintf("task:%s", id))

	if err != nil {
		log.Error("remove cache task failed", "task_id", id, "error", err)
		http.Error(w, "remove cache task failed", http.StatusInternalServerError)
		return
	}

	util.WriteJSON(w, http.StatusOK, fmt.Sprintf("delete task by id %s success", id))
}
