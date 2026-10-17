CREATE TYPE task_status AS ENUM ('todo', 'in_progress', 'done');

CREATE TABLE tasks (
  id          BIGSERIAL    PRIMARY KEY,
  title       VARCHAR(255) NOT NULL,
  description TEXT         NOT NULL DEFAULT '',
  status      task_status  NOT NULL DEFAULT 'todo',
  assignee    VARCHAR(100),
  created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_tasks_status   ON tasks (status);
CREATE INDEX idx_tasks_assignee ON tasks (assignee);