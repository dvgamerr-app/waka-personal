package domain

import "testing"

func TestInferAIAgent(t *testing.T) {
	tests := []struct {
		userAgent string
		wantKey   string
		wantName  string
	}{
		{
			userAgent: "wakatime/2.19.0 (Windows-amd64) go1.26.0 plugin/0.0.1 claude-code/2.1.45",
			wantKey:   "claude",
			wantName:  "Claude",
		},
		{
			userAgent: "wakatime/2.19.0 codex/0.12.0 vscode-wakatime/24.13.0",
			wantKey:   "codex",
			wantName:  "Codex",
		},
		{
			userAgent: "vscode-wakatime/24.13.0",
			wantKey:   "",
			wantName:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.userAgent, func(t *testing.T) {
			gotKey, gotName := InferAIAgent(tc.userAgent)
			if gotKey != tc.wantKey || gotName != tc.wantName {
				t.Fatalf("expected %q/%q, got %q/%q", tc.wantKey, tc.wantName, gotKey, gotName)
			}
		})
	}
}
