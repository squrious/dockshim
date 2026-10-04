package style

import "testing"

func TestStyle(t *testing.T) {
	tests := []struct {
		name string
		fn   func(Style, string) string
		want string
	}{
		{"Error", Style.Error, "\x1b[1;31mx\x1b[0m"},
		{"Warning", Style.Warning, "\x1b[1;33mx\x1b[0m"},
		{"Success", Style.Success, "\x1b[32mx\x1b[0m"},
		{"Title", Style.Title, "\x1b[1mx\x1b[0m"},
		{"Key", Style.Key, "\x1b[36mx\x1b[0m"},
		{"Faint", Style.Faint, "\x1b[2mx\x1b[0m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(Style{Enabled: true}, "x"); got != tt.want {
				t.Errorf("enabled = %q, want %q", got, tt.want)
			}
			if got := tt.fn(Style{}, "x"); got != "x" {
				t.Errorf("disabled = %q, want %q", got, "x")
			}
			if got := tt.fn(Style{Enabled: true}, ""); got != "" {
				t.Errorf("empty = %q, want no codes", got)
			}
		})
	}
}
