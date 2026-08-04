package player

import (
	"fmt"
	"math/rand"
	"time"
)

// PlaybackState define los estados posibles de la reproducción.
type PlaybackState int

const (
	StateStopped PlaybackState = iota
	StatePlaying
	StatePaused
)

// Icon devuelve el icono correspondiente al estado de reproducción actual.
func (s PlaybackState) Icon() string {
	switch s {
	case StatePlaying:
		return iconPlay
	case StatePaused:
		return iconPause
	default:
		return iconStop
	}
}

// Label devuelve la etiqueta legible para el usuario del estado de reproducción actual.
func (s PlaybackState) Label() string {
	switch s {
	case StatePlaying:
		return "Reproduciendo"
	case StatePaused:
		return "Pausado"
	default:
		return "Detenido"
	}
}

// RepeatMode define los modos de repetición de la lista de reproducción.
type RepeatMode int

const (
	RepeatOff RepeatMode = iota
	RepeatOne
	RepeatAll
)

// Icon devuelve el icono correspondiente al modo de repetición seleccionado.
func (r RepeatMode) Icon() string {
	switch r {
	case RepeatOne:
		return iconRepeatOne
	case RepeatAll:
		return iconRepeatAll
	default:
		return iconRepeatOff
	}
}

// Track representa una pista de audio con sus metadatos.
type Track struct {
	ID       string
	Title    string
	Artist   string
	Album    string
	Duration time.Duration
	Path     string // Ruta absoluta o relativa al archivo de audio físico
}

// DisplayName devuelve una cadena formateada con el artista y título de la pista.
func (t Track) DisplayName() string {
	if t.Artist == "" {
		return t.Title
	}
	return fmt.Sprintf("%s — %s", t.Artist, t.Title)
}

// FormattedDuration devuelve la duración de la pista como una cadena de tiempo legible.
func (t Track) FormattedDuration() string {
	if t.Duration <= 0 {
		return "?:??"
	}
	return formatDuration(t.Duration)
}

// Playlist gestiona la lista de pistas, el orden y los modos de reproducción.
type Playlist struct {
	tracks          []Track
	current         int
	shuffle         bool
	repeat          RepeatMode
	shuffleOrder    []int
	shuffleIdx      int
	shuffleStartIdx int
}

// NewPlaylist inicializa una nueva lista de reproducción vacía.
func NewPlaylist() *Playlist {
	return &Playlist{
		tracks:  make([]Track, 0),
		current: -1,
	}
}

// Add añade una pista a la lista de reproducción.
func (p *Playlist) Add(track Track) {
	p.tracks = append(p.tracks, track)
	if p.current == -1 {
		p.current = 0
	}
	if p.shuffle {
		p.regenerateShuffle()
	}
}

// Remove elimina una pista de la lista de reproducción por índice.
func (p *Playlist) Remove(index int) {
	if !p.isValidIndex(index) {
		return
	}
	p.tracks = append(p.tracks[:index], p.tracks[index+1:]...)
	if p.shuffle {
		p.rebuildShuffleAfterRemove(index)
	}
	if p.current >= len(p.tracks) {
		p.current = len(p.tracks) - 1
	}
	if len(p.tracks) == 0 {
		p.current = -1
		p.shuffleOrder = nil
	} else if p.shuffle {
		p.shuffleIdx = p.findInShuffleOrder(p.current)
		p.shuffleStartIdx = p.shuffleIdx
	}
}

// Clear vacía la lista de reproducción.
func (p *Playlist) Clear() {
	p.tracks = make([]Track, 0)
	p.current = -1
	p.shuffleOrder = nil
	p.shuffleIdx = 0
	p.shuffleStartIdx = 0
}

// Current devuelve la pista actualmente seleccionada en la lista de reproducción.
func (p *Playlist) Current() (Track, bool) {
	if !p.isValidIndex(p.current) {
		return Track{}, false
	}
	return p.tracks[p.current], true
}

