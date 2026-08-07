package player

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dhowden/tag"
)

// formatDuration convierte una duración en un formato de cadena "MM:SS".
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	return fmt.Sprintf("%02d:%02d", d/time.Minute, (d%time.Minute)/time.Second)
}

// clampDuration limita una duración entre un valor mínimo y máximo.
func clampDuration(val, minVal, maxVal time.Duration) time.Duration {
	return max(minVal, min(maxVal, val))
}

// truncate recorta una cadena a una longitud máxima, añadiendo "..." si es necesario.
func truncate(str string, maxLen int) string {
	runes := []rune(str)
	if len(runes) > maxLen {
		return string(runes[:maxLen-3]) + "..."
	}
	return str
}

// ExtractMetadata intenta extraer metadatos de un archivo de audio.
// Si falla o faltan campos, usa valores predeterminados basados en el nombre del archivo.
func ExtractMetadata(path string) Track {
	ext := strings.ToLower(filepath.Ext(path))
	filename := strings.TrimSuffix(filepath.Base(path), ext)

	track := Track{
		Title:  filename,
		Artist: "Desconocido",
		Album:  "Desconocido",
		Path:   path,
	}

	f, err := os.Open(path)
	if err != nil {
		return track
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return track
	}

	if t := m.Title(); t != "" {
		track.Title = t
	}
	if a := m.Artist(); a != "" {
		track.Artist = a
	}
	if al := m.Album(); al != "" {
		track.Album = al
	}

	return track
}

// renderVolumeBar renderiza una barra de volumen visual basada en el nivel y estado de silencio.
func renderVolumeBar(level, minVal, maxVal float64, muted bool) string {
	if muted {
		return lipgloss.NewStyle().Foreground(red).Bold(true).Render(iconMute + " MUTE")
	}
	if maxVal == minVal {
		return fmt.Sprintf("%s ░░░░░░░░░░ 0%%", iconVolume)
	}

	pct := max(0.0, min(1.0, (level-minVal)/(maxVal-minVal)))
	filled := int(pct * 10)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)

	color := green
	if pct > 0.85 {
		color = orange
	} else if pct > 0.5 {
		color = yellow
	}
	return lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf("%s %s %3.0f%%", iconVolume, bar, pct*100))
}
