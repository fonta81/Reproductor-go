// Package player define constantes globales de configuración, iconos, colores y estilos de interfaz.
package player

import (
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/faiface/beep"
)

// Constantes generales de configuración de la aplicación.
const (
	appName         = "GoPlayer"
	appSubtitle     = "Reproductor de música TUI"
	defaultDir      = "./music"       // Directorio predeterminado donde buscar archivos de música si no se especifica otro.
	seekSeconds     = 10              // Intervalo de segundos para saltar hacia adelante o atrás en la pista.
	progressWidth   = 50              // Ancho base por defecto para la barra de progreso en caracteres.
	defaultDuration = 3 * time.Minute // Duración predeterminada asignada si no es posible obtener los metadatos del archivo.

	// standardSampleRate define la frecuencia de muestreo estandarizada (44.1 kHz, calidad CD)
	// a la cual el motor de audio remuestrea todas las fuentes para el reproductor de altavoces.
	standardSampleRate = beep.SampleRate(44100)
)

// Glifos e iconos (Nerd Fonts / Unicode) empleados en los componentes de la interfaz de usuario.
const (
	iconPlay      = "󰐊" // Icono de reproducción activa
	iconPause     = "󰏤" // Icono de reproducción pausada
	iconStop      = "󰓛" // Icono de reproducción detenida
	iconNext      = "󰒭" // Icono de salto a la siguiente pista
	iconPrev      = "󰒮" // Icono de salto a la pista anterior
	iconRepeatOne = "󰑘" // Icono de repetición de una pista
	iconRepeatAll = "󰑗" // Icono de repetición de toda la lista
	iconRepeatOff = "󰁔" // Icono de repetición desactivada
	iconShuffle   = "󰒟" // Icono de modo aleatorio activado
	iconVolume    = "󰕾" // Icono de indicador de volumen
	iconMute      = "󰖁" // Icono de audio silenciado
	iconQueue     = "󰉖" // Icono de cola de reproducción
	iconUp        = "󰅂" // Flecha indicadora de más elementos arriba
	iconDown      = "󰅀" // Flecha indicadora de más elementos abajo
	iconNav       = "󰍉" // Icono de navegación
	iconAudio     = "󰕾" // Icono de archivos de audio
	iconSystem    = "󰒓" // Icono de opciones del sistema
	iconFolder    = "󰉋" // Icono de carpeta o directorio
)

// Constantes para el ajuste de volumen expresado en decibelios (dB).
const (
	volumeStep = 3.0   // Paso de 3 dB que representa un cambio claramente perceptible en la presión sonora.
	maxVolume  = 0.0   // 0 dBFS representa la amplitud digital máxima sin distorsión por saturación (clipping).
	minVolume  = -30.0 // -30 dB se utiliza como umbral mínimo audible (virtualmente silencioso).
)

// Paleta de colores para la interfaz TUI inspirada en el tema Catppuccin Mocha.
var (
	pink       = lipgloss.Color("#F5C2E7") // Color de acento principal (títulos, pistas destacadas)
	cyan       = lipgloss.Color("#94E2D5") // Color de realce secundario (atajos, cursor, indicadores)
	green      = lipgloss.Color("#A6E3A1") // Estado de reproducción activa y volumen moderado
	yellow     = lipgloss.Color("#F9E2AF") // Estado pausado y volumen medio-alto
	orange     = lipgloss.Color("#FAB387") // Volumen alto y títulos de sección
	red        = lipgloss.Color("#F38BA8") // Errores, silencios y estado detenido
	purple     = lipgloss.Color("#CBA6F7") // Bordes de contenedores y categoría de ayuda
	foreground = lipgloss.Color("#CDD6F4") // Texto principal de alto contraste
	comment    = lipgloss.Color("#7F849C") // Texto secundario, duraciones e información atenuada
	selection  = lipgloss.Color("#313244") // Fondos de selecciones, barras inactivas y bordes

	// helpContainerStyle define el contenedor exterior para el panel de ayuda y atajos de teclado.
	helpContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(selection).
				Padding(1, 2).
				MarginLeft(2).
				MarginTop(1)

	// helpCategoryStyle define el encabezado estilizado de cada sección del panel de ayuda.
	helpCategoryStyle = lipgloss.NewStyle().
				Foreground(purple).
				Bold(true).
				MarginBottom(1)

	// helpKeyStyle define la apariencia visual del botón de tecla en el panel de ayuda.
	helpKeyStyle = lipgloss.NewStyle().
			Background(selection).
			Foreground(cyan).
			Bold(true).
			Align(lipgloss.Center).
			Width(10).
			MarginRight(1).
			MarginBottom(1)

	// helpDescStyle define el texto descriptivo asociado a cada tecla del panel de ayuda.
	helpDescStyle = lipgloss.NewStyle().
			Foreground(comment).
			MarginBottom(1)
)
