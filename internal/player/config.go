package player

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	MusicDir string `json:"music_dir"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	cfgDir := filepath.Join(dir, "goplayer")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "config.json"), nil
}

func LoadConfig() (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return "", err
	}
	return c.MusicDir, nil
}

func SaveMusicDir(dir string) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	c := Config{MusicDir: dir}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}
