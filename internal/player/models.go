// Package player define las estructuras de datos y modelos de estado del reproductor de audio,
// incluyendo estados de reproducción, modos de repetición, metadatos de pistas y la gestión
// algorítmica de la lista de reproducción con soporte para orden secuencial y aleatorio.
package player

import (
	"fmt"
	"math/rand"
	"time"
)

// PlaybackState define los estados posibles en el ciclo de vida de la reproducción de audio.
type PlaybackState int

// Enumeración de los estados de reproducción admitidos.
const (
	StateStopped PlaybackState = iota // Reproducción totalmente detenida; no hay flujo de audio activo.
	StatePlaying                      // Flujo de audio activo enviando muestras al altavoz.
	StatePaused                       // Reproducción suspendida temporalmente preservando la posición actual.
)

// Icon devuelve el glifo visual (Nerd Font / Unicode) representativo del estado de reproducción actual.
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

// Label devuelve la descripción textual legible para el usuario en la interfaz según el estado de reproducción activo.
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

// RepeatMode define las políticas de repetición configurables para el recorrido de la lista de reproducción.
type RepeatMode int

// Modos de repetición soportados por la lista de reproducción.
const (
	RepeatOff RepeatMode = iota // Sin repetición; la reproducción se detiene al alcanzar el final de la lista.
	RepeatOne                   // Repetición continua de la pista actualmente seleccionada en bucle infinito.
	RepeatAll                   // Repetición cíclica de la lista completa al concluir la última pista.
)

// Icon devuelve el glifo visual correspondiente al modo de repetición configurado.
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

// Track encapsula los metadatos y la ubicación física en el sistema de archivos de una pista de audio.
type Track struct {
	ID       string        // Identificador único asignado a la pista dentro de la biblioteca musical
	Title    string        // Título de la pista extraído de las etiquetas ID3/Vorbis o del nombre de archivo
	Artist   string        // Nombre del artista o grupo musical intérprete
	Album    string        // Nombre del álbum discográfico al que pertenece la pista
	Duration time.Duration // Duración total del audio decodificado o estimada desde metadatos
	Path     string        // Ruta absoluta o relativa al archivo físico en el almacenamiento
}

// DisplayName devuelve una cadena formateada que combina el artista y el título ("Artista — Título").
// Si el artista no está presente en los metadatos, devuelve exclusivamente el título de la canción.
func (t Track) DisplayName() string {
	if t.Artist == "" {
		return t.Title
	}
	return fmt.Sprintf("%s — %s", t.Artist, t.Title)
}

// FormattedDuration devuelve la duración de la pista como una cadena legible en formato "MM:SS".
// Si la duración no está disponible o es menor o igual a cero, retorna el marcador "?:??".
func (t Track) FormattedDuration() string {
	if t.Duration <= 0 {
		return "?:??"
	}
	return formatDuration(t.Duration)
}

// Playlist gestiona la colección de pistas en memoria, controlando el índice actual
// y aplicando algoritmos para la navegación lineal y aleatoria (shuffle), asegurando
// una transición ordenada y coherente entre canciones.
type Playlist struct {
	tracks          []Track    // Colección ordenada de pistas que integran la lista de reproducción
	current         int        // Índice de la pista activa en el arreglo tracks (-1 si la lista está vacía)
	shuffle         bool       // Indica si el modo aleatorio está activo
	repeat          RepeatMode // Modo de repetición configurado (Off, One, All)
	shuffleOrder    []int      // Permutación pseudoaleatoria de índices generada por el algoritmo Fisher-Yates
	shuffleIdx      int        // Posición actual del cursor dentro de la secuencia permutada shuffleOrder
	shuffleStartIdx int        // Índice de inicio de la permutación aleatoria para detectar ciclos completos
}

// NewPlaylist inicializa y devuelve una lista de reproducción vacía con el cursor inactivo (-1).
func NewPlaylist() *Playlist {
	return &Playlist{
		tracks:  make([]Track, 0),
		current: -1,
	}
}

// Add incorpora una nueva pista al final de la lista de reproducción.
// Si la lista estaba vacía, inicializa el cursor en la primera posición.
// Si el modo aleatorio se encuentra habilitado, regenera la secuencia permutada.
func (p *Playlist) Add(track Track) {
	p.tracks = append(p.tracks, track)
	if p.current == -1 {
		p.current = 0
	}
	if p.shuffle {
		p.regenerateShuffle()
	}
}

// Remove elimina la pista ubicada en el índice especificado garantizando la consistencia interna.
// Ajusta de forma segura el cursor actual y actualiza la permutación del modo aleatorio
// para sincronizar los índices restantes tras la eliminación.
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

// Clear vacía por completo la lista de reproducción y reinicia los cursores
// y las estructuras de seguimiento del modo aleatorio a su estado predeterminado.
func (p *Playlist) Clear() {
	p.tracks = make([]Track, 0)
	p.current = -1
	p.shuffleOrder = nil
	p.shuffleIdx = 0
	p.shuffleStartIdx = 0
}

// Current devuelve la pista actualmente seleccionada en el cursor de reproducción.
// Retorna una estructura Track vacía y false si el cursor no apunta a un elemento válido.
func (p *Playlist) Current() (Track, bool) {
	if !p.isValidIndex(p.current) {
		return Track{}, false
	}
	return p.tracks[p.current], true
}

