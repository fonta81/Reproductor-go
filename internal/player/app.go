// Package player proporciona la interfaz de usuario basada en terminal (TUI) con Bubble Tea,
// la gestión de eventos de teclado, escaneo de biblioteca y control del reproductor.
package player

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Tipos de mensaje internos utilizados por la arquitectura Elm de Bubble Tea.
type (
	// trackLoadedMsg se emite cuando un archivo de audio se decodifica y carga correctamente en el motor.
	trackLoadedMsg struct {
		track    Track
		duration time.Duration
	}
	// playbackEndedMsg se emite al terminar la reproducción del stream; sessionID evita procesar eventos antiguos.
	playbackEndedMsg struct{ sessionID int }
	// tickMsg se emite periódicamente para refrescar el tiempo transcurrido y la barra de progreso en la UI.
	tickMsg time.Time
	// libraryScannedMsg se emite tras completar el escaneo recursivo de un directorio musical.
	libraryScannedMsg struct {
		tracks []Track
		dir    string
	}
	// noMusicFoundMsg se emite cuando un escaneo no encuentra canciones compatibles; activa el explorador de carpetas.
	noMusicFoundMsg struct{ dir string }
	// errorMsg encapsula un error en tiempo de ejecución para mostrarlo temporalmente al usuario.
	errorMsg struct{ err error }
)

// AppModel es el modelo central de Bubble Tea que gestiona el estado del reproductor,
// las interacciones de usuario, la interfaz TUI y la integración con el motor de audio.
type AppModel struct {
	playlist *Playlist    // Lista de reproducción con control secuencial y aleatorio
	Audio    *AudioEngine // Motor de reproducción de bajo nivel

	state       PlaybackState // Estado de reproducción (detenido, reproduciendo, pausado)
	elapsed     time.Duration // Tiempo reproducido de la pista actual
	totalTime   time.Duration // Duración total de la pista actual
	cursorIndex int           // Posición del cursor en la lista de canciones
	volumeLevel float64       // Nivel de ganancia en decibelios (dB)
	lastError   error         // Último error capturado para la pantalla de alerta

	width       int            // Ancho actual del terminal
	height      int            // Alto actual del terminal
	progressBar progress.Model // Componente visual de la barra de progreso
	showHelp    bool           // Indica si se debe mostrar el panel de atajos de teclado
	showQueue   bool           // Indica si se debe mostrar la cola de pistas

	// Explorador interactivo de directorios
	musicDir        string         // Directorio de música actualmente configurado
	isPickingFolder bool           // Indica si el explorador de carpetas está en primer plano
	browserPath     string         // Ruta del directorio explorado en tiempo real
	browserEntries  []browserEntry // Elementos (carpetas y pistas compatibles) en browserPath
	browserCursor   int            // Posición del cursor en el explorador de carpetas

	// Búsqueda y filtrado rápido difuso (fuzzy filter)
	isFiltering     bool            // Indica si el campo de búsqueda rápida está activo
	filterInput     textinput.Model // Componente de entrada de texto interactivo
	filterQuery     string          // Texto ingresado por el usuario para filtrar
	filteredIndices []int           // Índices de playlist.tracks que coinciden con la búsqueda
	filterCursor    int             // Posición del cursor dentro de filteredIndices

	// Ecualizador
	eqActive bool // Indica si el panel de ajuste del ecualizador tiene el foco
	eqCursor int  // Banda del ecualizador seleccionada actualmente

	// Lista de sugerencias destacadas para la búsqueda rápida
	suggestionLimit   int   // Límite máximo de sugerencias simultáneas a mostrar
	suggestionIndices []int // Índices de las mejores sugerencias calculadas
	suggestionCursor  int   // Índice de la sugerencia seleccionada en la lista emergente
}

// browserEntry representa un elemento individual (directorio o archivo de audio) en el explorador.
type browserEntry struct {
	name  string // Nombre del archivo o subdirectorio
	isDir bool   // True si el elemento es un directorio; false si es un archivo de audio
}

// NewAppModel construye e inicializa una nueva instancia de AppModel con valores predeterminados.
func NewAppModel(initialDir string) AppModel {
	bar := progress.New(progress.WithDefaultGradient())
	bar.Width = progressWidth
	bar.ShowPercentage = false

	ti := textinput.New()
	ti.Placeholder = "Buscar título o artista..."
	ti.CharLimit = 200
	ti.Width = 40

	return AppModel{
		playlist:        NewPlaylist(),
		Audio:           NewAudioEngine(),
		state:           StateStopped,
		progressBar:     bar,
		volumeLevel:     0,
		showHelp:        true,
		showQueue:       true,
		musicDir:        initialDir,
		filterInput:     ti,
		isFiltering:     false,
		filterQuery:     "",
		filteredIndices: nil,
		filterCursor:    0,
		// Configuración de sugerencias de búsqueda
		suggestionLimit:   6,
		suggestionIndices: nil,
		suggestionCursor:  0,
	}
}

// Init inicializa la aplicación escaneando la biblioteca y iniciando el temporizador de refresco.
func (m AppModel) Init() tea.Cmd {
	return tea.Batch(m.scanLibraryCmd(m.musicDir), m.tick())
}

