package config

import "testing"

func TestLoadAutoMigrate(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "unset defaults to off", value: "", want: false},
		{name: "enabled", value: "true", want: true},
		{name: "disabled", value: "0", want: false},
		{name: "invalid falls back to off", value: "maybe", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AUTO_MIGRATE", tt.value)
			if got := Load().AutoMigrate; got != tt.want {
				t.Fatalf("AutoMigrate = %v, want %v", got, tt.want)
			}
		})
	}
}
