package socket

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-gost/x/config"
)

func TestSanitizeConfigForSave_RemovesServiceStatusAndDedupes(t *testing.T) {
	status := &config.ServiceStatus{State: "ready"}
	cfg := &config.Config{
		Services: []*config.ServiceConfig{
			{
				Name:   "svc-a",
				Addr:   "[::]:10001",
				Status: status,
			},
			{
				Name: "svc-a",
				Addr: "[::]:10001",
			},
			{
				Name:   "svc-b",
				Addr:   "[::]:10002",
				Status: status,
			},
		},
	}

	sanitized, changed := SanitizeConfigForRuntime(cfg)
	if !changed {
		t.Fatal("expected config sanitize to report changes")
	}
	if sanitized == cfg {
		t.Fatal("expected sanitizeConfigForSave to return a cloned config")
	}
	if len(sanitized.Services) != 2 {
		t.Fatalf("expected 2 deduped services, got %d", len(sanitized.Services))
	}
	if sanitized.Services[0] == cfg.Services[0] {
		t.Fatal("expected service config to be cloned")
	}
	if sanitized.Services[0].Status != nil || sanitized.Services[1].Status != nil {
		t.Fatal("expected runtime status to be stripped before save")
	}
	if sanitized.Services[0].Name != "svc-a" || sanitized.Services[1].Name != "svc-b" {
		t.Fatalf("unexpected service order after sanitize: %q, %q", sanitized.Services[0].Name, sanitized.Services[1].Name)
	}
	if cfg.Services[0].Status == nil || cfg.Services[2].Status == nil {
		t.Fatal("expected original config to remain unchanged")
	}
}

func TestSanitizeServicesForSave_SkipsNilAndBlankNames(t *testing.T) {
	services, changed := sanitizeServicesForSave([]*config.ServiceConfig{
		nil,
		{Name: "   "},
		{Name: "svc-a", Addr: "[::]:10001"},
	})
	if !changed {
		t.Fatal("expected service sanitize to report changes")
	}
	if len(services) != 1 {
		t.Fatalf("expected only one valid service, got %d", len(services))
	}
	if services[0].Name != "svc-a" {
		t.Fatalf("expected service name svc-a, got %q", services[0].Name)
	}
}

func TestPersistSanitizedConfigCreatesBackupAndWritesAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gost.json")

	original := &config.Config{
		Services: []*config.ServiceConfig{{Name: "svc-old", Addr: "[::]:10001"}},
	}
	if err := PersistSanitizedConfig(path, original); err != nil {
		t.Fatalf("persist original config: %v", err)
	}

	updated := &config.Config{
		Services: []*config.ServiceConfig{{Name: "svc-new", Addr: "[::]:10002"}},
	}
	if err := PersistSanitizedConfig(path, updated); err != nil {
		t.Fatalf("persist updated config: %v", err)
	}

	currentData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read current config: %v", err)
	}
	currentCfg, err := decodeRuntimeConfigJSON(currentData)
	if err != nil {
		t.Fatalf("decode current config: %v", err)
	}
	if len(currentCfg.Services) != 1 || currentCfg.Services[0].Name != "svc-new" {
		t.Fatalf("unexpected current services: %+v", currentCfg.Services)
	}

	backupData, err := os.ReadFile(path + runtimeConfigBackupSuffix)
	if err != nil {
		t.Fatalf("read backup config: %v", err)
	}
	backupCfg, err := decodeRuntimeConfigJSON(backupData)
	if err != nil {
		t.Fatalf("decode backup config: %v", err)
	}
	if len(backupCfg.Services) != 1 || backupCfg.Services[0].Name != "svc-old" {
		t.Fatalf("unexpected backup services: %+v", backupCfg.Services)
	}
}

func TestRecoverRuntimeConfigRestoresBackupWhenPrimaryInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gost.json")

	validBackup := &config.Config{
		Services: []*config.ServiceConfig{{Name: "svc-backup", Addr: "[::]:10003"}},
	}
	backupData, err := encodeRuntimeConfigJSON(validBackup)
	if err != nil {
		t.Fatalf("encode backup config: %v", err)
	}
	if err := os.WriteFile(path+runtimeConfigBackupSuffix, backupData, 0o600); err != nil {
		t.Fatalf("write backup config: %v", err)
	}
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("write invalid primary config: %v", err)
	}

	recovered, err := RecoverRuntimeConfig(path)
	if err != nil {
		t.Fatalf("recover runtime config: %v", err)
	}
	if !recovered {
		t.Fatal("expected recovery to report changes")
	}

	currentData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read recovered config: %v", err)
	}
	currentCfg, err := decodeRuntimeConfigJSON(currentData)
	if err != nil {
		t.Fatalf("decode recovered config: %v", err)
	}
	if len(currentCfg.Services) != 1 || currentCfg.Services[0].Name != "svc-backup" {
		t.Fatalf("unexpected recovered services: %+v", currentCfg.Services)
	}
}

func TestRecoverRuntimeConfigCreatesEmptyConfigWhenBackupMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gost.json")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	recovered, err := RecoverRuntimeConfig(path)
	if err != nil {
		t.Fatalf("recover runtime config: %v", err)
	}
	if !recovered {
		t.Fatal("expected recovery to report changes")
	}

	currentData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read recovered config: %v", err)
	}
	currentCfg, err := decodeRuntimeConfigJSON(currentData)
	if err != nil {
		t.Fatalf("decode recovered config: %v", err)
	}
	if len(currentCfg.Services) != 0 {
		t.Fatalf("expected empty runtime services after fallback, got %+v", currentCfg.Services)
	}
}
