package setup

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestValidators(t *testing.T) {
	tests := []struct {
		name    string
		fn      func(string) error
		input   string
		wantErr bool
	}{
		{"required ok", required, "x", false},
		{"required blank", required, "  ", true},
		{"url https", validURL, "https://a.com/rss", false},
		{"url no scheme", validURL, "a.com/rss", true},
		{"int ok", positiveInt, "5", false},
		{"int zero", positiveInt, "0", true},
		{"int text", positiveInt, "five", true},
		{"duration ok", validDuration, "24h", false},
		{"duration zero", validDuration, "0", false},
		{"duration bad", validDuration, "soon", true},
		{"duration combined", validDuration, "1h30m", false},
		{"duration empty", validDuration, "", true},
		{"duration missing unit", validDuration, "24", true},
		{"duration days", validDuration, "3d", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSplitList(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a, b, ,c ", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		if got := splitList(tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitList(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestLoadStartingConfig(t *testing.T) {
	// write a file into a temp dir and return its path
	write := func(dir, name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("existing config wins", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := write(dir, "config.yaml", "sources:\n  - name: Mine\n    url: https://x.com/rss\n")
		exPath := write(dir, "example.yaml", "sources:\n  - name: Example\n    url: https://x.com/rss\n")
		cfg, existing, err := loadStartingConfig(cfgPath, exPath)
		if err != nil || !existing || cfg.Sources[0].Name != "Mine" {
			t.Fatalf("got %+v, existing=%v, err=%v", cfg, existing, err)
		}
	})

	t.Run("falls back to example", func(t *testing.T) {
		dir := t.TempDir()
		exPath := write(dir, "example.yaml", "sources:\n  - name: Example\n    url: https://x.com/rss\n")
		cfg, existing, err := loadStartingConfig(filepath.Join(dir, "missing.yaml"), exPath)
		if err != nil || existing || cfg.Sources[0].Name != "Example" {
			t.Fatalf("got %+v, existing=%v, err=%v", cfg, existing, err)
		}
		if len(cfg.Notifications) != 1 || cfg.Notifications[0] != "email" {
			t.Errorf("expected default email notification, got %v", cfg.Notifications)
		}
	})

	t.Run("neither file exists", func(t *testing.T) {
		dir := t.TempDir()
		cfg, existing, err := loadStartingConfig(filepath.Join(dir, "a.yaml"), filepath.Join(dir, "b.yaml"))
		if err != nil || existing || len(cfg.Sources) != 0 {
			t.Fatalf("got %+v, existing=%v, err=%v", cfg, existing, err)
		}
	})

}
