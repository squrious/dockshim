package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreDir(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, dir string)
		want    string // .gitignore content after the call, "" when it must not be a regular file
		wantErr bool
	}{
		{name: "created with its directory", want: "*\n"},
		{
			name: "existing file kept",
			setup: func(t *testing.T, dir string) {
				must(t, os.MkdirAll(dir, 0o755))
				must(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("custom\n"), 0o644))
			},
			want: "custom\n",
		},
		{
			name:  "existing directory kept",
			setup: func(t *testing.T, dir string) { must(t, os.MkdirAll(filepath.Join(dir, ".gitignore"), 0o755)) },
		},
		{
			name:    "dir is a file",
			setup:   func(t *testing.T, dir string) { must(t, os.WriteFile(dir, nil, 0o644)) },
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), ".dockshim")
			if tt.setup != nil {
				tt.setup(t, dir)
			}
			err := ignoreDir(dir)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if tt.wantErr {
				return
			}
			content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
			if tt.want == "" {
				if err == nil {
					t.Fatal(".gitignore should not be a regular file")
				}
				return
			}
			if err != nil || string(content) != tt.want {
				t.Fatalf("content = %q, err = %v", content, err)
			}
		})
	}
}