// tick genera un comando para actualizar la interfaz periódicamente.
func (m AppModel) tick() tea.Cmd {
	return tea.Tick(time.Millisecond*250, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// scanLibraryCmd escanea un directorio en busca de archivos de audio compatibles de forma recursiva.
func (m AppModel) scanLibraryCmd(targetDir string) tea.Cmd {
	return func() tea.Msg {
		var dirs []string

		if targetDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				home = ""
			}
			dirs = []string{defaultDir, "./songs"}
			if home != "" {
				dirs = append(dirs, filepath.Join(home, "Music"), filepath.Join(home, "Música"))
			}
		} else {
			dirs = []string{targetDir}
		}

		found := make([]Track, 0, 50)
		for _, dir := range dirs {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				continue
			}

			err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil // Ignorar errores de acceso a directorios
				}
				if d.IsDir() {
					return nil
				}

				if !IsSupportedAudio(d.Name()) {
					return nil
				}

				track := ExtractMetadata(path)
				track.ID = fmt.Sprintf("track-%d", len(found))
				found = append(found, track)
				return nil
			})
			if err != nil {
				continue
			}
		}

		if len(found) == 0 {
			return noMusicFoundMsg{dir: targetDir}
		}

		return libraryScannedMsg{tracks: found, dir: targetDir}
	}
}

// loadTrackCmd ejecuta un comando asíncrono que delega en el motor de audio la apertura
// y decodificación de la pista, retornando un mensaje con su duración calculada.
func (m AppModel) loadTrackCmd(track Track) tea.Cmd {
	return func() tea.Msg {
		duration, err := m.Audio.Load(track)
		if err != nil {
			return errorMsg{err}
		}
		track.Duration = duration
		return trackLoadedMsg{track: track, duration: duration}
	}
}

// loadBrowserDir lee el contenido de una ruta en disco, resuelve enlaces simbólicos
// y agrupa las carpetas y archivos de audio compatibles ordenados alfabéticamente.
func (m *AppModel) loadBrowserDir(target string) tea.Cmd {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return func() tea.Msg { return errorMsg{err} }
	}

	entries, err := os.ReadDir(absTarget)
	if err != nil {
		return func() tea.Msg { return errorMsg{err} }
	}

	m.browserPath = absTarget

	var dirs, files []browserEntry
	for _, e := range entries {
		name := e.Name()
		isDir := e.IsDir()

		if e.Type()&os.ModeSymlink != 0 {
			if info, statErr := os.Stat(filepath.Join(target, name)); statErr == nil {
				isDir = info.IsDir()
			} else {
				continue
			}
		}

		if isDir {
			dirs = append(dirs, browserEntry{name: name, isDir: true})
			continue
		}

		if IsSupportedAudio(name) {
			files = append(files, browserEntry{name: name, isDir: false})
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].name) < strings.ToLower(dirs[j].name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].name) < strings.ToLower(files[j].name)
	})

	m.browserEntries = append(dirs, files...)
	m.browserCursor = 0
	return nil
}

// openBrowser activa el explorador de carpetas apuntando a dir (o "." si está vacío).
func (m *AppModel) openBrowser(dir string) tea.Cmd {

	m.isPickingFolder = true
	if dir == "" {
		dir = "."
	}
	return m.loadBrowserDir(dir)
}

// Update procesa los eventos y mensajes del bucle de Bubble Tea (redimensionamiento, escaneos de biblioteca,
// carga de pistas, temporizadores de refresco, fin de reproducción y entrada de teclado en diferentes modos).
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.progressBar.Width = max(int(float64(msg.Width)*0.7), 30)
		return m, nil

	case tea.KeyMsg:
		if m.isPickingFolder {
			return m.handleBrowserInput(msg)
		}
		if m.isFiltering {
			return m.handleFilterInput(msg)
		}
		if m.eqActive {
			return m.handleEQInput(msg)
		}
		return m.handleKeyInput(msg)

	case libraryScannedMsg:
		m.playlist.Clear()
		m.resetPlayback()
		m.musicDir = msg.dir
		m.cursorIndex = 0

		for _, track := range msg.tracks {
			m.playlist.Add(track)
		}
		if !m.playlist.IsEmpty() && m.state == StateStopped {
			m.playlist.current = 0
		}
		if msg.dir != "" {
			// Recordar el directorio para que LoadConfig() lo recupere en el
			// próximo arranque. Antes SaveMusicDir nunca se invocaba, así
			// que la carpeta elegida con el explorador ('o') se perdía al
			// cerrar la aplicación.
			_ = SaveMusicDir(msg.dir)
		}
		return m, nil

	case noMusicFoundMsg:
		// Abrir el navegador para que el usuario seleccione una carpeta con pistas
		m.lastError = nil
		cmd := m.openBrowser(msg.dir)
		return m, cmd

	case trackLoadedMsg:
		return m.handleTrackLoaded(msg)

	case tickMsg:
		if m.state == StatePlaying {
			m.elapsed = min(m.Audio.Position(), m.totalTime)
			return m, m.tick()
		}
		return m, nil

	case playbackEndedMsg:
		if msg.sessionID != m.Audio.sessionID {
			return m, nil
		}
		if m.playlist.isLast() && m.playlist.repeat == RepeatOff {
			m.state, m.elapsed = StateStopped, 0
			m.Audio.Stop()
			return m, nil
		}
		return m.playNext()

	case errorMsg:
		m.lastError = msg.err
		if msg.err != nil {
			// Solo se programa el borrado automático cuando llega un error
			// real. De lo contrario, el propio mensaje de limpieza
			// (errorMsg{nil}) volvía a programar otro temporizador cada 5
			// segundos indefinidamente.
			return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg { return errorMsg{nil} })
		}
		return m, nil
	default:
		return m, nil
	}
}

