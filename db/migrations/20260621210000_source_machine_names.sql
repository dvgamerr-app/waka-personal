-- +goose Up
CREATE TABLE IF NOT EXISTS source_machine_names (
    id TEXT PRIMARY KEY,
    machine_name TEXT,
    source TEXT NOT NULL DEFAULT 'local',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_source_machine_names_machine_name
    ON source_machine_names (machine_name)
    WHERE machine_name IS NOT NULL AND machine_name <> '';

-- Adopt machine ids that already arrived from the wakatime export.
INSERT INTO source_machine_names (id, machine_name, source)
SELECT DISTINCT ON (source_machine_name_id)
    source_machine_name_id,
    NULLIF(machine_name, ''),
    'wakatime-export'
FROM heartbeats
WHERE source_machine_name_id IS NOT NULL AND source_machine_name_id <> ''
ORDER BY source_machine_name_id, CASE WHEN machine_name IS NULL OR machine_name = '' THEN 1 ELSE 0 END, time DESC
ON CONFLICT (id) DO NOTHING;

-- Generate ids for machines that only have a name.
INSERT INTO source_machine_names (id, machine_name, source)
SELECT DISTINCT ON (machine_name)
    'local-' || md5(machine_name),
    machine_name,
    'local'
FROM heartbeats
WHERE (source_machine_name_id IS NULL OR source_machine_name_id = '')
  AND machine_name IS NOT NULL
  AND machine_name <> ''
ON CONFLICT (id) DO NOTHING;

UPDATE heartbeats AS heartbeats
SET source_machine_name_id = machines.id
FROM source_machine_names AS machines
WHERE (heartbeats.source_machine_name_id IS NULL OR heartbeats.source_machine_name_id = '')
  AND heartbeats.machine_name = machines.machine_name
  AND heartbeats.machine_name IS NOT NULL
  AND heartbeats.machine_name <> '';

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_heartbeats_source_machine_name_id'
    ) THEN
        ALTER TABLE heartbeats
            ADD CONSTRAINT fk_heartbeats_source_machine_name_id
            FOREIGN KEY (source_machine_name_id)
            REFERENCES source_machine_names(id)
            ON UPDATE CASCADE
            ON DELETE SET NULL;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE heartbeats
    DROP CONSTRAINT IF EXISTS fk_heartbeats_source_machine_name_id;

DROP TABLE IF EXISTS source_machine_names;
