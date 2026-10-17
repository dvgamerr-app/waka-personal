-- +goose Up
INSERT INTO ai_model_pricing
    (model_key, display_name, provider, input_cost_per_mtok, output_cost_per_mtok, notes)
VALUES
('claude-sonnet-5',  'Claude Sonnet 5',  'anthropic', 2.0,  10.0, NULL),
('claude-haiku-5-5', 'Claude Haiku 5.5', 'anthropic', 0.10, 0.50, 'Rate for prompts up to 100K tokens'),
('claude-opus-4-7',  'Claude Opus 4.7',  'anthropic', 5.0,  25.0, NULL),
('claude-opus-4-6',  'Claude Opus 4.6',  'anthropic', 5.0,  25.0, NULL)
ON CONFLICT (model_key) DO UPDATE
SET display_name = EXCLUDED.display_name,
    provider = EXCLUDED.provider,
    input_cost_per_mtok = EXCLUDED.input_cost_per_mtok,
    output_cost_per_mtok = EXCLUDED.output_cost_per_mtok,
    notes = EXCLUDED.notes,
    updated_at = NOW();

-- +goose Down
DELETE FROM ai_model_pricing
WHERE model_key IN ('claude-haiku-5-5', 'claude-opus-4-7', 'claude-opus-4-6');

UPDATE ai_model_pricing
SET input_cost_per_mtok = 3.0,
    output_cost_per_mtok = 15.0,
    updated_at = NOW()
WHERE model_key = 'claude-sonnet-5';
