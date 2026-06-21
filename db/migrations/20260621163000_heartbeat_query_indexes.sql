-- +goose Up
CREATE INDEX IF NOT EXISTS idx_heartbeats_time_entity ON heartbeats (time, entity);
CREATE INDEX IF NOT EXISTS idx_heartbeats_entity_project_time ON heartbeats (entity, project, time);

-- +goose Down
DROP INDEX IF EXISTS idx_heartbeats_entity_project_time;
DROP INDEX IF EXISTS idx_heartbeats_time_entity;
