package menu

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Action representa la acción solicitada por el usuario en el menú.
type Action int

const (
	ActionTogglePlay Action = iota
	ActionViewPlayer
	ActionNextTrack
	ActionPrevTrack
	ActionLibrary
	ActionSettings
	ActionQuit
)

// PlayerState contiene la información del reproductor necesaria para el menú.
type PlayerState struct {
	State                string // "Reproduciendo", "Pausado", "Detenido"
	IsPlaying            bool
	IsPaused             bool
	CurrentTrackTitle    string
	CurrentTrackArtist   string
	CurrentTrackDuration string
	TrackCount           int
	Volume               int // 0 - 100%
	IsMuted              bool
	HasTrack             bool
}

// SelectMsg se emite cuando el usuario selecciona una opción del menú.
type SelectMsg struct {
	Action Action
	Choice string
}

// UpdateStateMsg permite actualizar el estado del menú mediante Bubble Tea.
type UpdateStateMsg struct {
	State PlayerState
}

type menuItem struct {
	action Action
	label  string
	icon   string
	badge  string
}

// Estilos de lipgloss para Catppuccin Mocha / UI moderna
var (
	colorAccent    = lipgloss.Color("#CBA6F7") // Lavanda
	colorSecondary = lipgloss.Color("#94E2D5") // Cyan / Teal
	colorGreen     = lipgloss.Color("#A6E3A1") // Verde
	colorYellow    = lipgloss.Color("#F9E2AF") // Amarillo
	colorOrange    = lipgloss.Color("#FAB387") // Naranja
	colorRed       = lipgloss.Color("#F38BA8") // Rojo
	colorText      = lipgloss.Color("#CDD6F4") // Texto principal
	colorSubtext   = lipgloss.Color("#A6ADC8") // Texto secundario
	colorMuted     = lipgloss.Color("#6C7086") // Texto atenuado / bordes tenues
	colorSurface   = lipgloss.Color("#313244") // Superficie de selección
	colorDarkBg    = lipgloss.Color("#181825") // Fondo oscuro

	menuBoxStyle = lipgloss.NewStyle().
			Width(58).
			Padding(0, 1)

	headerBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Width(56).
			Align(lipgloss.Center).
			Padding(0, 1).
			MarginBottom(1)

	headerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent)

	headerSubStyle = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Italic(true)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Width(56).
			Padding(0, 1).
			MarginBottom(1)

	cardHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			MarginBottom(0)

	trackTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText)

	trackArtistStyle = lipgloss.NewStyle().
				Foreground(colorSecondary)

	metaInfoStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	itemBadgeNormal = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	itemBadgeSelected = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	selectedItemStyle = lipgloss.NewStyle().
				Background(colorSurface).
				Foreground(colorAccent).
				Bold(true).
				Padding(0, 1).
				Width(56)

	unselectedItemStyle = lipgloss.NewStyle().
				Foreground(colorText).
				Padding(0, 1).
				Width(56)

	footerStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Align(lipgloss.Center).
			Width(56).
			MarginTop(1)
)

// Model representa el estado del componente de menú.
type Model struct {
	cursor   int
	selected string
	state    PlayerState
	width    int
	height   int
}

// New inicializa una nueva instancia del modelo de menú.
func New() Model {
	return Model{
		cursor: 0,
	}
}

// SetState actualiza directamente el estado del reproductor reflejado en el menú.
func (m *Model) SetState(s PlayerState) {
	m.state = s
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) getMenuItems() []menuItem {
	var playLabel, playIcon string
	if m.state.IsPlaying {
		playLabel = "Pausar música"
		playIcon = "⏸"
	} else if m.state.IsPaused {
		playLabel = "Reanudar música"
		playIcon = "▶"
	} else {
		playLabel = "Reproducir pista"
		playIcon = "▶"
	}

	return []menuItem{
		{action: ActionTogglePlay, label: playLabel, icon: playIcon, badge: "1"},
		{action: ActionViewPlayer, label: "Ir al reproductor (Vista completa)", icon: "📺", badge: "2"},
		{action: ActionNextTrack, label: "Siguiente canción", icon: "⏭", badge: "3"},
		{action: ActionPrevTrack, label: "Anterior canción", icon: "⏮", badge: "4"},
		{action: ActionLibrary, label: "Biblioteca musical", icon: "📁", badge: "5"},
		{action: ActionSettings, label: "Configuración (Cambiar carpeta)", icon: "⚙", badge: "6"},
		{action: ActionQuit, label: "Salir", icon: "🚪", badge: "7"},
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case UpdateStateMsg:
		m.state = msg.State
		return m, nil

	case tea.KeyMsg:
		items := m.getMenuItems()

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "p", "esc":
			return m, func() tea.Msg {
				return SelectMsg{
					Action: ActionViewPlayer,
					Choice: "Ir al reproductor (Vista completa)",
				}
			}

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(items) - 1
			}

		case "down", "j":
			if m.cursor < len(items)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}

		case "1", "2", "3", "4", "5", "6", "7":
			idx := int(msg.Runes[0] - '1')
			if idx >= 0 && idx < len(items) {
				m.cursor = idx
				selected := items[idx]
				m.selected = selected.label
				if selected.action == ActionQuit {
					return m, tea.Quit
				}
				return m, func() tea.Msg {
					return SelectMsg{
						Action: selected.action,
						Choice: selected.label,
					}
				}
			}

		case "enter", " ":
			if m.cursor >= 0 && m.cursor < len(items) {
				selected := items[m.cursor]
				m.selected = selected.label
				if selected.action == ActionQuit {
					return m, tea.Quit
				}
				return m, func() tea.Msg {
					return SelectMsg{
						Action: selected.action,
						Choice: selected.label,
					}
				}
			}
		}
	}
	return m, nil
}

