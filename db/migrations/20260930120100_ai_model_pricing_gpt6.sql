-- +goose Up
INSERT INTO ai_model_pricing
    (model_key, display_name, provider, input_cost_per_mtok, output_cost_per_mtok, notes)
VALUES
('gpt-6-astra',  'GPT-6 Astra',  'openai', 10.0, 50.0, 'Short-context rate (<=272K input tokens)'),
('gpt-6-sol',    'GPT-6 Sol',    'openai', 2.0,  10.0, 'Short-context rate (<=272K input tokens)'),
('gpt-6-luna',   'GPT-6 Luna',   'openai', 0.10, 0.50, 'Short-context rate (<=272K input tokens)'),
('gpt-6.1-sol',  'GPT-6.1 Sol',  'openai', 2.0,  10.0, 'Short-context rate (<=272K input tokens)')
ON CONFLICT (model_key) DO UPDATE
SET display_name = EXCLUDED.display_name,
    provider = EXCLUDED.provider,
    input_cost_per_mtok = EXCLUDED.input_cost_per_mtok,
    output_cost_per_mtok = EXCLUDED.output_cost_per_mtok,
    notes = EXCLUDED.notes,
    updated_at = NOW();

-- +goose Down
DELETE FROM ai_model_pricing
WHERE model_key IN ('gpt-6-astra', 'gpt-6-sol', 'gpt-6-luna', 'gpt-6.1-sol');
