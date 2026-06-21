-- +goose Up
CREATE TABLE IF NOT EXISTS source_user_agents (
    id TEXT PRIMARY KEY,
    user_agent TEXT,
    ai_agent_key TEXT,
    ai_agent_name TEXT,
    source TEXT NOT NULL DEFAULT 'local',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_source_user_agents_user_agent
    ON source_user_agents (user_agent)
    WHERE user_agent IS NOT NULL AND user_agent <> '';

INSERT INTO source_user_agents (id, user_agent, ai_agent_key, ai_agent_name, source)
SELECT DISTINCT ON (source_user_agent_id)
    source_user_agent_id,
    NULLIF(plugin, ''),
    CASE
        WHEN lower(plugin) LIKE '%claude%' THEN 'claude'
        WHEN lower(plugin) LIKE '%codex%' THEN 'codex'
        WHEN lower(plugin) LIKE '%copilot%' THEN 'copilot'
        WHEN lower(plugin) LIKE '%cursor%' THEN 'cursor'
        WHEN lower(plugin) LIKE '%gemini%' THEN 'gemini'
        WHEN lower(plugin) LIKE '%qwen%' THEN 'qwen'
        WHEN lower(plugin) LIKE '%kiro%' THEN 'kiro'
        WHEN lower(plugin) LIKE '%goose%' THEN 'goose'
        WHEN lower(plugin) LIKE '%continue%' THEN 'continue'
        WHEN lower(plugin) LIKE '%windsurf%' THEN 'windsurf'
        WHEN lower(plugin) LIKE '%cline%' THEN 'cline'
        WHEN lower(plugin) LIKE '%roo%' THEN 'roo'
        WHEN lower(plugin) LIKE '%cody%' THEN 'cody'
        WHEN lower(plugin) LIKE '%opencode%' THEN 'opencode'
        WHEN lower(plugin) LIKE '%qoder%' THEN 'qoder'
        ELSE NULL
    END,
    CASE
        WHEN lower(plugin) LIKE '%claude%' THEN 'Claude'
        WHEN lower(plugin) LIKE '%codex%' THEN 'Codex'
        WHEN lower(plugin) LIKE '%copilot%' THEN 'Copilot'
        WHEN lower(plugin) LIKE '%cursor%' THEN 'Cursor'
        WHEN lower(plugin) LIKE '%gemini%' THEN 'Gemini'
        WHEN lower(plugin) LIKE '%qwen%' THEN 'Qwen'
        WHEN lower(plugin) LIKE '%kiro%' THEN 'Kiro'
        WHEN lower(plugin) LIKE '%goose%' THEN 'Goose'
        WHEN lower(plugin) LIKE '%continue%' THEN 'Continue'
        WHEN lower(plugin) LIKE '%windsurf%' THEN 'Windsurf'
        WHEN lower(plugin) LIKE '%cline%' THEN 'Cline'
        WHEN lower(plugin) LIKE '%roo%' THEN 'Roo'
        WHEN lower(plugin) LIKE '%cody%' THEN 'Cody'
        WHEN lower(plugin) LIKE '%opencode%' THEN 'OpenCode'
        WHEN lower(plugin) LIKE '%qoder%' THEN 'Qoder'
        ELSE NULL
    END,
    'wakatime-export'
FROM heartbeats
WHERE source_user_agent_id IS NOT NULL AND source_user_agent_id <> ''
ORDER BY source_user_agent_id, CASE WHEN plugin IS NULL OR plugin = '' THEN 1 ELSE 0 END, time DESC
ON CONFLICT (id) DO NOTHING;

INSERT INTO source_user_agents (id, user_agent, ai_agent_key, ai_agent_name, source)
SELECT DISTINCT ON (plugin)
    'local-' || md5(plugin),
    plugin,
    CASE
        WHEN lower(plugin) LIKE '%claude%' THEN 'claude'
        WHEN lower(plugin) LIKE '%codex%' THEN 'codex'
        WHEN lower(plugin) LIKE '%copilot%' THEN 'copilot'
        WHEN lower(plugin) LIKE '%cursor%' THEN 'cursor'
        WHEN lower(plugin) LIKE '%gemini%' THEN 'gemini'
        WHEN lower(plugin) LIKE '%qwen%' THEN 'qwen'
        WHEN lower(plugin) LIKE '%kiro%' THEN 'kiro'
        WHEN lower(plugin) LIKE '%goose%' THEN 'goose'
        WHEN lower(plugin) LIKE '%continue%' THEN 'continue'
        WHEN lower(plugin) LIKE '%windsurf%' THEN 'windsurf'
        WHEN lower(plugin) LIKE '%cline%' THEN 'cline'
        WHEN lower(plugin) LIKE '%roo%' THEN 'roo'
        WHEN lower(plugin) LIKE '%cody%' THEN 'cody'
        WHEN lower(plugin) LIKE '%opencode%' THEN 'opencode'
        WHEN lower(plugin) LIKE '%qoder%' THEN 'qoder'
        ELSE NULL
    END,
    CASE
        WHEN lower(plugin) LIKE '%claude%' THEN 'Claude'
        WHEN lower(plugin) LIKE '%codex%' THEN 'Codex'
        WHEN lower(plugin) LIKE '%copilot%' THEN 'Copilot'
        WHEN lower(plugin) LIKE '%cursor%' THEN 'Cursor'
        WHEN lower(plugin) LIKE '%gemini%' THEN 'Gemini'
        WHEN lower(plugin) LIKE '%qwen%' THEN 'Qwen'
        WHEN lower(plugin) LIKE '%kiro%' THEN 'Kiro'
        WHEN lower(plugin) LIKE '%goose%' THEN 'Goose'
        WHEN lower(plugin) LIKE '%continue%' THEN 'Continue'
        WHEN lower(plugin) LIKE '%windsurf%' THEN 'Windsurf'
        WHEN lower(plugin) LIKE '%cline%' THEN 'Cline'
        WHEN lower(plugin) LIKE '%roo%' THEN 'Roo'
        WHEN lower(plugin) LIKE '%cody%' THEN 'Cody'
        WHEN lower(plugin) LIKE '%opencode%' THEN 'OpenCode'
        WHEN lower(plugin) LIKE '%qoder%' THEN 'Qoder'
        ELSE NULL
    END,
    'local'
FROM heartbeats
WHERE (source_user_agent_id IS NULL OR source_user_agent_id = '')
  AND plugin IS NOT NULL
  AND plugin <> ''
ON CONFLICT (id) DO NOTHING;

UPDATE heartbeats AS heartbeats
SET source_user_agent_id = agents.id
FROM source_user_agents AS agents
WHERE (heartbeats.source_user_agent_id IS NULL OR heartbeats.source_user_agent_id = '')
  AND heartbeats.plugin = agents.user_agent
  AND heartbeats.plugin IS NOT NULL
  AND heartbeats.plugin <> '';

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_heartbeats_source_user_agent_id'
    ) THEN
        ALTER TABLE heartbeats
            ADD CONSTRAINT fk_heartbeats_source_user_agent_id
            FOREIGN KEY (source_user_agent_id)
            REFERENCES source_user_agents(id)
            ON UPDATE CASCADE
            ON DELETE SET NULL;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE heartbeats
    DROP CONSTRAINT IF EXISTS fk_heartbeats_source_user_agent_id;

DROP TABLE IF EXISTS source_user_agents;
