package menu

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMenuDynamicOptions(t *testing.T) {
	m := New()

	// Estado detenido por defecto
	items := m.getMenuItems()
	if len(items) != 7 {
		t.Fatalf("se esperaban 7 opciones, se obtuvieron %d", len(items))
	}
	if items[0].label != "Reproducir pista" {
		t.Errorf("esperado 'Reproducir pista', obtenido '%s'", items[0].label)
	}

	// Estado reproduciendo
	m.SetState(PlayerState{
		State:              "Reproduciendo",
		IsPlaying:          true,
		CurrentTrackTitle:  "Track 1",
		CurrentTrackArtist: "Artist 1",
		TrackCount:         10,
		Volume:             70,
	})

	items = m.getMenuItems()
	if items[0].label != "Pausar música" {
		t.Errorf("esperado 'Pausar música', obtenido '%s'", items[0].label)
	}

	// Estado pausado
	m.SetState(PlayerState{
		State:              "Pausado",
		IsPaused:           true,
		CurrentTrackTitle:  "Track 1",
		CurrentTrackArtist: "Artist 1",
		TrackCount:         10,
		Volume:             70,
	})

	items = m.getMenuItems()
	if items[0].label != "Reanudar música" {
		t.Errorf("esperado 'Reanudar música', obtenido '%s'", items[0].label)
	}
}

func TestMenuKeyShortcuts(t *testing.T) {
	m := New()

	// Prueba número directo '2' (Ir al reproductor)
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if cmd == nil {
		t.Fatal("se esperaba un comando al presionar '2'")
	}
	msg := cmd()
	selectMsg, ok := msg.(SelectMsg)
	if !ok {
		t.Fatalf("se esperaba SelectMsg, obtenido %T", msg)
	}
	if selectMsg.Action != ActionViewPlayer {
		t.Errorf("esperado ActionViewPlayer, obtenido %v", selectMsg.Action)
	}

	// Prueba de tecla 'p'
	_, cmdP := newModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmdP == nil {
		t.Fatal("se esperaba un comando al presionar 'p'")
	}
	msgP := cmdP()
	selectMsgP, ok := msgP.(SelectMsg)
	if !ok || selectMsgP.Action != ActionViewPlayer {
		t.Errorf("esperado ActionViewPlayer al pulsar 'p', obtenido %v", msgP)
	}

	// Prueba de tecla 'esc'
	_, cmdEsc := newModel.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmdEsc == nil {
		t.Fatal("se esperaba un comando al presionar 'esc'")
	}
	msgEsc := cmdEsc()
	selectMsgEsc, ok := msgEsc.(SelectMsg)
	if !ok || selectMsgEsc.Action != ActionViewPlayer {
		t.Errorf("esperado ActionViewPlayer al pulsar 'esc', obtenido %v", msgEsc)
	}
}

func TestMenuViewRendering(t *testing.T) {
	m := New()
	m.SetState(PlayerState{
		State:              "Reproduciendo",
		IsPlaying:          true,
		CurrentTrackTitle:  "Song A",
		CurrentTrackArtist: "Artist B",
		TrackCount:         5,
		Volume:             65,
		HasTrack:           true,
	})

	// Render sin tamaño establecido
	view := m.View()
	if !strings.Contains(view, "Song A") {
		t.Errorf("la vista debe contener el título de la canción 'Song A'")
	}
	if !strings.Contains(view, "Artist B") {
		t.Errorf("la vista debe contener el artista 'Artist B'")
	}

	// Render con redimensionado (WindowSizeMsg)
	resizedModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	viewResized := resizedModel.View()
	if !strings.Contains(viewResized, "Song A") {
		t.Errorf("la vista redimensionada debe contener el título de la canción")
	}
}
