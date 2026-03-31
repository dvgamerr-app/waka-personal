-- +goose Up
CREATE TABLE IF NOT EXISTS heartbeats (
    id TEXT PRIMARY KEY,
    source_heartbeat_id TEXT,
    dedupe_hash TEXT NOT NULL UNIQUE,
    time TIMESTAMPTZ NOT NULL,
    source_created_at TIMESTAMPTZ,
    entity TEXT NOT NULL,
    type TEXT NOT NULL,
    category TEXT NOT NULL,
    project TEXT,
    branch TEXT,
    language TEXT,
    project_root_count INTEGER,
    project_folder TEXT,
    lineno INTEGER,
    cursorpos INTEGER,
    lines INTEGER,
    is_write BOOLEAN NOT NULL DEFAULT FALSE,
    is_unsaved_entity BOOLEAN NOT NULL DEFAULT FALSE,
    ai_line_changes INTEGER,
    human_line_changes INTEGER,
    machine_name TEXT,
    source_machine_name_id TEXT,
    plugin TEXT,
    source_user_agent_id TEXT,
    dependencies JSONB NOT NULL DEFAULT '[]'::jsonb,
    import_batch_id TEXT,
    origin_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_heartbeats_time ON heartbeats (time);
CREATE INDEX IF NOT EXISTS idx_heartbeats_entity_time ON heartbeats (entity, time);
CREATE INDEX IF NOT EXISTS idx_heartbeats_project_time ON heartbeats (project, time);
CREATE INDEX IF NOT EXISTS idx_heartbeats_import_batch_id ON heartbeats (import_batch_id);

-- +goose Down
DROP TABLE IF EXISTS heartbeats;

