package player

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fonta81/Reproductor-go/menu"
)

type Screen int

const (
	ScreenMenu Screen = iota
	ScreenPlayer
)

type RootModel struct {
	currentScreen Screen
	menuModel     menu.Model
	playerModel   AppModel
}

func NewRootModel(initialDir string) RootModel {
	return RootModel{
		currentScreen: ScreenMenu,
		menuModel:     menu.New(),
		playerModel:   NewAppModel(initialDir),
	}
}

func (r RootModel) Init() tea.Cmd {
	return tea.Batch(
		r.menuModel.Init(),
		r.playerModel.Init(),
	)
}

func (r RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case menu.SelectMsg:
		switch msg.Choice {
		case "Reproducir pista":
			r.currentScreen = ScreenPlayer
			if r.playerModel.state == StateStopped {
				var cmd tea.Cmd
				var pm tea.Model
				pm, cmd = r.playerModel.playCurrent()
				r.playerModel = pm.(AppModel)
				return r, cmd
			}
			return r, nil

		case "Pausar / Detener":
			var cmd tea.Cmd
			var pm tea.Model
			pm, cmd = r.playerModel.togglePlayback()
			r.playerModel = pm.(AppModel)
			return r, cmd

		case "Siguiente canción":
			var cmd tea.Cmd
			var pm tea.Model
			pm, cmd = r.playerModel.playNext()
			r.playerModel = pm.(AppModel)
			return r, cmd

		case "Anterior canción":
			var cmd tea.Cmd
			var pm tea.Model
			pm, cmd = r.playerModel.playPrevious()
			r.playerModel = pm.(AppModel)
			return r, cmd

		case "Biblioteca musical":
			r.currentScreen = ScreenPlayer
			r.playerModel.isPickingFolder = true
			initialDir := r.playerModel.musicDir
			if initialDir == "" {
				initialDir = "."
			}
			cmd := r.playerModel.loadBrowserDir(initialDir)
			return r, cmd

		case "Configuración":
			r.currentScreen = ScreenPlayer
			r.playerModel.isPickingFolder = true
			initialDir := r.playerModel.musicDir
			if initialDir == "" {
				initialDir = "."
			}
			cmd := r.playerModel.loadBrowserDir(initialDir)
			return r, cmd
		}

	case tea.KeyMsg:
		// Global quit from menu/player using ctrl+c
		if msg.String() == "ctrl+c" {
			r.playerModel.Audio.Close()
			return r, tea.Quit
		}

		if r.currentScreen == ScreenMenu {
			var menuModel tea.Model
			var cmd tea.Cmd
			menuModel, cmd = r.menuModel.Update(msg)
			r.menuModel = menuModel.(menu.Model)
			cmds = append(cmds, cmd)
			return r, tea.Batch(cmds...)
		} else {
			// In player screen
			// If Esc is pressed on player screen (not filtering or folder picking), go back to menu
			if msg.String() == "esc" && !r.playerModel.isFiltering && !r.playerModel.isPickingFolder {
				r.currentScreen = ScreenMenu
				return r, nil
			}
			var playerModel tea.Model
			var cmd tea.Cmd
			playerModel, cmd = r.playerModel.Update(msg)
			r.playerModel = playerModel.(AppModel)
			cmds = append(cmds, cmd)
			return r, tea.Batch(cmds...)
		}
	}

	// For non-key messages (ticks, audio finished, etc.), forward to both models
	var menuModel tea.Model
	var playerModel tea.Model
	var cmdMenu, cmdPlayer tea.Cmd

	menuModel, cmdMenu = r.menuModel.Update(msg)
	r.menuModel = menuModel.(menu.Model)

	playerModel, cmdPlayer = r.playerModel.Update(msg)
	r.playerModel = playerModel.(AppModel)

	cmds = append(cmds, cmdMenu, cmdPlayer)
	return r, tea.Batch(cmds...)
}

func (r RootModel) View() string {
	if r.currentScreen == ScreenMenu {
		return r.menuModel.View()
	}
	return r.playerModel.View()
}

func (r RootModel) Close() {
	r.playerModel.Audio.Close()
}
