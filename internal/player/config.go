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
	// EQEnabled indica si el ecualizador estaba activo al salir.
	EQEnabled bool `json:"eq_enabled"`
	// EQGains guarda la ganancia en dB de cada banda del ecualizador.
	EQGains []float64 `json:"eq_gains,omitempty"`
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

// loadConfig lee y deserializa el archivo de configuración completo.
func loadConfig() (Config, error) {
	var c Config
	p, err := configPath()
	if err != nil {
		return c, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	return c, nil
}

// saveConfig serializa la configuración completa conservando todos los campos.
func saveConfig(c Config) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

// LoadConfig lee y deserializa el archivo de configuración del usuario.
// Devuelve la ruta guardada del directorio de música o una cadena vacía si no existe.
func LoadConfig() (string, error) {
	c, err := loadConfig()
	if err != nil {
		return "", err
	}
	return c.MusicDir, nil
}

// LoadEQ devuelve el estado guardado del ecualizador (activado y ganancias por banda).
func LoadEQ() (bool, [eqBandCount]float64, error) {
	var gains [eqBandCount]float64
	c, err := loadConfig()
	if err != nil {
		return false, gains, err
	}
	if len(c.EQGains) == 0 {
		// Configuración sin ajustes de EQ (primera ejecución): valores por defecto.
		return true, gains, nil
	}
	copy(gains[:], c.EQGains)
	return c.EQEnabled, gains, nil
}

// SaveMusicDir persiste la ruta del directorio de música sin perder el resto de la configuración.
func SaveMusicDir(dir string) error {
	c, _ := loadConfig()
	c.MusicDir = dir
	return saveConfig(c)
}

// SaveEQ persiste el estado del ecualizador sin perder el resto de la configuración.
func SaveEQ(enabled bool, gains [eqBandCount]float64) error {
	c, _ := loadConfig()
	c.EQEnabled = enabled
	c.EQGains = gains[:]
	return saveConfig(c)
}
