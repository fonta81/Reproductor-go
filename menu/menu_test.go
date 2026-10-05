// Package menu reúne las pruebas unitarias para validar la reactividad del menú,
// la adaptación dinámica de opciones según el estado de reproducción, el manejo
// de pulsaciones de teclas y atajos rápidos, y el renderizado visual con Lipgloss.
package menu

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestMenuDynamicOptions verifica que los textos descriptivos e iconos de las opciones del menú
// se adapten de forma dinámica según el estado del reproductor (detenido, reproduciendo o pausado).
func TestMenuDynamicOptions(t *testing.T) {
	m := New()

	// 1. Verificación del estado inicial: reproducción detenida por defecto
	items := m.getMenuItems()
	if len(items) != 7 {
		t.Fatalf("se esperaban 7 opciones, se obtuvieron %d", len(items))
	}
	if items[0].label != "Reproducir pista" {
		t.Errorf("esperado 'Reproducir pista', obtenido '%s'", items[0].label)
	}

	// 2. Verificación de adaptación dinámica al pasar al estado de reproducción activa
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

	// 3. Verificación de adaptación dinámica al pasar al estado de pausa
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

// TestMenuKeyShortcuts valida el procesamiento de atajos de teclado numéricos directos ('2')
// y teclas rápidas de navegación ('p', 'esc'), asegurando la emisión correcta de SelectMsg.
func TestMenuKeyShortcuts(t *testing.T) {
	m := New()

	// 1. Verificación de selección directa con dígito numérico '2' (Ir al reproductor)
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

	// 2. Verificación del atajo de teclado alfanumérico 'p' para conmutar a la vista del reproductor
	_, cmdP := newModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmdP == nil {
		t.Fatal("se esperaba un comando al presionar 'p'")
	}
	msgP := cmdP()
	selectMsgP, ok := msgP.(SelectMsg)
	if !ok || selectMsgP.Action != ActionViewPlayer {
		t.Errorf("esperado ActionViewPlayer al pulsar 'p', obtenido %v", msgP)
	}

	// 3. Verificación de la tecla 'esc' para retornar a la vista del reproductor
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

// TestMenuViewRendering comprueba que la vista del menú contenga los metadatos de la pista activa
// y gestione adecuadamente los mensajes de redimensionamiento de la ventana del terminal.
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

	// 1. Renderizado sin dimensiones previas explícitas de la ventana
	view := m.View()
	if !strings.Contains(view, "Song A") {
		t.Errorf("la vista debe contener el título de la canción 'Song A'")
	}
	if !strings.Contains(view, "Artist B") {
		t.Errorf("la vista debe contener el artista 'Artist B'")
	}

	// 2. Renderizado reactivo tras la recepción de evento WindowSizeMsg
	resizedModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	viewResized := resizedModel.View()
	if !strings.Contains(viewResized, "Song A") {
		t.Errorf("la vista redimensionada debe contener el título de la canción")
	}
}