// Next determina y devuelve la siguiente pista basada en el modo actual.
func (p *Playlist) Next() (Track, bool) {
	if len(p.tracks) == 0 {
		return Track{}, false
	}
	if p.repeat == RepeatOne && p.isValidIndex(p.current) {
		return p.tracks[p.current], true
	}
	if p.shuffle {
		if len(p.shuffleOrder) == 0 {
			p.regenerateShuffle()
			if len(p.shuffleOrder) == 0 {
				return Track{}, false
			}
		}
		nextIdx := (p.shuffleIdx + 1) % len(p.shuffleOrder)
		if p.repeat == RepeatOff && nextIdx == p.shuffleStartIdx {
			return Track{}, false
		}
		p.shuffleIdx = nextIdx
		p.current = p.shuffleOrder[p.shuffleIdx]
		return p.tracks[p.current], true
	}
	if p.repeat == RepeatAll {
		p.current = (p.current + 1) % len(p.tracks)
		return p.tracks[p.current], true
	}
	if p.isLastSequential() {
		return Track{}, false
	}
	p.current++
	return p.tracks[p.current], true
}

// Previous determina y devuelve la pista anterior basada en el modo actual.
func (p *Playlist) Previous() (Track, bool) {
	if len(p.tracks) == 0 || p.current < 0 {
		return Track{}, false
	}
	if p.shuffle {
		if len(p.shuffleOrder) == 0 {
			return Track{}, false
		}
		p.shuffleIdx--
		if p.shuffleIdx < 0 {
			p.shuffleIdx = len(p.shuffleOrder) - 1
		}
		p.current = p.shuffleOrder[p.shuffleIdx]
		return p.tracks[p.current], true
	}
	if p.current <= 0 {
		return Track{}, false
	}
	p.current--
	return p.tracks[p.current], true
}

// JumpTo cambia la pista actual a la especificada por índice.
func (p *Playlist) JumpTo(index int) bool {
	if !p.isValidIndex(index) {
		return false
	}
	p.current = index
	if p.shuffle {
		p.shuffleIdx = p.findInShuffleOrder(index)
		p.shuffleStartIdx = p.shuffleIdx
	}
	return true
}

// ToggleShuffle activa o desactiva el modo aleatorio.
func (p *Playlist) ToggleShuffle() {
	p.shuffle = !p.shuffle
	if p.shuffle && len(p.tracks) > 0 {
		p.regenerateShuffle()
	} else {
		p.shuffleOrder = nil
		p.shuffleIdx = 0
		p.shuffleStartIdx = 0
	}
}

// Length devuelve el número total de pistas en la lista.
func (p *Playlist) Length() int { return len(p.tracks) }

// IsEmpty verifica si la lista de reproducción está vacía.
func (p *Playlist) IsEmpty() bool { return len(p.tracks) == 0 }

// isLast comprueba si la pista actual es la última, según el modo de repetición.
func (p *Playlist) isLast() bool {
	if p.repeat != RepeatOff {
		return false
	}
	if p.shuffle {
		if len(p.shuffleOrder) == 0 {
			return true
		}
		return (p.shuffleIdx+1)%len(p.shuffleOrder) == p.shuffleStartIdx
	}
	return p.isLastSequential()
}

// isLastSequential comprueba si la pista actual es la última en orden secuencial.
func (p *Playlist) isLastSequential() bool {
	return p.current >= len(p.tracks)-1
}

// isValidIndex comprueba si el índice dado está dentro de los límites de la lista.
func (p *Playlist) isValidIndex(index int) bool {
	return index >= 0 && index < len(p.tracks)
}

// regenerateShuffle regenera el orden aleatorio de las pistas.
func (p *Playlist) regenerateShuffle() {
	n := len(p.tracks)
	if n == 0 {
		return
	}
	p.shuffleOrder = make([]int, n)
	for i := range n {
		p.shuffleOrder[i] = i
	}
	for i := n - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		p.shuffleOrder[i], p.shuffleOrder[j] = p.shuffleOrder[j], p.shuffleOrder[i]
	}
	p.shuffleIdx = p.findInShuffleOrder(p.current)
	p.shuffleStartIdx = p.shuffleIdx
}

// rebuildShuffleAfterRemove actualiza el orden aleatorio tras eliminar una pista.
func (p *Playlist) rebuildShuffleAfterRemove(removedIndex int) {
	newOrder := make([]int, 0, len(p.shuffleOrder)-1)
	for _, idx := range p.shuffleOrder {
		if idx == removedIndex {
			continue
		}
		if idx > removedIndex {
			idx--
		}
		newOrder = append(newOrder, idx)
	}
	p.shuffleOrder = newOrder
}

// findInShuffleOrder busca el índice de la pista en el orden aleatorio.
func (p *Playlist) findInShuffleOrder(trackIndex int) int {
	for i, idx := range p.shuffleOrder {
		if idx == trackIndex {
			return i
		}
	}
	return 0
}
