package menu

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	appStyle = lipgloss.NewStyle().Margin(1, 2)

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 2).
		MarginBottom(1)

	itemStyle = lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(lipgloss.Color("#B0B0B0"))

	selectedItemStyle = lipgloss.NewStyle().
		PaddingLeft(1).
		Foreground(lipgloss.Color("#EE6FF8")).
		Bold(true).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("#EE6FF8"))

	footerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		MarginTop(2)
)

type Model struct {
	choices  []string
	cursor   int
	selected string
}

func New() Model {
	return Model{
		choices: []string{
			"Reproducir pista",
			"Pausar / Detener",
			"Siguiente canción",
			"Anterior canción",
			"Biblioteca musical",
			"Configuración",
			"Salir",
		},
		cursor: 0,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(m.choices) - 1
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
		case "enter", " ":
			m.selected = m.choices[m.cursor]
			if m.selected == "Salir" {
				return m, tea.Quit
			}
			return m, func() tea.Msg {
				return SelectMsg{Choice: m.selected}
			}
		}
	}
	return m, nil
}

type SelectMsg struct {
	Choice string
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" Bubbletea Go - Reproductor "))
	b.WriteString("\n")

	for i, choice := range m.choices {
		if m.cursor == i {
			b.WriteString(selectedItemStyle.Render("▶ " + choice))
		} else {
			b.WriteString(itemStyle.Render("  " + choice))
		}
		b.WriteString("\n")
	}

	if m.selected != "" && m.selected != "Salir" {
		status := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Render(m.selected)
		b.WriteString(fmt.Sprintf("\nEstado: %s\n", status))
	} else {
		b.WriteString("\n\n")
	}

	b.WriteString(footerStyle.Render("↑/k: subir • ↓/j: bajar • enter: seleccionar • q: salir"))

	return appStyle.Render(b.String())
}
