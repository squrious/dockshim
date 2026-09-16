package pathmap

import "testing"

func TestToContainer(t *testing.T) {
	m := Map{
		{Host: "/proj", Container: "/app"},
		{Host: "/proj/assets", Container: "/assets/build"},
		{Host: "/proj/lib", Container: "/app/lib"},
	}
	tests := []struct {
		host, want string
		ok         bool
	}{
		{"/proj", "/app", true},
		{"/proj/src/x", "/app/src/x", true},
		{"/proj/assets", "/assets/build", true},
		{"/proj/assets/css", "/assets/build/css", true},
		{"/proj/assetsX", "/app/assetsX", true},
		{"/proj/..foo", "/app/..foo", true},
		{"/project", "", false},
		{"/other", "", false},
		{"/", "", false},
	}
	for _, tt := range tests {
		got, ok := m.ToContainer(tt.host)
		if got != tt.want || ok != tt.ok {
			t.Errorf("ToContainer(%q) = %q, %v; want %q, %v", tt.host, got, ok, tt.want, tt.ok)
		}
	}
	if _, ok := (Map{}).ToContainer("/proj"); ok {
		t.Error("empty map must not match")
	}
}
