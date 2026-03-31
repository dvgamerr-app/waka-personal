-- +goose Up
CREATE TABLE IF NOT EXISTS import_snapshot (
    id TEXT PRIMARY KEY,
    source_path TEXT NOT NULL,
    source_format TEXT NOT NULL,
    source_sha256 TEXT NOT NULL,
    status TEXT NOT NULL,
    range_start TIMESTAMPTZ,
    range_end TIMESTAMPTZ,
    imported_rows BIGINT NOT NULL DEFAULT 0,
    skipped_rows BIGINT NOT NULL DEFAULT 0,
    error_text TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_import_snapshot_sha256 ON import_snapshot (source_sha256);

CREATE TABLE IF NOT EXISTS import_profile (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    external_user_id TEXT,
    username TEXT,
    display_name TEXT,
    full_name TEXT,
    email TEXT,
    photo TEXT,
    profile_url TEXT,
    timezone TEXT,
    plan TEXT,
    timeout_minutes INTEGER,
    writes_only BOOLEAN,
    city JSONB NOT NULL DEFAULT 'null'::jsonb,
    last_branch TEXT,
    last_language TEXT,
    last_plugin TEXT,
    last_project TEXT,
    profile_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);


-- +goose Down
DROP TABLE IF EXISTS import_snapshot;
DROP TABLE IF EXISTS import_profile;
