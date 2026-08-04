package player

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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