// handleBrowserInput procesa las pulsaciones de teclado cuando el explorador de carpetas está en primer plano,
// permitiendo navegar directorios, subir de nivel respetando el home del usuario y confirmar la selección.
func (m AppModel) handleBrowserInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.isPickingFolder = false
		return m, nil
	case "up", "k":
		m.browserCursor = max(0, m.browserCursor-1)
	case "down", "j":
		m.browserCursor = min(len(m.browserEntries)-1, m.browserCursor+1)
	case "left", "backspace":
		home, err := os.UserHomeDir()
		if err != nil {
			return m, nil // No es posible restringir navegación sin directorio home
		}

		absPath, err := filepath.Abs(m.browserPath)
		if err != nil {
			return m, nil
		}

		parent := filepath.Dir(absPath)

		// Asegurar que el directorio padre permanezca dentro de home para evitar salir del espacio de usuario
		if !strings.HasPrefix(parent, home) {
			return m, nil // No permitir salir de la carpeta personal
		}

		cmd := m.loadBrowserDir(parent)
		return m, cmd
	case "right", "enter":
		if len(m.browserEntries) > 0 {
			selected := m.browserEntries[m.browserCursor]
			if selected.isDir {
				newPath := filepath.Join(m.browserPath, selected.name)
				cmd := m.loadBrowserDir(newPath)
				return m, cmd
			}
		}
	case " ":
		target := m.browserPath
		if len(m.browserEntries) > 0 {
			selected := m.browserEntries[m.browserCursor]
			if selected.isDir {
				target := filepath.Join(m.browserPath, selected.name)
				home, err := os.UserHomeDir()
				if err == nil && !strings.HasPrefix(target, home) {
					return m, nil // Evitar seleccionar carpetas fuera del directorio personal
				}
				m.isPickingFolder = false
				return m, m.scanLibraryCmd(target)
			}
		}
		m.isPickingFolder = false
		return m, m.scanLibraryCmd(target)
	}
	return m, nil
}

// handleFilterInput procesa los eventos de teclado cuando el modo de búsqueda rápida está activo.
// Permite navegar por la lista de sugerencias o resultados coincidentes, selecciona y reproduce
// la pista destacada con Enter, o cancela la búsqueda restaurando la vista normal con Esc.
func (m AppModel) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.isFiltering = false
		m.filterInput.SetValue("")
		m.filterQuery = ""
		m.filteredIndices = nil
		m.filterCursor = 0
		return m, nil
	case "enter":
		// Reproducir el elemento filtrado seleccionado actualmente (si existe alguno)
		if len(m.filteredIndices) > 0 {
			m.cursorIndex = m.filteredIndices[m.filterCursor]
			m.isFiltering = false
			return m.playSelected()
		}
		return m, nil
	case "up", "k":
		// Mover hacia arriba en las sugerencias o resultados filtrados
		if len(m.suggestionIndices) > 0 {
			m.suggestionCursor = max(0, m.suggestionCursor-1)
			// Mantener filterCursor alineado con la posición de la sugerencia
			m.filterCursor = m.suggestionCursor
		} else {
			m.filterCursor = max(0, m.filterCursor-1)
		}
		return m, nil
	case "down", "j":
		// Mover hacia abajo en las sugerencias o resultados filtrados
		if len(m.suggestionIndices) > 0 {
			m.suggestionCursor = min(len(m.suggestionIndices)-1, m.suggestionCursor+1)
			m.filterCursor = m.suggestionCursor
		} else {
			m.filterCursor = min(len(m.filteredIndices)-1, m.filterCursor+1)
		}
		return m, nil
	default:
		// Delegar el procesamiento al componente de texto y recalcular coincidencias
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		oldQ := m.filterQuery
		m.filterQuery = m.filterInput.Value()
		m.rebuildFilteredIndices()

		// Reiniciar cursores si la consulta de búsqueda cambió
		if m.filterQuery != oldQ {
			m.suggestionCursor = 0
			m.filterCursor = 0
		}

		// Ajustar cursores dentro de los rangos válidos
		if m.filterCursor >= len(m.filteredIndices) {
			m.filterCursor = max(0, len(m.filteredIndices)-1)
		}
		if m.suggestionCursor >= len(m.suggestionIndices) {
			m.suggestionCursor = max(0, len(m.suggestionIndices)-1)
		}
		return m, cmd
	}
}

