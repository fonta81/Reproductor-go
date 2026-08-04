package player

import (
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/faiface/beep"
)

// Configuración básica de la aplicación
const (
	appName         = "GoPlayer"
	appSubtitle     = "Reproductor de música TUI"
	defaultDir      = "./music" // Directorio predeterminado donde buscar archivos de música
	seekSeconds     = 10        // Número de segundos para saltar hacia adelante/atrás
	progressWidth   = 50
	defaultDuration = 3 * time.Minute // Duración predeterminada si falta metadatos

	standardSampleRate = beep.SampleRate(44100)
)

// Iconos utilizados en la interfaz TUI
const (
	iconPlay      = "󰐊"
	iconPause     = "󰏤"
	iconStop      = "󰓛"
	iconNext      = "󰒭"
	iconPrev      = "󰒮"
	iconRepeatOne = "󰑘"
	iconRepeatAll = "󰑗"
	iconRepeatOff = "󰁔"
	iconShuffle   = "󰒟"
	iconVolume    = "󰕾"
	iconMute      = "󰖁"
	iconQueue     = "󰉖"
	iconUp        = "󰅂"
	iconDown      = "󰅀"
	iconNav       = "󰍉"
	iconAudio     = "󰕾"
	iconSystem    = "󰒓"
	iconFolder    = "󰉋"
)

// Configuración de volumen en decibelios (dB)
const (
	volumeStep = 3.0   // Paso de 3 dB que representa un cambio claramente perceptible en el volumen.
	maxVolume  = 0.0   // 0 dBFS es el volumen digital máximo antes de que ocurra saturación (clipping).
	minVolume  = -30.0 // -30 dB se considera el umbral mínimo de volumen (virtualmente silencioso).
)

// Paleta de colores para la interfaz TUI (Dracula theme)
var (
	pink       = lipgloss.Color("#FF79C6")
	cyan       = lipgloss.Color("#8BE9FD")
	green      = lipgloss.Color("#50FA7B")
	yellow     = lipgloss.Color("#F1FA8C")
	orange     = lipgloss.Color("#FFB86C")
	red        = lipgloss.Color("#FF5555")
	purple     = lipgloss.Color("#BD93F9")
	foreground = lipgloss.Color("#F8F8F2")
	comment    = lipgloss.Color("#6272A4")
	selection  = lipgloss.Color("#44475A")

	// Estilos de lipgloss para los componentes de la interfaz
	helpContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(selection).
				Padding(1, 2).
				MarginLeft(2).
				MarginTop(1)

	helpCategoryStyle = lipgloss.NewStyle().
				Foreground(purple).
				Bold(true).
				MarginBottom(1)

	helpKeyStyle = lipgloss.NewStyle().
			Background(selection).
			Foreground(cyan).
			Bold(true).
			Align(lipgloss.Center).
			Width(10).
			MarginRight(1).
			MarginBottom(1)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(comment).
			MarginBottom(1)
)
