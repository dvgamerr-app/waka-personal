-- +goose Up
WITH exported AS (
    SELECT DISTINCT ON (source_user_agent_id)
        source_user_agent_id AS id,
        NULLIF(plugin, '') AS user_agent,
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
        END AS ai_agent_key,
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
        END AS ai_agent_name
    FROM heartbeats
    WHERE source_user_agent_id IS NOT NULL
      AND source_user_agent_id <> ''
    ORDER BY source_user_agent_id, CASE WHEN plugin IS NULL OR plugin = '' THEN 1 ELSE 0 END, time DESC
),
replacements AS (
    SELECT DISTINCT ON (user_agent) *
    FROM exported
    WHERE user_agent IS NOT NULL
    ORDER BY user_agent, id
)
UPDATE source_user_agents AS agents
SET id = replacements.id,
    ai_agent_key = COALESCE(replacements.ai_agent_key, agents.ai_agent_key),
    ai_agent_name = COALESCE(replacements.ai_agent_name, agents.ai_agent_name),
    source = 'wakatime-export',
    updated_at = NOW()
FROM replacements
WHERE agents.user_agent = replacements.user_agent
  AND agents.id <> replacements.id
  AND NOT EXISTS (
    SELECT 1
    FROM source_user_agents AS existing
    WHERE existing.id = replacements.id
  );

WITH exported AS (
    SELECT DISTINCT ON (source_user_agent_id)
        source_user_agent_id AS id,
        NULLIF(plugin, '') AS user_agent
    FROM heartbeats
    WHERE source_user_agent_id IS NOT NULL
      AND source_user_agent_id <> ''
    ORDER BY source_user_agent_id, CASE WHEN plugin IS NULL OR plugin = '' THEN 1 ELSE 0 END, time DESC
),
replacements AS (
    SELECT DISTINCT ON (user_agent) *
    FROM exported
    WHERE user_agent IS NOT NULL
    ORDER BY user_agent, id
),
moved_heartbeats AS (
    UPDATE heartbeats AS heartbeats
    SET source_user_agent_id = replacements.id
    FROM source_user_agents AS agents
    JOIN replacements ON replacements.user_agent = agents.user_agent
    WHERE agents.id <> replacements.id
      AND heartbeats.source_user_agent_id = agents.id
      AND EXISTS (
        SELECT 1
        FROM source_user_agents AS existing
        WHERE existing.id = replacements.id
      )
    RETURNING agents.id
)
DELETE FROM source_user_agents AS agents
USING replacements
WHERE agents.user_agent = replacements.user_agent
  AND agents.id <> replacements.id
  AND EXISTS (
    SELECT 1
    FROM source_user_agents AS existing
    WHERE existing.id = replacements.id
  );

INSERT INTO source_user_agents (id, user_agent, ai_agent_key, ai_agent_name, source, updated_at)
SELECT id, user_agent, ai_agent_key, ai_agent_name, 'wakatime-export', NOW()
FROM (
    SELECT DISTINCT ON (source_user_agent_id)
        source_user_agent_id AS id,
        NULLIF(plugin, '') AS user_agent,
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
        END AS ai_agent_key,
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
        END AS ai_agent_name
    FROM heartbeats
    WHERE source_user_agent_id IS NOT NULL
      AND source_user_agent_id <> ''
    ORDER BY source_user_agent_id, CASE WHEN plugin IS NULL OR plugin = '' THEN 1 ELSE 0 END, time DESC
) AS exported
ON CONFLICT (id) DO UPDATE
SET user_agent = COALESCE(EXCLUDED.user_agent, source_user_agents.user_agent),
    ai_agent_key = COALESCE(EXCLUDED.ai_agent_key, source_user_agents.ai_agent_key),
    ai_agent_name = COALESCE(EXCLUDED.ai_agent_name, source_user_agents.ai_agent_name),
    source = EXCLUDED.source,
    updated_at = NOW();

-- +goose Down
UPDATE source_user_agents
SET user_agent = NULL,
    ai_agent_key = NULL,
    ai_agent_name = NULL,
    updated_at = NOW()
WHERE source = 'wakatime-export';
