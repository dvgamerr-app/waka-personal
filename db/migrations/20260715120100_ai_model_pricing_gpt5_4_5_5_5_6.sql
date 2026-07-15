-- +goose Up
INSERT INTO ai_model_pricing
    (model_key, display_name, provider, input_cost_per_mtok, output_cost_per_mtok, notes)
VALUES
('gpt-5.4',         'GPT-5.4',              'openai', 2.5,  15.0,  NULL),
('gpt-5.4-mini',    'GPT-5.4 Mini',         'openai', 0.75, 4.5,   NULL),
('gpt-5.4-nano',    'GPT-5.4 Nano',         'openai', 0.20, 1.25,  NULL),
('gpt-5.4-pro',     'GPT-5.4 Pro',          'openai', 30.0, 180.0, NULL),
('gpt-5.5',         'GPT-5.5',              'openai', 5.0,  30.0,  NULL),
('gpt-5.5-pro',     'GPT-5.5 Pro',          'openai', 30.0, 180.0, NULL),
('gpt-5.6-sol',     'GPT-5.6 Sol',          'openai', 5.0,  30.0,  'Frontier tier of the GPT-5.6 family'),
('gpt-5.6-terra',   'GPT-5.6 Terra',        'openai', 2.5,  15.0,  'Balanced tier of the GPT-5.6 family'),
('gpt-5.6-luna',    'GPT-5.6 Luna',         'openai', 1.0,  6.0,   'Cost-sensitive tier of the GPT-5.6 family')
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
    'gpt-5.4', 'gpt-5.4-mini', 'gpt-5.4-nano', 'gpt-5.4-pro',
    'gpt-5.5', 'gpt-5.5-pro',
    'gpt-5.6-sol', 'gpt-5.6-terra', 'gpt-5.6-luna'
);
