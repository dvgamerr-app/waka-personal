-- +goose Up
DROP INDEX IF EXISTS idx_import_snapshot_sha256;
CREATE INDEX IF NOT EXISTS idx_import_snapshot_source_sha256 ON import_snapshot (source_sha256);


-- +goose Down
DROP INDEX IF EXISTS idx_import_snapshot_source_sha256;
CREATE UNIQUE INDEX IF NOT EXISTS idx_import_snapshot_sha256 ON import_snapshot (source_sha256);
