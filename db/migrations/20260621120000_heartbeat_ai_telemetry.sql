-- +goose Up
ALTER TABLE heartbeats
    ADD COLUMN IF NOT EXISTS ai_session TEXT,
    ADD COLUMN IF NOT EXISTS ai_subscription_plan TEXT,
    ADD COLUMN IF NOT EXISTS ai_input_tokens BIGINT,
    ADD COLUMN IF NOT EXISTS ai_output_tokens BIGINT,
    ADD COLUMN IF NOT EXISTS ai_prompt_length INTEGER;

-- +goose Down
ALTER TABLE heartbeats
    DROP COLUMN IF EXISTS ai_session,
    DROP COLUMN IF EXISTS ai_subscription_plan,
    DROP COLUMN IF EXISTS ai_input_tokens,
    DROP COLUMN IF EXISTS ai_output_tokens,
    DROP COLUMN IF EXISTS ai_prompt_length;