// rebuildFilteredIndices calcula la lista de pistas que coinciden con el texto de búsqueda actual
// y selecciona las mejores sugerencias. Las coincidencias se ordenan ponderando mayor puntuación
// a coincidencias exactas o de prefijo, penalizando posiciones lejanas y aplicando coincidencia
// difusa (fuzzy subsequence) como respaldo.
func (m *AppModel) rebuildFilteredIndices() {
	m.filteredIndices = make([]int, 0)
	m.suggestionIndices = make([]int, 0)
	q := strings.TrimSpace(m.filterQuery)
	if q == "" {
		// Sin consulta: incluir todas las pistas y fijar las primeras N como sugerencias
		for i := 0; i < m.playlist.Length(); i++ {
			m.filteredIndices = append(m.filteredIndices, i)
		}
		limit := min(m.suggestionLimit, len(m.filteredIndices))
		for i := 0; i < limit; i++ {
			m.suggestionIndices = append(m.suggestionIndices, m.filteredIndices[i])
		}
		return
	}

	// Construir candidatos de visualización usando DisplayName() para unificar la representación
	cands := make([]string, 0, m.playlist.Length())
	for _, t := range m.playlist.tracks {
		cands = append(cands, t.DisplayName())
	}

	type pair struct {
		idx   int
		score int
	}
	qLower := strings.ToLower(q)
	pairs := make([]pair, 0, len(cands))

	// Calcular puntaje para cada candidato: prioriza coincidencias directas y cadenas cortas,
	// con retroceso a coincidencia difusa (subsequence)
	for i := range cands {
		candidate := cands[i]
		candLower := strings.ToLower(candidate)

		// Coincidencia por subcadena: premia aparición temprana y menor longitud de cadena
		if pos := strings.Index(candLower, qLower); pos >= 0 {
			score := 100
			score += max(0, 30-pos*2)
			if strings.HasPrefix(candLower, qLower) {
				score += 50
			}
			score += max(0, 50-len([]rune(candLower)))
			pairs = append(pairs, pair{idx: i, score: score})
			continue
		}

		// Coincidencia difusa de subsecuencia (señal secundaria)
		if isSubsequence(qLower, candLower) {
			score := 30 - len([]rune(candLower))/10
			if score < 1 {
				score = 1
			}
			pairs = append(pairs, pair{idx: i, score: score})
		}
	}

	// Ordenar las coincidencias de mayor a menor puntuación y extraer índices
	sort.Slice(pairs, func(a, b int) bool { return pairs[a].score > pairs[b].score })
	for _, p := range pairs {
		m.filteredIndices = append(m.filteredIndices, p.idx)
	}

	// Extraer las primeras N mejores coincidencias para el panel de sugerencias
	limit := min(m.suggestionLimit, len(m.filteredIndices))
	for i := 0; i < limit; i++ {
		m.suggestionIndices = append(m.suggestionIndices, m.filteredIndices[i])
	}
}

// isSubsequence comprueba si todos los caracteres (runas) de 'small' aparecen en orden secuencial
// dentro de 'big'. Se utiliza como heurística ligera de búsqueda difusa (fuzzy search).
func isSubsequence(small, big string) bool {
	if small == "" {
		return true
	}
	rSmall := []rune(small)
	rBig := []rune(big)
	j := 0
	for _, rb := range rBig {
		if rSmall[j] == rb {
			j++
			if j == len(rSmall) {
				return true
			}
		}
	}
	return false
}

// renderFilterBar genera el componente visual de la barra de búsqueda rápida, mostrando
// el campo de texto interactivo junto al conteo de canciones coincidentes.
func (m AppModel) renderFilterBar() string {
	if !m.isFiltering {
		return ""
	}
	count := len(m.filteredIndices)
	summary := fmt.Sprintf(" [%d matches]", count)
	bar := lipgloss.NewStyle().Foreground(cyan).Render("/ ") + lipgloss.NewStyle().Foreground(foreground).Render(m.filterInput.View())
	return lipgloss.JoinHorizontal(lipgloss.Left, bar, lipgloss.NewStyle().Foreground(comment).Render(summary))
}

// renderTrackRow construye y pinta una fila de pista con cursor, número, título truncado,
// duración y el estilo adecuado según si es la pista actual o la destacada.
func renderTrackRow(cursor string, index int, track Track, isCurrent, isHighlighted bool, titleWidth int) string {
	style := lipgloss.NewStyle()
	if isCurrent {
		style = style.Bold(true).Foreground(pink)
	} else if isHighlighted {
		style = style.Foreground(cyan)
	} else {
		style = style.Foreground(foreground)
	}
	row := fmt.Sprintf("%s%d. %-*s [%s]", cursor, index+1, titleWidth, truncate(track.DisplayName(), titleWidth), track.FormattedDuration())
	return "  " + style.Render(row) + "\n"
}

// renderSuggestions renderiza la caja emergente con la lista compacta de mejores sugerencias
// calculadas a partir del filtro de búsqueda actual.
func (m AppModel) renderSuggestions() string {

	if !m.isFiltering || len(m.suggestionIndices) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(comment).Render(" Sugerencias:"))
	b.WriteString("\n")
	limit := len(m.suggestionIndices)
	for i := 0; i < limit; i++ {
		idx := m.suggestionIndices[i]
		t := m.playlist.tracks[idx]
		cursor := "   "
		if i == m.suggestionCursor {
			cursor = "→  "
		}
		b.WriteString(renderTrackRow(cursor, idx, t, idx == m.playlist.current, i == m.suggestionCursor, 40))
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(selection).Padding(0, 1).MarginLeft(2).Render(b.String())
}

