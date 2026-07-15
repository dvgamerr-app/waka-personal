-- +goose Up
UPDATE ai_model_pricing
SET input_cost_per_mtok = 1.75,
    output_cost_per_mtok = 14.0,
    notes = 'Coarse match; gpt-5.3-codex rate',
    updated_at = NOW()
WHERE model_key = 'Codex';

INSERT INTO ai_model_pricing
    (model_key, display_name, provider, input_cost_per_mtok, output_cost_per_mtok, notes)
VALUES
('gpt-5.3-codex', 'GPT-5.3 Codex', 'openai', 1.75, 14.0, NULL)
ON CONFLICT (model_key) DO UPDATE
SET display_name = EXCLUDED.display_name,
    provider = EXCLUDED.provider,
    input_cost_per_mtok = EXCLUDED.input_cost_per_mtok,
    output_cost_per_mtok = EXCLUDED.output_cost_per_mtok,
    updated_at = NOW();

-- +goose Down
DELETE FROM ai_model_pricing WHERE model_key = 'gpt-5.3-codex';

UPDATE ai_model_pricing
SET input_cost_per_mtok = 2.5,
    output_cost_per_mtok = 10.0,
    notes = 'Coarse match; gpt-4o rate',
    updated_at = NOW()
WHERE model_key = 'Codex';
