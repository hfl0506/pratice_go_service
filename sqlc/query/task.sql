-- name: CreateTask :one
INSERT INTO tasks (
  context
) VALUES (
  $1
) RETURNING *;

-- name: ListTasks :many
SELECT id, context, done, created_at, updated_at 
FROM tasks 
ORDER BY created_at DESC 
LIMIT $1 
OFFSET $2;

-- name: GetTaskById :one
SELECT id, context, done, created_at, updated_at 
FROM tasks 
WHERE id = $1 
LIMIT 1;

-- name: UpdateTaskById :one
UPDATE tasks 
SET context = $2,
    updated_at = now() 
WHERE id = $1 
RETURNING *;

-- name: DeleteTaskById :exec
DELETE FROM tasks 
WHERE id = $1;