// handleKeyInput procesa los atajos de teclado estándar en la vista principal del reproductor
// para controlar la reproducción, volumen, modos (shuffle/repeat), búsqueda y navegación por la cola.
func (m AppModel) handleKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		if m.Audio.HasEQ() {
			_ = SaveEQ(m.Audio.EQEnabled(), m.Audio.GetEQBands())
		}
		m.Audio.Close()
		return m, tea.Quit
	case "o", "ctrl+o":
		cmd := m.openBrowser(m.musicDir)
		return m, cmd
	case " ":
		return m.togglePlayback()
	case "n":
		return m.playNext()
	case "N":
		return m.playPrevious()
	case "enter":
		return m.playSelected()
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "d":
		return m.removeSelected()
	case "+", "=":
		m.adjustVolume(volumeStep)
	case "-":
		m.adjustVolume(-volumeStep)
	case "m":
		m.Audio.ToggleMute()
	case "0":
		return m, m.seekTo(0)
	case ".", ">":
		return m, m.seekForward(seekSeconds)
	case ",", "<":
		return m, m.seekBackward(seekSeconds)
	case "s":
		m.playlist.ToggleShuffle()
	case "r":
		m.playlist.repeat = (m.playlist.repeat + 1) % 3
	case "l":
		m.showQueue = !m.showQueue
	case "h", "?":
		m.showHelp = !m.showHelp
	case "ctrl+f":
		// Activar modo de búsqueda rápida
		m.isFiltering = true
		m.filterInput.SetValue("")
		m.filterQuery = ""
		m.filteredIndices = nil
		m.filterCursor = 0
		return m, nil
	case "e":
		// Abrir/cerrar el panel del ecualizador
		m.eqActive = !m.eqActive
	}
	return m, nil
}

// handleEQInput procesa las teclas mientras el panel del ecualizador tiene el foco:
// izquierda/derecha eligen banda, arriba/abajo ajustan dB, 0 resetea la banda,
// x activa/desactiva el EQ y Esc cierra el panel.
func (m AppModel) handleEQInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	bands := m.Audio.GetEQBands()
	switch msg.String() {
	case "esc", "e":
		m.eqActive = false
		if m.Audio.HasEQ() {
			_ = SaveEQ(m.Audio.EQEnabled(), bands)
		}
	case "left", "h":
		m.eqCursor = max(0, m.eqCursor-1)
	case "right", "l":
		m.eqCursor = min(eqBandCount-1, m.eqCursor+1)
	case "up", "k":
		m.Audio.SetEQBand(m.eqCursor, bands[m.eqCursor]+eqGainStep)
	case "down", "j":
		m.Audio.SetEQBand(m.eqCursor, bands[m.eqCursor]-eqGainStep)
	case "0":
		m.Audio.SetEQBand(m.eqCursor, 0)
	case "x":
		m.Audio.ToggleEQ()
	}
	return m, nil
}

// handleTrackLoaded inicia la reproducción al recibir trackLoadedMsg, incrementa el sessionID
// para invalidar eventos de pistas anteriores y programa una goroutine que aguarda el final de la pista.
func (m AppModel) handleTrackLoaded(msg trackLoadedMsg) (tea.Model, tea.Cmd) {
	m.Audio.sessionID++
	sessionID := m.Audio.sessionID
	m.Audio.cancelChan = make(chan struct{})
	cancelChan := m.Audio.cancelChan

	done := m.Audio.Play()
	m.state = StatePlaying
	m.totalTime = max(msg.duration, defaultDuration)

	m.playlist.tracks[m.playlist.current].Duration = msg.duration
	m.elapsed = 0

	waitCmd := func() tea.Msg {
		select {
		case <-done:
			return playbackEndedMsg{sessionID: sessionID}
		case <-cancelChan:
			return nil
		}
	}
	return m, tea.Batch(m.tick(), waitCmd)
}

// togglePlayback conmuta entre reproducir y pausar según el estado actual del reproductor.
func (m AppModel) togglePlayback() (tea.Model, tea.Cmd) {
	switch m.state {
	case StatePlaying:
		m.Audio.Pause()
		m.state = StatePaused
	case StatePaused:
		m.Audio.Resume()
		m.state = StatePlaying
		return m, m.tick()
	case StateStopped:
		return m.playCurrent()
	}
	return m, nil
}

// playCurrent carga y reproduce la pista actualmente activa en la lista.
func (m AppModel) playCurrent() (tea.Model, tea.Cmd) {
	if track, ok := m.playlist.Current(); ok {
		return m, m.loadTrackCmd(track)
	}
	return m, nil
}

// playNext avanza a la siguiente canción de la lista y la reproduce.
func (m AppModel) playNext() (tea.Model, tea.Cmd) {
	if track, ok := m.playlist.Next(); ok {
		m.cursorIndex = m.playlist.current
		return m, m.loadTrackCmd(track)
	}
	m.resetPlayback()
	return m, nil
}

// playPrevious reinicia la pista si han transcurrido más de 3 segundos, o retrocede a la canción previa.
func (m AppModel) playPrevious() (tea.Model, tea.Cmd) {
	if m.elapsed > 3*time.Second {
		cmd := m.seekTo(0)
		return m, cmd
	}
	if track, ok := m.playlist.Previous(); ok {
		m.cursorIndex = m.playlist.current
		return m, m.loadTrackCmd(track)
	}
	return m, nil
}