func (m Model) renderNowPlayingCard() string {
	var borderColor lipgloss.Color
	var statusBadge string

	if m.state.IsPlaying {
		borderColor = colorGreen
		statusBadge = lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("▶ REPRODUCIENDO")
	} else if m.state.IsPaused {
		borderColor = colorYellow
		statusBadge = lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Render("⏸ PAUSADO")
	} else {
		borderColor = colorMuted
		statusBadge = lipgloss.NewStyle().Foreground(colorSubtext).Bold(true).Render("⏹ DETENIDO")
	}

	var volText string
	if m.state.IsMuted {
		volText = lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render("󰖁 MUTE")
	} else {
		filled := m.state.Volume / 10
		if filled > 10 {
			filled = 10
		} else if filled < 0 {
			filled = 0
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)
		volText = fmt.Sprintf("🔊 %s %d%%", bar, m.state.Volume)
	}

	trackCountText := fmt.Sprintf("📁 %d pistas", m.state.TrackCount)
	footerMeta := metaInfoStyle.Render(fmt.Sprintf("%s   •   %s", volText, trackCountText))

	var b strings.Builder
	b.WriteString(cardHeaderStyle.Render(statusBadge) + "\n")

	if m.state.HasTrack && m.state.CurrentTrackTitle != "" {
		title := m.state.CurrentTrackTitle
		if len(title) > 48 {
			title = title[:45] + "..."
		}
		b.WriteString(trackTitleStyle.Render("🎵 " + title) + "\n")

		artist := m.state.CurrentTrackArtist
		if artist == "" {
			artist = "Artista desconocido"
		}
		if len(artist) > 48 {
			artist = artist[:45] + "..."
		}
		b.WriteString(trackArtistStyle.Render("👤 " + artist) + "\n")
	} else {
		if m.state.TrackCount == 0 {
			b.WriteString(metaInfoStyle.Render("Sin pistas en biblioteca. Pulsa [5] o [6] para explorar carpetas.") + "\n")
		} else {
			b.WriteString(metaInfoStyle.Render("Reproducción en espera. Selecciona [1] para iniciar.") + "\n")
		}
	}

	b.WriteString(footerMeta)

	return cardStyle.Copy().BorderForeground(borderColor).Render(b.String())
}

func (m Model) View() string {
	var b strings.Builder

	// 1. Header con título estilizado
	headerContent := fmt.Sprintf("%s\n%s",
		headerTitleStyle.Render("🎵  G O P L A Y E R  •  M E N Ú"),
		headerSubStyle.Render("Reproductor de Audio TUI"),
	)
	b.WriteString(headerBoxStyle.Render(headerContent))
	b.WriteString("\n")

	// 2. Mini-Widget "Now Playing"
	b.WriteString(m.renderNowPlayingCard())
	b.WriteString("\n")

	// 3. Opciones del Menú
	items := m.getMenuItems()
	for i, item := range items {
		if m.cursor == i {
			line := fmt.Sprintf("▸ %s  %s %s",
				itemBadgeSelected.Render("["+item.badge+"]"),
				item.icon,
				item.label,
			)
			b.WriteString(selectedItemStyle.Render(line))
		} else {
			line := fmt.Sprintf("  %s  %s %s",
				itemBadgeNormal.Render("["+item.badge+"]"),
				item.icon,
				item.label,
			)
			b.WriteString(unselectedItemStyle.Render(line))
		}
		b.WriteString("\n")
	}

	// 4. Footer con atajos de ayuda
	helpText := "1-7: selección • ↑/↓: navegar • enter: elegir\nesc/p: reproductor • q: salir"
	b.WriteString(footerStyle.Render(helpText))

	content := menuBoxStyle.Render(b.String())

	// Centrado adaptable si se conocen las dimensiones de la terminal
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	return content
}
