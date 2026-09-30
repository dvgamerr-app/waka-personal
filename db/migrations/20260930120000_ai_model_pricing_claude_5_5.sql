-- +goose Up
INSERT INTO ai_model_pricing
    (model_key, display_name, provider, input_cost_per_mtok, output_cost_per_mtok, notes)
VALUES
('claude-fable-5-1',  'Claude Fable 5.1',  'anthropic', 10.0, 50.0, NULL),
('claude-opus-5-5',   'Claude Opus 5.5',   'anthropic', 4.0,  20.0, NULL),
('claude-opus-5',     'Claude Opus 5',     'anthropic', 5.0,  25.0, NULL),
('claude-sonnet-5-5', 'Claude Sonnet 5.5', 'anthropic', 2.0,  10.0, NULL)
ON CONFLICT (model_key) DO UPDATE
SET display_name = EXCLUDED.display_name,
    provider = EXCLUDED.provider,
    input_cost_per_mtok = EXCLUDED.input_cost_per_mtok,
    output_cost_per_mtok = EXCLUDED.output_cost_per_mtok,
    notes = EXCLUDED.notes,
    updated_at = NOW();

-- +goose Down
DELETE FROM ai_model_pricing
WHERE model_key IN (
    'claude-fable-5-1', 'claude-opus-5-5', 'claude-opus-5', 'claude-sonnet-5-5'
);
