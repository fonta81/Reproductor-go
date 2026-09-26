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
	r := RootModel{
		currentScreen: ScreenMenu,
		menuModel:     menu.New(),
		playerModel:   NewAppModel(initialDir),
	}
	r.syncMenuState()
	return r
}

func (r *RootModel) syncMenuState() {
	var trackCount int
	if r.playerModel.playlist != nil {
		trackCount = r.playerModel.playlist.Length()
	}

	// Calcular volumen en porcentaje 0-100%
	var volPct int
	if maxVolume != minVolume {
		pct := (r.playerModel.volumeLevel - minVolume) / (maxVolume - minVolume)
		if pct < 0 {
			pct = 0
		} else if pct > 1 {
			pct = 1
		}
		volPct = int(pct * 100)
	}

	isMuted := false
	if r.playerModel.Audio != nil {
		isMuted = r.playerModel.Audio.IsMuted()
	}

	state := menu.PlayerState{
		TrackCount: trackCount,
		Volume:     volPct,
		IsMuted:    isMuted,
	}

	switch r.playerModel.state {
	case StatePlaying:
		state.State = "Reproduciendo"
		state.IsPlaying = true
	case StatePaused:
		state.State = "Pausado"
		state.IsPaused = true
	default:
		state.State = "Detenido"
	}

	if r.playerModel.playlist != nil {
		if track, ok := r.playerModel.playlist.Current(); ok {
			state.HasTrack = true
			state.CurrentTrackTitle = track.Title
			state.CurrentTrackArtist = track.Artist
			if track.Duration > 0 {
				state.CurrentTrackDuration = formatDuration(track.Duration)
			}
		}
	}

	r.menuModel.SetState(state)
}

func (r RootModel) Init() tea.Cmd {
	r.syncMenuState()
	return tea.Batch(
		r.menuModel.Init(),
		r.playerModel.Init(),
	)
}

func (r RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		var cmdMenu, cmdPlayer tea.Cmd
		var mm, pm tea.Model

		mm, cmdMenu = r.menuModel.Update(msg)
		r.menuModel = mm.(menu.Model)

		pm, cmdPlayer = r.playerModel.Update(msg)
		r.playerModel = pm.(AppModel)

		r.syncMenuState()
		return r, tea.Batch(cmdMenu, cmdPlayer)

	case menu.SelectMsg:
		switch msg.Action {
		case menu.ActionTogglePlay:
			if r.playerModel.state == StateStopped {
				if r.playerModel.playlist.Length() > 0 {
					var pm tea.Model
					var cmd tea.Cmd
					pm, cmd = r.playerModel.playCurrent()
					r.playerModel = pm.(AppModel)
					r.currentScreen = ScreenPlayer
					r.syncMenuState()
					return r, cmd
				}
				r.syncMenuState()
				return r, nil
			}
			var pm tea.Model
			var cmd tea.Cmd
			pm, cmd = r.playerModel.togglePlayback()
			r.playerModel = pm.(AppModel)
			r.syncMenuState()
			return r, cmd

		case menu.ActionViewPlayer:
			r.currentScreen = ScreenPlayer
			r.syncMenuState()
			return r, nil

		case menu.ActionNextTrack:
			var pm tea.Model
			var cmd tea.Cmd
			pm, cmd = r.playerModel.playNext()
			r.playerModel = pm.(AppModel)
			r.syncMenuState()
			return r, cmd

		case menu.ActionPrevTrack:
			var pm tea.Model
			var cmd tea.Cmd
			pm, cmd = r.playerModel.playPrevious()
			r.playerModel = pm.(AppModel)
			r.syncMenuState()
			return r, cmd

		case menu.ActionLibrary:
			r.currentScreen = ScreenPlayer
			r.playerModel.isPickingFolder = false
			r.playerModel.isFiltering = false
			r.syncMenuState()
			return r, nil

		case menu.ActionSettings:
			r.currentScreen = ScreenPlayer
			r.playerModel.isPickingFolder = true
			r.playerModel.isFiltering = false
			initialDir := r.playerModel.musicDir
			if initialDir == "" {
				initialDir = "."
			}
			cmd := r.playerModel.loadBrowserDir(initialDir)
			r.syncMenuState()
			return r, cmd

		case menu.ActionQuit:
			r.playerModel.Audio.Close()
			return r, tea.Quit
		}

	case tea.KeyMsg:
		// Global quit from menu/player using ctrl+c
		if msg.String() == "ctrl+c" {
			r.playerModel.Audio.Close()
			return r, tea.Quit
		}

		if r.currentScreen == ScreenMenu {
			if msg.String() == "esc" || msg.String() == "p" {
				r.currentScreen = ScreenPlayer
				r.syncMenuState()
				return r, nil
			}

			var menuModel tea.Model
			var cmd tea.Cmd
			menuModel, cmd = r.menuModel.Update(msg)
			r.menuModel = menuModel.(menu.Model)
			r.syncMenuState()
			cmds = append(cmds, cmd)
			return r, tea.Batch(cmds...)
		} else {
			// In player screen
			// If Esc is pressed on player screen (not filtering or folder picking), go back to menu
			if msg.String() == "esc" && !r.playerModel.isFiltering && !r.playerModel.isPickingFolder {
				r.currentScreen = ScreenMenu
				r.syncMenuState()
				return r, nil
			}
			var playerModel tea.Model
			var cmd tea.Cmd
			playerModel, cmd = r.playerModel.Update(msg)
			r.playerModel = playerModel.(AppModel)
			r.syncMenuState()
			cmds = append(cmds, cmd)
			return r, tea.Batch(cmds...)
		}
	}

	// For non-key messages (ticks, audio finished, library scanned, etc.), forward to both models
	var menuModel tea.Model
	var playerModel tea.Model
	var cmdMenu, cmdPlayer tea.Cmd

	menuModel, cmdMenu = r.menuModel.Update(msg)
	r.menuModel = menuModel.(menu.Model)

	playerModel, cmdPlayer = r.playerModel.Update(msg)
	r.playerModel = playerModel.(AppModel)

	r.syncMenuState()

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
