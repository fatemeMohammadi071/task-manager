-- name: CreateTask :one
INSERT INTO tasks (title, description, status, assignee)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetTask :one
SELECT * FROM tasks
WHERE id = $1
LIMIT 1;

-- name: ListTasks :many
SELECT * FROM tasks
WHERE (sqlc.narg('status')::task_status IS NULL OR status = sqlc.narg('status')::task_status)
  AND (sqlc.narg('assignee')::varchar IS NULL OR assignee = sqlc.narg('assignee')::varchar)
ORDER BY id
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountTasks :one
SELECT COUNT(*) FROM tasks
WHERE (sqlc.narg('status')::task_status IS NULL OR status = sqlc.narg('status')::task_status)
  AND (sqlc.narg('assignee')::varchar IS NULL OR assignee = sqlc.narg('assignee')::varchar);

-- name: UpdateTask :one
UPDATE tasks SET
  title       = COALESCE(sqlc.narg('title'), title),
  description = COALESCE(sqlc.narg('description'), description),
  status      = COALESCE(sqlc.narg('status')::task_status, status),
  assignee    = COALESCE(sqlc.narg('assignee'), assignee),
  updated_at  = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteTask :execrows
DELETE FROM tasks
WHERE id = $1;