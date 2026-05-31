package socket

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-gost/x/config"
)

const runtimeConfigBackupSuffix = ".bak"

// configMutex serializes runtime config persistence and recovery.
var configMutex sync.Mutex

func saveConfig() error {
	configMutex.Lock()
	defer configMutex.Unlock()

	return persistSanitizedConfigLocked("gost.json", config.Global())
}

func SanitizeConfigForRuntime(cfg *config.Config) (*config.Config, bool) {
	if cfg == nil {
		return newEmptyRuntimeConfig(), false
	}

	out := *cfg
	var changed bool
	out.Services, changed = sanitizeServicesForSave(cfg.Services)
	return ensureRuntimeConfigSlices(&out), changed
}

func PersistSanitizedConfig(file string, cfg *config.Config) error {
	configMutex.Lock()
	defer configMutex.Unlock()
	return persistSanitizedConfigLocked(file, cfg)
}

func RecoverRuntimeConfig(file string) (bool, error) {
	configMutex.Lock()
	defer configMutex.Unlock()
	return recoverRuntimeConfigLocked(file)
}

func persistSanitizedConfigLocked(file string, cfg *config.Config) error {
	file = normalizeRuntimeConfigPath(file)

	sanitized, _ := SanitizeConfigForRuntime(cfg)
	payload, err := encodeRuntimeConfigJSON(sanitized)
	if err != nil {
		return err
	}

	mode := runtimeConfigFileMode(file)
	if err := backupCurrentRuntimeConfigLocked(file, mode); err != nil {
		return err
	}

	return atomicWriteFile(file, payload, mode)
}

func recoverRuntimeConfigLocked(file string) (bool, error) {
	file = normalizeRuntimeConfigPath(file)

	if data, err := os.ReadFile(file); err == nil {
		if _, parseErr := decodeRuntimeConfigJSON(data); parseErr == nil {
			return false, nil
		}
		if err := backupBrokenRuntimeConfigLocked(file, data); err != nil {
			return false, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	mode := runtimeConfigFileMode(file)
	backupFile := file + runtimeConfigBackupSuffix
	if backupData, err := os.ReadFile(backupFile); err == nil {
		if _, parseErr := decodeRuntimeConfigJSON(backupData); parseErr == nil {
			return true, atomicWriteFile(file, backupData, mode)
		}
	}

	return true, persistSanitizedConfigLocked(file, newEmptyRuntimeConfig())
}

func newEmptyRuntimeConfig() *config.Config {
	return &config.Config{
		Services:  []*config.ServiceConfig{},
		Chains:    []*config.ChainConfig{},
		Hops:      []*config.HopConfig{},
		Limiters:  []*config.LimiterConfig{},
		CLimiters: []*config.LimiterConfig{},
		RLimiters: []*config.LimiterConfig{},
	}
}

func ensureRuntimeConfigSlices(cfg *config.Config) *config.Config {
	if cfg == nil {
		return newEmptyRuntimeConfig()
	}
	if cfg.Services == nil {
		cfg.Services = []*config.ServiceConfig{}
	}
	if cfg.Chains == nil {
		cfg.Chains = []*config.ChainConfig{}
	}
	if cfg.Hops == nil {
		cfg.Hops = []*config.HopConfig{}
	}
	if cfg.Limiters == nil {
		cfg.Limiters = []*config.LimiterConfig{}
	}
	if cfg.CLimiters == nil {
		cfg.CLimiters = []*config.LimiterConfig{}
	}
	if cfg.RLimiters == nil {
		cfg.RLimiters = []*config.LimiterConfig{}
	}
	return cfg
}

func normalizeRuntimeConfigPath(file string) string {
	file = strings.TrimSpace(file)
	if file == "" {
		return "gost.json"
	}
	return file
}

func runtimeConfigFileMode(file string) os.FileMode {
	info, err := os.Stat(file)
	if err == nil {
		return info.Mode().Perm()
	}
	return 0o600
}

func encodeRuntimeConfigJSON(cfg *config.Config) ([]byte, error) {
	cfg = ensureRuntimeConfigSlices(cfg)
	var buf bytes.Buffer
	if err := cfg.Write(&buf, "json"); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeRuntimeConfigJSON(data []byte) (*config.Config, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("empty runtime config")
	}
	cfg := new(config.Config)
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return ensureRuntimeConfigSlices(cfg), nil
}

func backupCurrentRuntimeConfigLocked(file string, mode os.FileMode) error {
	data, err := os.ReadFile(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if _, err := decodeRuntimeConfigJSON(data); err != nil {
		return nil
	}
	return atomicWriteFile(file+runtimeConfigBackupSuffix, data, mode)
}

func backupBrokenRuntimeConfigLocked(file string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	brokenPath := file + ".broken-" + time.Now().UTC().Format("20060102T150405Z")
	return atomicWriteFile(brokenPath, data, runtimeConfigFileMode(file))
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) (err error) {
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Chmod(mode); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		_ = os.Remove(path)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return err
	}

	if dirHandle, openErr := os.Open(dir); openErr == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}

	return nil
}

func sanitizeServicesForSave(services []*config.ServiceConfig) ([]*config.ServiceConfig, bool) {
	if len(services) == 0 {
		return nil, false
	}

	type serviceEntry struct {
		index     int
		hasStatus bool
		service   *config.ServiceConfig
	}

	ordered := make([]serviceEntry, 0, len(services))
	byName := make(map[string]int, len(services))
	changed := false

	for _, svc := range services {
		if svc == nil {
			changed = true
			continue
		}

		name := strings.TrimSpace(svc.Name)
		if name == "" {
			changed = true
			continue
		}

		cloned := *svc
		if cloned.Name != name {
			changed = true
		}
		cloned.Name = name
		if cloned.Status != nil {
			changed = true
		}
		cloned.Status = nil

		entry := serviceEntry{
			hasStatus: svc.Status != nil,
			service:   &cloned,
		}

		if idx, exists := byName[name]; exists {
			changed = true
			if ordered[idx].hasStatus && !entry.hasStatus {
				ordered[idx].hasStatus = false
				ordered[idx].service = entry.service
			}
			continue
		}

		entry.index = len(ordered)
		ordered = append(ordered, entry)
		byName[name] = entry.index
	}

	result := make([]*config.ServiceConfig, 0, len(ordered))
	for _, entry := range ordered {
		if entry.service != nil {
			result = append(result, entry.service)
		}
	}
	return result, changed
}