// playSelected reproduce la pista sobre la que está posicionado el cursor.
func (m AppModel) playSelected() (tea.Model, tea.Cmd) {
	if m.playlist.JumpTo(m.cursorIndex) {
		return m.playCurrent()
	}
	return m, nil
}

// removeSelected elimina de la lista la pista bajo el cursor, gestionando la continuidad
// de la reproducción si correspondía a la canción en curso.
func (m AppModel) removeSelected() (tea.Model, tea.Cmd) {
	if !m.playlist.isValidIndex(m.cursorIndex) {
		return m, nil
	}

	wasPlaying := m.cursorIndex == m.playlist.current
	m.playlist.Remove(m.cursorIndex)

	if m.cursorIndex >= m.playlist.Length() && m.cursorIndex > 0 {
		m.cursorIndex--
	}
	if wasPlaying {
		m.resetPlayback()
		if !m.playlist.IsEmpty() {
			return m.playCurrent()
		}
	}
	return m, nil
}

// moveCursor desplaza el cursor de navegación vertical dentro de los límites válidos de la lista.
func (m *AppModel) moveCursor(delta int) {
	m.cursorIndex = max(0, min(m.cursorIndex+delta, m.playlist.Length()-1))
}

// adjustVolume modifica la ganancia de salida en pasos de decibelios respetando los topes configurados.
func (m *AppModel) adjustVolume(delta float64) {
	m.volumeLevel = max(minVolume, min(maxVolume, m.volumeLevel+delta))
	m.Audio.SetVolume(m.volumeLevel)
}

// seekTo desplaza la reproducción a un punto temporal concreto, limitándolo entre 0 y la duración total
// para evitar errores críticos de decodificación al rebasar los extremos del archivo.
func (m *AppModel) seekTo(position time.Duration) tea.Cmd {
	position = clampDuration(position, 0, m.totalTime)
	if err := m.Audio.Seek(position); err != nil {
		return func() tea.Msg { return errorMsg{err} }
	}
	m.elapsed = position
	return nil
}

// seekForward adelanta la reproducción en el número de segundos especificado.
func (m *AppModel) seekForward(seconds int) tea.Cmd {
	return m.seekTo(m.elapsed + time.Duration(seconds)*time.Second)
}

// seekBackward retrocede la reproducción en el número de segundos especificado.
func (m *AppModel) seekBackward(seconds int) tea.Cmd {
	return m.seekTo(m.elapsed - time.Duration(seconds)*time.Second)
}

// resetPlayback detiene la salida de audio y restablece a cero los tiempos y estados de reproducción.
func (m *AppModel) resetPlayback() {
	m.Audio.Stop()
	m.state, m.elapsed, m.totalTime = StateStopped, 0, 0
}