// Next calcula y avanza a la siguiente pista de acuerdo con las reglas del modo de repetición
// y el estado del modo aleatorio. Devuelve la pista seleccionada y true, o una pista vacía
// y false si se alcanzó el final de la lista de reproducción sin repetición activa.
func (p *Playlist) Next() (Track, bool) {
	if len(p.tracks) == 0 {
		return Track{}, false
	}
	// Modo de repetición unitaria: mantiene la misma pista indefinidamente
	if p.repeat == RepeatOne && p.isValidIndex(p.current) {
		return p.tracks[p.current], true
	}
	// Modo de reproducción aleatoria (Shuffle)
	if p.shuffle {
		if len(p.shuffleOrder) == 0 {
			p.regenerateShuffle()
			if len(p.shuffleOrder) == 0 {
				return Track{}, false
			}
		}
		nextIdx := (p.shuffleIdx + 1) % len(p.shuffleOrder)
		// Si se completó un ciclo completo de barajado y la repetición está inactiva, detener reproducción
		if p.repeat == RepeatOff && nextIdx == p.shuffleStartIdx {
			return Track{}, false
		}
		p.shuffleIdx = nextIdx
		p.current = p.shuffleOrder[p.shuffleIdx]
		return p.tracks[p.current], true
	}
	// Modo de repetición completa: avanza de forma cíclica volviendo al primer elemento
	if p.repeat == RepeatAll {
		p.current = (p.current + 1) % len(p.tracks)
		return p.tracks[p.current], true
	}
	// Navegación secuencial estándar: detiene el avance si ya se alcanzó la última pista
	if p.isLastSequential() {
		return Track{}, false
	}
	p.current++
	return p.tracks[p.current], true
}

// Previous determina y retrocede a la pista previa en la secuencia según el modo activo
// (lineal o aleatorio). Devuelve la pista resultante y true, o una pista vacía y false
// si la lista está vacía o ya se encuentra al inicio de la secuencia.
func (p *Playlist) Previous() (Track, bool) {
	if len(p.tracks) == 0 || p.current < 0 {
		return Track{}, false
	}
	// Retroceso en orden aleatorio
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
	// Retroceso en orden secuencial
	if p.current <= 0 {
		return Track{}, false
	}
	p.current--
	return p.tracks[p.current], true
}

// JumpTo desplaza inmediatamente el cursor de reproducción al índice especificado.
// Si el modo aleatorio está activo, localiza y sincroniza la posición dentro de shuffleOrder.
// Retorna true si el índice es válido y el salto se realizó con éxito; false en caso contrario.
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

// ToggleShuffle alterna el estado del modo de reproducción aleatoria.
// Si se activa, calcula una nueva permutación mediante el algoritmo Fisher-Yates.
// Si se desactiva, libera la memoria de la permutación y restablece los cursores aleatorios.
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

// Length devuelve el número total de pistas almacenadas en la lista de reproducción.
func (p *Playlist) Length() int { return len(p.tracks) }

// IsEmpty verifica si la lista de reproducción carece de pistas registradas.
func (p *Playlist) IsEmpty() bool { return len(p.tracks) == 0 }

// isLast determina si la pista actual representa el último elemento de la secuencia de reproducción,
// evaluando las condiciones del modo de repetición y distinguiendo entre el orden lineal y aleatorio.
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

// isLastSequential comprueba si el cursor se sitúa en la última posición física del arreglo de pistas.
func (p *Playlist) isLastSequential() bool {
	return p.current >= len(p.tracks)-1
}

// isValidIndex valida si el índice proporcionado se encuentra dentro del rango admisible [0, len(tracks)-1].
func (p *Playlist) isValidIndex(index int) bool {
	return index >= 0 && index < len(p.tracks)
}

// regenerateShuffle construye una permutación pseudoaleatoria completa de los índices de pistas
// implementando el algoritmo de barajado Fisher-Yates (Knuth shuffle) y preserva la sincronización
// del cursor aleatorio con la pista actualmente seleccionada.
func (p *Playlist) regenerateShuffle() {
	n := len(p.tracks)
	if n == 0 {
		return
	}
	p.shuffleOrder = make([]int, n)
	for i := range n {
		p.shuffleOrder[i] = i
	}
	// Aplicación del algoritmo Fisher-Yates en orden descendente O(n)
	for i := n - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		p.shuffleOrder[i], p.shuffleOrder[j] = p.shuffleOrder[j], p.shuffleOrder[i]
	}
	p.shuffleIdx = p.findInShuffleOrder(p.current)
	p.shuffleStartIdx = p.shuffleIdx
}

// rebuildShuffleAfterRemove reconstruye el arreglo de permutación aleatoria tras la eliminación
// de una pista, descartando el índice suprimido y decrementando los índices mayores a este
// para mantener la correspondencia con las posiciones reales del arreglo tracks.
func (p *Playlist) rebuildShuffleAfterRemove(removedIndex int) {
	newOrder := make([]int, 0, len(p.shuffleOrder)-1)
	for _, idx := range p.shuffleOrder {
		if idx == removedIndex {
			continue // Excluir el elemento suprimido de la permutación
		}
		if idx > removedIndex {
			idx-- // Desplazar hacia abajo los índices superiores para reflejar el corte del slice
		}
		newOrder = append(newOrder, idx)
	}
	p.shuffleOrder = newOrder
}

// findInShuffleOrder localiza la posición ordinal que ocupa un determinado índice de pista
// dentro de la secuencia permutada shuffleOrder. Devuelve 0 si no se encuentra coincidencia.
func (p *Playlist) findInShuffleOrder(trackIndex int) int {
	for i, idx := range p.shuffleOrder {
		if idx == trackIndex {
			return i
		}
	}
	return 0
}
