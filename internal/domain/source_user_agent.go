package domain

import "strings"

type SourceUserAgent struct {
	ID          string
	UserAgent   string
	AIAgentKey  string
	AIAgentName string
	Source      string
}

func InferAIAgent(userAgent string) (key, name string) {
	normalized := strings.ToLower(strings.TrimSpace(userAgent))
	if normalized == "" {
		return "", ""
	}

	candidates := []struct {
		key      string
		name     string
		patterns []string
	}{
		{key: "claude", name: "Claude", patterns: []string{"claude-code", "claude"}},
		{key: "codex", name: "Codex", patterns: []string{"codex"}},
		{key: "copilot", name: "Copilot", patterns: []string{"copilot"}},
		{key: "cursor", name: "Cursor", patterns: []string{"cursor"}},
		{key: "gemini", name: "Gemini", patterns: []string{"gemini"}},
		{key: "qwen", name: "Qwen", patterns: []string{"qwen"}},
		{key: "kiro", name: "Kiro", patterns: []string{"kiro"}},
		{key: "goose", name: "Goose", patterns: []string{"goose"}},
		{key: "continue", name: "Continue", patterns: []string{"continue"}},
		{key: "windsurf", name: "Windsurf", patterns: []string{"windsurf"}},
		{key: "cline", name: "Cline", patterns: []string{"cline"}},
		{key: "roo", name: "Roo", patterns: []string{"roo"}},
		{key: "cody", name: "Cody", patterns: []string{"cody"}},
		{key: "opencode", name: "OpenCode", patterns: []string{"opencode"}},
		{key: "qoder", name: "Qoder", patterns: []string{"qoder"}},
	}

	for _, candidate := range candidates {
		for _, pattern := range candidate.patterns {
			if strings.Contains(normalized, pattern) {
				return candidate.key, candidate.name
			}
		}
	}

	return "", ""
}
