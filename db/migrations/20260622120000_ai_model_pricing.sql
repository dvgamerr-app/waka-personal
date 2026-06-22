-- +goose Up
CREATE TABLE ai_model_pricing (
    model_key            TEXT        PRIMARY KEY,
    display_name         TEXT        NOT NULL,
    provider             TEXT        NOT NULL DEFAULT 'unknown',
    input_cost_per_mtok  NUMERIC(12,6) NOT NULL DEFAULT 0,
    output_cost_per_mtok NUMERIC(12,6) NOT NULL DEFAULT 0,
    notes                TEXT,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO ai_model_pricing
    (model_key, display_name, provider, input_cost_per_mtok, output_cost_per_mtok, notes)
VALUES
-- fallback when model is unknown
('default',            'Default (Unknown)',        'unknown',     3.0,    15.0,   'Fallback; Claude Sonnet 4.6 rate'),
-- Anthropic — coarse agent name from InferAIAgent
('Claude',             'Claude (Sonnet default)',  'anthropic',   3.0,    15.0,   'Coarse match; claude-sonnet-4-6 rate'),
-- Anthropic — specific model IDs
('claude-haiku-4-5',   'Claude Haiku 4.5',        'anthropic',   0.8,    4.0,    NULL),
('claude-sonnet-4-6',  'Claude Sonnet 4.6',       'anthropic',   3.0,    15.0,   NULL),
('claude-opus-4-8',    'Claude Opus 4.8',         'anthropic',   15.0,   75.0,   NULL),
('claude-fable-5',     'Claude Fable 5',          'anthropic',   3.0,    15.0,   'Estimate — update when official rate published'),
-- OpenAI
('Codex',              'Codex CLI',               'openai',      2.5,    10.0,   'Coarse match; gpt-4o rate'),
('gpt-4o',             'GPT-4o',                  'openai',      2.5,    10.0,   NULL),
('gpt-4o-mini',        'GPT-4o Mini',             'openai',      0.15,   0.6,    NULL),
('o3',                 'o3',                      'openai',      10.0,   40.0,   NULL),
('o3-mini',            'o3-mini',                 'openai',      1.1,    4.4,    NULL),
-- GitHub Copilot
('Copilot',            'GitHub Copilot',          'github',      0.0,    0.0,    'Subscription-based — no per-token billing'),
-- Cursor
('Cursor',             'Cursor',                  'cursor',      2.5,    10.0,   'Uses multiple models; gpt-4o rate as estimate'),
-- Google
('Gemini',             'Gemini',                  'google',      0.075,  0.30,   'Coarse match; Gemini 2.0 Flash rate'),
('gemini-2.0-flash',   'Gemini 2.0 Flash',        'google',      0.075,  0.30,   NULL),
('gemini-1.5-pro',     'Gemini 1.5 Pro',          'google',      1.25,   5.0,    NULL),
-- Others
('Kiro',               'Amazon Kiro',             'amazon',      3.0,    15.0,   'Estimate'),
('Goose',              'Block Goose',             'block',       3.0,    15.0,   'Estimate'),
('Continue',           'Continue',                'various',     3.0,    15.0,   'Uses various models'),
('Windsurf',           'Windsurf',                'codeium',     0.0,    0.0,    'Subscription-based'),
('Cline',              'Cline',                   'various',     3.0,    15.0,   'Uses various models'),
('Roo',                'Roo Code',                'various',     3.0,    15.0,   'Uses various models'),
('Cody',               'Sourcegraph Cody',        'sourcegraph', 0.0,    0.0,    'Subscription-based'),
('OpenCode',           'OpenCode',                'various',     3.0,    15.0,   'Estimate'),
('Qwen',               'Alibaba Qwen',            'alibaba',     0.0,    0.0,    'Pricing TBD'),
('Qoder',              'Qoder',                   'various',     0.0,    0.0,    'Pricing TBD');

-- +goose Down
DROP TABLE IF EXISTS ai_model_pricing;