// View compone y renderiza la interfaz visual completa de la pantalla del reproductor
// reuniendo el encabezado, tarjeta de estado, explorador o cola de canciones y el panel de ayuda.
func (m AppModel) View() string {
	if m.lastError != nil {
		return m.renderErrorScreen()
	}

	sections := []string{m.renderHeader(), m.renderNowPlayingPanel(), ""}

	if m.isPickingFolder {
		sections = append(sections, m.renderBrowserPanel())
	} else {
		if m.eqActive {
			sections = append(sections, m.renderEQPanel())
		}
		if m.showQueue {
			if m.isFiltering {
				sections = append(sections, m.renderFilterBar())
				sections = append(sections, m.renderSuggestions())
			}
			sections = append(sections, m.renderPlaylistPanel())
		}
		if m.showHelp {
			sections = append(sections, "", m.renderHelpPanel())
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader construye el título de la aplicación y su subtítulo con estilos de Lipgloss.
func (m AppModel) renderHeader() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(pink).MarginLeft(2).MarginTop(1).Render(appName)
	return title + "  " + lipgloss.NewStyle().Foreground(comment).Render(appSubtitle)
}

// renderNowPlayingPanel construye la tarjeta central que muestra la pista activa,
// la barra de progreso, los metadatos de audio y la ruta del directorio actual.
func (m AppModel) renderNowPlayingPanel() string {
	track, hasTrack := m.playlist.Current()
	var content strings.Builder

	if hasTrack {
		content.WriteString(m.renderStatusLine(track))
		content.WriteString("\n\n")
		content.WriteString(m.renderProgressBar())
		content.WriteString("\n\n")
		content.WriteString(m.renderMetadataLine())
	} else {
		content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(red).Render(iconStop + " Sin canciones\n"))
		content.WriteString(lipgloss.NewStyle().Foreground(comment).Render("Coloca archivos .mp3 o .wav en el directorio o presiona 'o' para explorar carpetas."))
	}

	dirDisplay := lipgloss.NewStyle().Foreground(comment).MarginLeft(2).MarginTop(1).Render(iconFolder + " Directorio actual: " + m.musicDir)

	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(purple).Padding(1, 2).Margin(0, 2).Render(content.String()) + "\n" + dirDisplay
}

// renderStatusLine devuelve la línea con el icono de estado coloreado (reproduciendo, pausado, detenido)
// y el nombre de la pista activa formateado.
func (m AppModel) renderStatusLine(track Track) string {
	var icon string
	var style lipgloss.Style

	switch m.state {
	case StatePlaying:
		icon, style = iconPlay, lipgloss.NewStyle().Bold(true).Foreground(green)
	case StatePaused:
		icon, style = iconPause, lipgloss.NewStyle().Bold(true).Foreground(yellow)
	default:
		icon, style = iconStop, lipgloss.NewStyle().Bold(true).Foreground(red)
	}
	return lipgloss.NewStyle().MarginLeft(2).Render(style.Render(icon) + " " + style.Render(m.state.Label()) + "  " + track.DisplayName())
}

// renderProgressBar dibuja la barra de progreso proporcional al tiempo transcurrido
// junto a los indicadores numéricos de tiempo ("MM:SS / MM:SS").
func (m AppModel) renderProgressBar() string {
	percent := 0.0
	if m.totalTime > 0 {
		percent = float64(m.elapsed) / float64(m.totalTime)
	}

	// Asignar colores de tema a la barra de progreso
	m.progressBar.FullColor = string(pink)
	m.progressBar.EmptyColor = string(selection)

	bar := m.progressBar.ViewAs(percent)
	timeInfo := fmt.Sprintf(" %s / %s", formatDuration(m.elapsed), formatDuration(m.totalTime))
	return lipgloss.NewStyle().MarginLeft(2).Render(bar) + lipgloss.NewStyle().Foreground(cyan).Render(timeInfo)
}

// renderMetadataLine genera la fila de información técnica: barra de volumen,
// número de pista en la cola, indicador de orden aleatorio e icono de repetición.
func (m AppModel) renderMetadataLine() string {
	queueDisplay := "0/0"
	if m.playlist.Length() > 0 {
		queueDisplay = fmt.Sprintf("%d/%d", m.playlist.current+1, m.playlist.Length())
	}

	shuffleIcon := ""
	if m.playlist.shuffle {
		shuffleIcon = lipgloss.NewStyle().Foreground(purple).Render(" " + iconShuffle + " ")
	}

	volBar := renderVolumeBar(m.volumeLevel, minVolume, maxVolume, m.Audio.IsMuted())
	return lipgloss.NewStyle().Foreground(comment).MarginLeft(2).Render(
		fmt.Sprintf("%s  |  %s %s  |%s| %s ", volBar, iconQueue, queueDisplay, shuffleIcon, m.playlist.repeat.Icon()),
	)
}

// renderEQPanel dibuja el panel del ecualizador con una columna por banda,
// barras proporcionales a la ganancia en dB y la banda activa resaltada.
func (m AppModel) renderEQPanel() string {
	bands := m.Audio.GetEQBands()
	labels := []string{" 60Hz", "250Hz", "  1kHz", "  4kHz", " 10kHz"}
	levels := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

	var b strings.Builder
	status := lipgloss.NewStyle().Foreground(green).Render("ON")
	if !m.Audio.EQEnabled() {
		status = lipgloss.NewStyle().Foreground(red).Render("OFF (bypass)")
	}
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(orange).Render("  Ecualizador ") + status + "\n\n")

	for i := 0; i < eqBandCount; i++ {
		g := bands[i]
		filled := int((g - eqGainMin) / (eqGainMax - eqGainMin) * float64(len(levels)-1))
		label := labels[i]
		style := lipgloss.NewStyle().Foreground(foreground)
		barStyle := lipgloss.NewStyle().Foreground(cyan)
		if i == m.eqCursor {
			label = ">" + label[1:]
			style = lipgloss.NewStyle().Bold(true).Foreground(pink)
			barStyle = lipgloss.NewStyle().Foreground(pink)
		}
		bar := ""
		for j := 0; j < len(levels); j++ {
			if j <= filled {
				bar += levels[j]
			} else {
				bar += " "
			}
		}
		b.WriteString(style.Render(label) + " " + barStyle.Render(bar) + style.Render(fmt.Sprintf(" %+5.0f dB\n", g)))
	}
	b.WriteString(lipgloss.NewStyle().Foreground(comment).Render("\n←/→ banda | ↑/↓ ±1dB | 0 reset | x on/off | Esc cerrar"))

	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(purple).Padding(0, 2).MarginLeft(2).Render(b.String())
}

