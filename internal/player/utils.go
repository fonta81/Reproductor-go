// Package player reúne funciones auxiliares y utilidades para el formateo de tiempo,
// acotamiento de valores, recorte de cadenas, cálculo de volumen, extracción de metadatos
// mediante etiquetas de audio y validación de directorios en el sistema de archivos.
package player

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dhowden/tag"
)

// formatDuration convierte un intervalo de tiempo time.Duration en una cadena formateada
// en minutos y segundos ("MM:SS"), redondeando previamente al segundo más cercano.
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	return fmt.Sprintf("%02d:%02d", d/time.Minute, (d%time.Minute)/time.Second)
}

// clampDuration restringe un intervalo de duración time.Duration dentro de los límites
// inferior (minVal) y superior (maxVal) especificados.
func clampDuration(val, minVal, maxVal time.Duration) time.Duration {
	return max(minVal, min(maxVal, val))
}

// truncate recorta una cadena de texto a una longitud máxima de caracteres (runas),
// añadiendo puntos suspensivos ("...") al final si excede dicho límite.
// Trabaja sobre runas para prevenir corrupciones al manipular caracteres multibyte UTF-8.
func truncate(str string, maxLen int) string {
	runes := []rune(str)
	if len(runes) > maxLen {
		return string(runes[:maxLen-3]) + "..."
	}
	return str
}

// SupportedAudioExtensions define el conjunto centralizado de extensiones de archivo de audio
// soportadas y decodificables por el motor de reproducción.
var SupportedAudioExtensions = map[string]bool{
	".mp3":  true, // MPEG-1/2 Audio Layer III
	".wav":  true, // Waveform Audio File Format (PCM sin compresión)
	".flac": true, // Free Lossless Audio Codec (compresión sin pérdida)
	".ogg":  true, // Ogg Vorbis (compresión abierta con pérdida)
}

// IsSupportedAudio determina si la ruta de archivo especificada corresponde a un formato
// de audio compatible evaluando su extensión en minúsculas contra SupportedAudioExtensions.
func IsSupportedAudio(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return SupportedAudioExtensions[ext]
}

// VolumePercentage calcula y normaliza el nivel de volumen en decibelios (dB)
// a un porcentaje de escala lineal en el rango [0.0, 1.0].
// Si maxVal es igual a minVal, retorna 0.0 para prevenir divisiones por cero.
func VolumePercentage(level, minVal, maxVal float64) float64 {
	if maxVal == minVal {
		return 0.0
	}
	return max(0.0, min(1.0, (level-minVal)/(maxVal-minVal)))
}

// VolumePercentageInt convierte el nivel de volumen en decibelios a un valor entero porcentual entre 0 y 100,
// adecuado para su presentación en barras de progreso e interfaces de usuario.
func VolumePercentageInt(level, minVal, maxVal float64) int {
	return int(VolumePercentage(level, minVal, maxVal) * 100)
}

// ExtractMetadata inspecciona el archivo de audio en la ruta indicada y lee sus etiquetas
// de metadatos (ID3v1, ID3v2, Vorbis Comments, etc.) utilizando la biblioteca tag.
// Si el archivo no contiene etiquetas válidas o la lectura falla, asigna valores de respaldo
// utilizando el nombre base del archivo como título y "Desconocido" como artista y álbum.
func ExtractMetadata(path string) Track {
	ext := strings.ToLower(filepath.Ext(path))
	filename := strings.TrimSuffix(filepath.Base(path), ext)

	// Estructura base con valores de respaldo predeterminados
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

	// Intentar decodificar etiquetas de metadatos desde el flujo del archivo
	m, err := tag.ReadFrom(f)
	if err != nil {
		return track
	}

	// Sobrescribir los valores por defecto si los campos contienen información válida
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

// renderVolumeBar construye y formatea una barra visual de volumen en caracteres de bloques ("█" y "░").
// Asigna colores dinámicos según el umbral alcanzado (verde para niveles estándar, amarillo para medios-altos,
// naranja para altos y rojo con etiqueta especial si el audio está silenciado).
func renderVolumeBar(level, minVal, maxVal float64, muted bool) string {
	// Representación visual en estado de silencio activo
	if muted {
		return lipgloss.NewStyle().Foreground(red).Bold(true).Render(iconMute + " MUTE")
	}

	pct := VolumePercentage(level, minVal, maxVal)
	if maxVal == minVal {
		return fmt.Sprintf("%s ░░░░░░░░░░ 0%%", iconVolume)
	}

	filled := int(pct * 10)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)

	// Selección de color según el umbral de sonoridad
	color := green
	if pct > 0.85 {
		color = orange
	} else if pct > 0.5 {
		color = yellow
	}
	return lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf("%s %s %3.0f%%", iconVolume, bar, pct*100))
}

// IsMusicDir comprueba si la ruta especificada corresponde a un directorio existente en disco
// y contiene al menos un archivo de audio compatible, ya sea en la raíz o en cualquier subcarpeta.
// La exploración recursiva se interrumpe inmediatamente tras encontrar la primera coincidencia válida.
func IsMusicDir(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	var found bool
	// Explorar el árbol de directorios abortando en el primer archivo compatible mediante un error centinela
	walkErr := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if IsSupportedAudio(d.Name()) {
			found = true
			return errors.New("found")
		}
		return nil
	})
	if walkErr != nil && walkErr.Error() == "found" {
		return true
	}
	return found
}
