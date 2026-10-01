package store

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Port          int    `json:"port"`
	DataDir       string `json:"data_dir"`
	ModelPath     string `json:"model_path"`
	PasswordHash  string `json:"password_hash"`
	EnginePath    string `json:"engine_path,omitempty"`
	SessionSecret string `json:"session_secret,omitempty"`
}

func DefaultDataDir() string {
	if v := os.Getenv("NOTEBOT_DATA"); v != "" {
		return v
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "notebot")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".notebot-data"
	}
	return filepath.Join(home, ".notebot")
}

func DefaultConfig(dataDir string) Config {
	return Config{
		Port:      8765,
		DataDir:   dataDir,
		ModelPath: filepath.Join(dataDir, "models", "needle3.cact"),
	}
}

func LoadConfig(dataDir string) (Config, error) {
	cfg := DefaultConfig(dataDir)
	path := filepath.Join(dataDir, "config.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, err
	}
	if cfg.DataDir == "" {
		cfg.DataDir = dataDir
	}
	if cfg.Port == 0 {
		cfg.Port = 8765
	}
	if cfg.ModelPath == "" {
		cfg.ModelPath = filepath.Join(dataDir, "models", "needle3.cact")
	}
	return cfg, nil
}

func (c Config) Save() error {
	if err := os.MkdirAll(c.DataDir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.DataDir, "config.json"), raw, 0o600)
}

func (c *Config) EnsureSessionSecret() (bool, error) {
	if c.SessionSecret != "" {
		return false, nil
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return false, err
	}
	c.SessionSecret = hex.EncodeToString(b)
	return true, c.Save()
}

// SessionToken derives the dashboard session cookie value from the secret.
func SessionToken(secret string) string {
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("notebot-session-v1"))
	return hex.EncodeToString(mac.Sum(nil))
}

func ResolveWeightsPath(cfg Config) string {
	if cfg.ModelPath != "" {
		if _, err := os.Stat(cfg.ModelPath); err == nil {
			return cfg.ModelPath
		}
	}
	if env := os.Getenv("NEEDLE_MODEL"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}
	for _, p := range []string{
		filepath.Join("models", "needle3.cact"),
		filepath.Join(cfg.DataDir, "models", "needle3.cact"),
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if cfg.ModelPath != "" {
		return cfg.ModelPath
	}
	return filepath.Join(cfg.DataDir, "models", "needle3.cact")
}
