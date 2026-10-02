// Package player proporciona la lógica central del reproductor de audio, incluyendo
// la interfaz de terminal (TUI), el motor de audio y la gestión de configuración.
package player

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config representa la estructura de configuración persistente del reproductor,
// guardada en disco en formato JSON.
type Config struct {
	// MusicDir es la ruta absoluta al último directorio de música escaneado o seleccionado.
	MusicDir string `json:"music_dir"`
}

// configPath devuelve la ruta absoluta al archivo config.json dentro del directorio
// estándar de configuración del usuario (ej. ~/.config/goplayer/config.json en Linux).
// Si el directorio no existe, lo crea con permisos 0755.
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

// LoadConfig lee y deserializa el archivo de configuración del usuario.
// Devuelve la ruta guardada del directorio de música o una cadena vacía si no existe.
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

// SaveMusicDir persiste la ruta del directorio de música en el archivo de configuración JSON.
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
