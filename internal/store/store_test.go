package store

import "testing"

func TestParseOptionalTimestamp(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "rfc3339",
			value: "2026-03-31T15:04:05Z",
			want:  "2026-03-31T15:04:05Z",
		},
		{
			name:  "legacy without timezone",
			value: "2016-08-06 06:21:41",
			want:  "2016-08-06T06:21:41Z",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseOptionalTimestamp(tc.value)
			if err != nil {
				t.Fatalf("parseOptionalTimestamp returned error: %v", err)
			}
			if got == nil {
				t.Fatal("expected parsed timestamp")
			}
			if got.Format("2006-01-02T15:04:05Z") != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got.Format("2006-01-02T15:04:05Z"))
			}
		})
	}
}
