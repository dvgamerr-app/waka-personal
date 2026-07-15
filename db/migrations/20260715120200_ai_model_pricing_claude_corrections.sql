-- +goose Up
INSERT INTO ai_model_pricing
    (model_key, display_name, provider, input_cost_per_mtok, output_cost_per_mtok, notes)
VALUES
('claude-sonnet-5', 'Claude Sonnet 5', 'anthropic', 3.0, 15.0, 'Intro rate $2.00/$10.00 per MTok through 2026-08-31')
ON CONFLICT (model_key) DO UPDATE
SET display_name = EXCLUDED.display_name,
    provider = EXCLUDED.provider,
    input_cost_per_mtok = EXCLUDED.input_cost_per_mtok,
    output_cost_per_mtok = EXCLUDED.output_cost_per_mtok,
    notes = EXCLUDED.notes,
    updated_at = NOW();

UPDATE ai_model_pricing
SET input_cost_per_mtok = 5.0,
    output_cost_per_mtok = 25.0,
    notes = NULL,
    updated_at = NOW()
WHERE model_key = 'claude-opus-4-8';

UPDATE ai_model_pricing
SET input_cost_per_mtok = 1.0,
    output_cost_per_mtok = 5.0,
    notes = NULL,
    updated_at = NOW()
WHERE model_key = 'claude-haiku-4-5';

UPDATE ai_model_pricing
SET input_cost_per_mtok = 10.0,
    output_cost_per_mtok = 50.0,
    notes = NULL,
    updated_at = NOW()
WHERE model_key = 'claude-fable-5';

-- +goose Down
DELETE FROM ai_model_pricing WHERE model_key = 'claude-sonnet-5';

UPDATE ai_model_pricing
SET input_cost_per_mtok = 15.0,
    output_cost_per_mtok = 75.0,
    notes = NULL,
    updated_at = NOW()
WHERE model_key = 'claude-opus-4-8';

UPDATE ai_model_pricing
SET input_cost_per_mtok = 0.8,
    output_cost_per_mtok = 4.0,
    notes = NULL,
    updated_at = NOW()
WHERE model_key = 'claude-haiku-4-5';

UPDATE ai_model_pricing
SET input_cost_per_mtok = 3.0,
    output_cost_per_mtok = 15.0,
    notes = 'Estimate — update when official rate published',
    updated_at = NOW()
WHERE model_key = 'claude-fable-5';