// renderPlaylistPanel devuelve una representación en lista de la cola de canciones.
// Si el filtro de búsqueda está activo, muestra una ventana centrada en el cursor de búsqueda;
// en caso contrario, muestra una ventana alrededor del cursor de la lista de reproducción.
func (m AppModel) renderPlaylistPanel() string {
	if m.playlist.IsEmpty() {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(orange).Render("  " + iconQueue + " Próximamente:"))
	builder.WriteString("\n")

	// Vista con filtro de búsqueda activo
	if m.isFiltering {
		if m.filteredIndices == nil || len(m.filteredIndices) == 0 {
			builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("  (No hay coincidencias)") + "\n")
			return builder.String()
		}

		start := max(0, m.filterCursor-3)
		end := min(len(m.filteredIndices), start+7)

		if start > 0 {
			builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("    " + iconUp + " ...\n"))
		}

		for idx := start; idx < end; idx++ {
			i := m.filteredIndices[idx]
			t := m.playlist.tracks[i]
			cursor := "   "
			if idx == m.filterCursor {
				cursor = "→  "
			}
			builder.WriteString(renderTrackRow(cursor, i, t, i == m.playlist.current, idx == m.filterCursor, 35))
		}

		if end < len(m.filteredIndices) {
			builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("    " + iconDown + " ...\n"))
		}

		return builder.String()
	}

	// Vista secuencial estándar de la cola de pistas
	start := max(0, m.cursorIndex-3)
	end := min(m.playlist.Length(), start+7)

	if start > 0 {
		builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("    " + iconUp + " ...\n"))
	}

	for i := start; i < end; i++ {
		t := m.playlist.tracks[i]
		cursor := "   "
		if i == m.cursorIndex {
			cursor = "→  "
		}
		builder.WriteString(renderTrackRow(cursor, i, t, i == m.playlist.current, i == m.cursorIndex, 35))
	}

	if end < m.playlist.Length() {
		builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("    " + iconDown + " ...\n"))
	}

	return builder.String()
}

// renderBrowserPanel dibuja el panel interactivo del explorador de carpetas,
// mostrando los archivos de audio y subdirectorios para su selección.
func (m AppModel) renderBrowserPanel() string {
	header := lipgloss.NewStyle().Bold(true).Foreground(cyan).Render(iconFolder + " Explorador de Carpetas: " + m.browserPath)

	var builder strings.Builder
	builder.WriteString(header)
	builder.WriteString("\n")
	builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("  ←/Retroceso: Subir | →/Enter: Entrar | Espacio: Confirmar Directorio | Esc: Cancelar"))
	builder.WriteString("\n\n")

	if len(m.browserEntries) == 0 {
		builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("  (Directorio vacío, sin subcarpetas ni pistas de audio)\n"))
	}

	start := max(0, m.browserCursor-5)
	end := min(len(m.browserEntries), start+10)

	for i := start; i < end; i++ {
		entry := m.browserEntries[i]
		cursor := "  "

		icon := iconFolder
		style := lipgloss.NewStyle().Foreground(foreground)
		if !entry.isDir {
			icon = iconAudio
			style = style.Foreground(comment)
		}

		if i == m.browserCursor {
			cursor = "→ "
			style = style.Foreground(pink).Bold(true)
		}

		builder.WriteString(style.Render(cursor + icon + " " + entry.name))
		builder.WriteString("\n")
	}

	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cyan).Padding(1, 2).MarginLeft(2).Render(builder.String())
}

// renderHelpPanel dibuja el catálogo de atajos de teclado agrupados temáticamente
// (Reproducción, Navegación, Audio y Sistema), adaptando el número de columnas al ancho del terminal.
func (m AppModel) renderHelpPanel() string {
	type binding struct{ key, desc string }
	type category struct {
		title    string
		bindings []binding
	}

	categories := []category{
		{iconPlay + " Reproducción", []binding{{"espacio", "Play / Pausa"}, {"n / N", "Sig / Anterior"}, {"> / <", "Adel. / Atrasar"}, {"0", "Reiniciar"}}},
		{iconNav + " Navegación", []binding{{"↑↓ / jk", "Mover cursor"}, {"enter", "Reproducir"}, {"d", "Eliminar de cola"}, {"l", "Ocultar cola"}, {"o", "Explorar carpetas"}, {"ctrl+f", "Buscar / Filtrar"}}},
		{iconAudio + " Audio & Modos", []binding{{"+ / -", "Volumen"}, {"m", "Silenciar"}, {"r", "Repetir"}, {"s", "Aleatorio"}, {"e", "Ecualizador"}}},
		{iconSystem + " Sistema", []binding{{"h / ?", "Ocultar ayuda"}, {"q", "Salir"}}},
	}

	var blocks []string
	for _, cat := range categories {
		var lines []string
		lines = append(lines, helpCategoryStyle.Render(cat.title))
		for _, b := range cat.bindings {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, helpKeyStyle.Render(b.key), helpDescStyle.Render(b.desc)))
		}
		blocks = append(blocks, lipgloss.JoinVertical(lipgloss.Left, lines...))
	}

	var rows []string
	colsPerRow := 1
	if m.width > 120 {
		colsPerRow = 4
	} else if m.width > 75 {
		colsPerRow = 2
	}

	for i := 0; i < len(blocks); i += colsPerRow {
		end := min(i+colsPerRow, len(blocks))
		var rowBlocks []string
		for _, block := range blocks[i:end] {
			rowBlocks = append(rowBlocks, lipgloss.NewStyle().Width(32).Render(block))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rowBlocks...))
		if end < len(blocks) {
			rows = append(rows, "")
		}
	}

	return helpContainerStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// renderErrorScreen construye la vista de alerta crítica con fondo rojo en caso de errores graves.
func (m AppModel) renderErrorScreen() string {
	if m.lastError == nil {
		return ""
	}
	return lipgloss.NewStyle().Bold(true).Foreground(foreground).Background(red).Padding(1, 2).Render(
		fmt.Sprintf("CRITICAL ERROR:\n\n%v\n\nEl sistema intentará recuperarse en 5 segundos o presiona 'q' para salir.", m.lastError),
	)
}
