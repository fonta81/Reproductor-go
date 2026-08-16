// Package player provides the core functionality for the audio playback application,
// including UI management, audio processing, and state handling.
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

// Mensajes de la aplicación para el modelo de Bubble Tea.
type (
	trackLoadedMsg struct {
		track    Track
		duration time.Duration
	}
	playbackEndedMsg  struct{ sessionID int }
	tickMsg           time.Time
	libraryScannedMsg struct {
		tracks []Track
		dir    string
	}
	// noMusicFoundMsg se envía cuando un escaneo no devuelve pistas; abre el explorador.
	noMusicFoundMsg struct{ dir string }
	errorMsg        struct{ err error }
)

// AppModel es el modelo principal de la aplicación que gestiona el estado del reproductor.
type AppModel struct {
	playlist *Playlist
	Audio    *AudioEngine

	state       PlaybackState
	elapsed     time.Duration
	totalTime   time.Duration
	cursorIndex int
	volumeLevel float64
	lastError   error

	width       int
	height      int
	progressBar progress.Model
	showHelp    bool
	showQueue   bool

	// Características de carga dinámica de directorios
	musicDir        string
	isPickingFolder bool
	browserPath     string
	browserEntries  []browserEntry
	browserCursor   int

	// Quick filter / fuzzy search
	isFiltering     bool
	filterInput     textinput.Model
	filterQuery     string
	filteredIndices []int // indices into playlist.tracks
	filterCursor    int   // position within filteredIndices
	// Suggestions list
	suggestionLimit   int
	suggestionIndices []int // top-N indices from filteredIndices
	suggestionCursor  int   // selected suggestion index
}

// browserEntry representa una entrada en el navegador de archivos.
type browserEntry struct {
	name  string
	isDir bool
}

// NewAppModel crea e inicializa una nueva instancia de AppModel.
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
		// suggestions
		suggestionLimit:   6,
		suggestionIndices: nil,
		suggestionCursor:  0,
	}
}

// Init inicializa la aplicación escaneando la biblioteca y iniciando el tick.
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

				ext := strings.ToLower(filepath.Ext(d.Name()))
				if ext != ".mp3" && ext != ".wav" && ext != ".flac" && ext != ".ogg" {
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

func (m *AppModel) loadBrowserDir(target string) tea.Cmd {
	entries, err := os.ReadDir(target)
	if err != nil {
		return func() tea.Msg { return errorMsg{err} }
	}

	m.browserPath = target

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

		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".mp3" || ext == ".wav" {
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
		m.isPickingFolder = true
		initial := msg.dir
		if initial == "" {
			initial = "."
		}
		cmd := m.loadBrowserDir(initial)
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
		parent := filepath.Dir(m.browserPath)
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
				target = filepath.Join(m.browserPath, selected.name)
			}
		}
		m.isPickingFolder = false
		return m, m.scanLibraryCmd(target)
	}
	return m, nil
}

// handleFilterInput routes keys to the text input and handles filtering navigation/selection.
// handleFilterInput processes key events while the quick-filter input is active.
// Navigation keys move through the suggestion list (if present) or the matched results.
// Enter selects the highlighted match and starts playback. Esc cancels filtering.
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
		// Play the currently selected filtered item (if any)
		if len(m.filteredIndices) > 0 {
			m.cursorIndex = m.filteredIndices[m.filterCursor]
			m.isFiltering = false
			return m.playSelected()
		}
		return m, nil
	case "up", "k":
		// Move up within suggestions or filtered results
		if len(m.suggestionIndices) > 0 {
			m.suggestionCursor = max(0, m.suggestionCursor-1)
			// keep filterCursor aligned to suggestion position
			m.filterCursor = m.suggestionCursor
		} else {
			m.filterCursor = max(0, m.filterCursor-1)
		}
		return m, nil
	case "down", "j":
		// Move down within suggestions or filtered results
		if len(m.suggestionIndices) > 0 {
			m.suggestionCursor = min(len(m.suggestionIndices)-1, m.suggestionCursor+1)
			m.filterCursor = m.suggestionCursor
		} else {
			m.filterCursor = min(len(m.filteredIndices)-1, m.filterCursor+1)
		}
		return m, nil
	default:
		// Delegate key handling to the text input widget, then rebuild matches
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		oldQ := m.filterQuery
		m.filterQuery = m.filterInput.Value()
		m.rebuildFilteredIndices()

		// Reset cursors if the query changed
		if m.filterQuery != oldQ {
			m.suggestionCursor = 0
			m.filterCursor = 0
		}

		// Clamp cursors to valid ranges
		if m.filterCursor >= len(m.filteredIndices) {
			m.filterCursor = max(0, len(m.filteredIndices)-1)
		}
		if m.suggestionCursor >= len(m.suggestionIndices) {
			m.suggestionCursor = max(0, len(m.suggestionIndices)-1)
		}
		return m, cmd
	}
}

// rebuildFilteredIndices computes the list of matching track indices for the
// current filter query, and prepares a small suggestion list. Matches are
// ranked so that exact or prefix substring matches score highest; fuzzy
// subsequence matches receive lower scores.
func (m *AppModel) rebuildFilteredIndices() {
	m.filteredIndices = make([]int, 0)
	m.suggestionIndices = make([]int, 0)
	q := strings.TrimSpace(m.filterQuery)
	if q == "" {
		// No query: include all tracks and set top-N suggestions to the first items
		for i := 0; i < m.playlist.Length(); i++ {
			m.filteredIndices = append(m.filteredIndices, i)
		}
		limit := min(m.suggestionLimit, len(m.filteredIndices))
		for i := 0; i < limit; i++ {
			m.suggestionIndices = append(m.suggestionIndices, m.filteredIndices[i])
		}
		return
	}

	// Build display candidates ("Artist — Title" or Title when Artist missing)
	cands := make([]string, 0, m.playlist.Length())
	for _, t := range m.playlist.tracks {
		if t.Artist == "" {
			cands = append(cands, t.Title)
		} else {
			cands = append(cands, t.Artist+" — "+t.Title)
		}
	}

	type pair struct {
		idx   int
		score int
	}
	qLower := strings.ToLower(q)
	pairs := make([]pair, 0, len(cands))

	// Score each candidate. Prefer substring matches (earlier position and
	// shorter strings), fallback to a weak subsequence/fuzzy score.
	for i := range cands {
		candidate := cands[i]
		candLower := strings.ToLower(candidate)

		// Substring match: prefer earlier positions and shorter candidates
		if pos := strings.Index(candLower, qLower); pos >= 0 {
			score := 100
			// earlier position -> higher score (reduce penalty by position)
			score += max(0, 30-pos*2)
			if strings.HasPrefix(candLower, qLower) {
				score += 50
			}
			// prefer shorter candidates
			score += max(0, 50-len([]rune(candLower)))
			pairs = append(pairs, pair{idx: i, score: score})
			continue
		}

		// Fuzzy subsequence match (weaker signal)
		if isSubsequence(qLower, candLower) {
			score := 30 - len([]rune(candLower))/10
			if score < 1 {
				score = 1
			}
			pairs = append(pairs, pair{idx: i, score: score})
		}
	}

	// Sort matches by descending score and extract indices
	sort.Slice(pairs, func(a, b int) bool { return pairs[a].score > pairs[b].score })
	for _, p := range pairs {
		m.filteredIndices = append(m.filteredIndices, p.idx)
	}

	// Populate top-N suggestions from filtered list
	limit := min(m.suggestionLimit, len(m.filteredIndices))
	for i := 0; i < limit; i++ {
		m.suggestionIndices = append(m.suggestionIndices, m.filteredIndices[i])
	}
}

// isSubsequence reports whether every rune in 'small' appears in order within
// 'big'. It performs a single left-to-right scan and returns true for an empty
// 'small' string (empty query matches everything). This is used as a cheap
// fuzzy matching heuristic.
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

// renderFilterBar builds the UI element shown when quick-filter is active.
// It displays the text input widget and a small summary with the number of
// matches found for the current query.
func (m AppModel) renderFilterBar() string {
	if !m.isFiltering {
		return ""
	}
	count := len(m.filteredIndices)
	summary := fmt.Sprintf(" [%d matches]", count)
	bar := lipgloss.NewStyle().Foreground(cyan).Render("/ ") + lipgloss.NewStyle().Foreground(foreground).Render(m.filterInput.View())
	return lipgloss.JoinHorizontal(lipgloss.Left, bar, lipgloss.NewStyle().Foreground(comment).Render(summary))
}

// renderSuggestions renders the compact list of top suggestions computed from
// the current filtered results. The suggestion list highlights the selected
// suggestion and marks the currently playing track.
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
		row := fmt.Sprintf("%s%d. %-40s [%s]", cursor, idx+1, truncate(t.DisplayName(), 40), t.FormattedDuration())
		style := lipgloss.NewStyle()
		if idx == m.playlist.current {
			style = style.Bold(true).Foreground(pink)
		} else if i == m.suggestionCursor {
			style = style.Foreground(cyan)
		} else {
			style = style.Foreground(foreground)
		}
		b.WriteString("  ")
		b.WriteString(style.Render(row))
		b.WriteString("\n")
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(selection).Padding(0, 1).MarginLeft(2).Render(b.String())
}

func (m AppModel) handleKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.Audio.Close()
		return m, tea.Quit
	case "o", "ctrl+o":
		m.isPickingFolder = true
		initialDir := m.musicDir
		if initialDir == "" {
			initialDir = "."
		}
		cmd := m.loadBrowserDir(initialDir)
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
		// Enter quick filter mode
		m.isFiltering = true
		m.filterInput.SetValue("")
		m.filterQuery = ""
		m.filteredIndices = nil
		m.filterCursor = 0
		return m, nil
	}
	return m, nil
}

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

func (m AppModel) playCurrent() (tea.Model, tea.Cmd) {
	if track, ok := m.playlist.Current(); ok {
		return m, m.loadTrackCmd(track)
	}
	return m, nil
}

func (m AppModel) playNext() (tea.Model, tea.Cmd) {
	if track, ok := m.playlist.Next(); ok {
		m.cursorIndex = m.playlist.current
		return m, m.loadTrackCmd(track)
	}
	m.resetPlayback()
	return m, nil
}

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

func (m AppModel) playSelected() (tea.Model, tea.Cmd) {
	if m.playlist.JumpTo(m.cursorIndex) {
		return m.playCurrent()
	}
	return m, nil
}

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

func (m *AppModel) moveCursor(delta int) {
	m.cursorIndex = max(0, min(m.cursorIndex+delta, m.playlist.Length()-1))
}

func (m *AppModel) adjustVolume(delta float64) {
	m.volumeLevel = max(minVolume, min(maxVolume, m.volumeLevel+delta))
	m.Audio.SetVolume(m.volumeLevel)
}

func (m *AppModel) seekTo(position time.Duration) tea.Cmd {
	// Limitar la posición antes de solicitarla al motor de audio: los
	// decodificadores subyacentes devuelven un error si se les pide una
	// posición negativa o posterior al final de la pista, lo que antes
	// disparaba una pantalla de "CRITICAL ERROR" al rebobinar cerca del
	// inicio o avanzar cerca del final de una canción.
	position = clampDuration(position, 0, m.totalTime)
	if err := m.Audio.Seek(position); err != nil {
		return func() tea.Msg { return errorMsg{err} }
	}
	m.elapsed = position
	return nil
}

func (m *AppModel) seekForward(seconds int) tea.Cmd {
	return m.seekTo(m.elapsed + time.Duration(seconds)*time.Second)
}

func (m *AppModel) seekBackward(seconds int) tea.Cmd {
	return m.seekTo(m.elapsed - time.Duration(seconds)*time.Second)
}

func (m *AppModel) resetPlayback() {
	m.Audio.Stop()
	m.state, m.elapsed, m.totalTime = StateStopped, 0, 0
}

func (m AppModel) View() string {
	if m.lastError != nil {
		return m.renderErrorScreen()
	}

	sections := []string{m.renderHeader(), m.renderNowPlayingPanel(), ""}

	if m.isPickingFolder {
		sections = append(sections, m.renderBrowserPanel())
	} else {
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

func (m AppModel) renderHeader() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(pink).MarginLeft(2).MarginTop(1).Render(appName)
	return title + "  " + lipgloss.NewStyle().Foreground(comment).Render(appSubtitle)
}

func (m AppModel) renderNowPlayingPanel() string {
	track, hasTrack := m.playlist.Current()
	var content strings.Builder

	if hasTrack {
		content.WriteString(m.renderStatusLine(track))
		content.WriteString("\n\n") // Added extra spacing
		content.WriteString(m.renderProgressBar())
		content.WriteString("\n\n") // Added extra spacing
		content.WriteString(m.renderMetadataLine())
	} else {
		content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(red).Render(iconStop + " Sin canciones\n"))
		content.WriteString(lipgloss.NewStyle().Foreground(comment).Render("Coloca archivos .mp3 o .wav en el directorio o presiona 'o' para explorar carpetas."))
	}

	dirDisplay := lipgloss.NewStyle().Foreground(comment).MarginLeft(2).MarginTop(1).Render(iconFolder + " Directorio actual: " + m.musicDir)

	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(purple).Padding(1, 2).Margin(0, 2).Render(content.String()) + "\n" + dirDisplay
}

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

func (m AppModel) renderProgressBar() string {
	percent := 0.0
	if m.totalTime > 0 {
		percent = float64(m.elapsed) / float64(m.totalTime)
	}

	// Apply theme colors to progress bar dynamically
	m.progressBar.FullColor = string(pink)
	m.progressBar.EmptyColor = string(selection)

	bar := m.progressBar.ViewAs(percent)
	timeInfo := fmt.Sprintf(" %s / %s", formatDuration(m.elapsed), formatDuration(m.totalTime))
	return lipgloss.NewStyle().MarginLeft(2).Render(bar) + lipgloss.NewStyle().Foreground(cyan).Render(timeInfo)
}

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

// renderPlaylistPanel returns a textual representation of the upcoming queue.
// When quick-filter is active it shows a windowed view of the filtered results
// centered at the current filter cursor. In normal mode it shows a windowed
// view around the main cursorIndex.
func (m AppModel) renderPlaylistPanel() string {
	if m.playlist.IsEmpty() {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(orange).Render("  " + iconQueue + " Próximamente:"))
	builder.WriteString("\n")

	// When filtering, render the filteredIndices list around filterCursor
	if m.isFiltering {
		if m.filteredIndices == nil || len(m.filteredIndices) == 0 {
			builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("  (No matches)") + "\n")
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

			style := lipgloss.NewStyle()
			if i == m.playlist.current {
				style = style.Bold(true).Foreground(pink)
			} else if idx == m.filterCursor {
				style = style.Foreground(cyan)
			} else {
				style = style.Foreground(foreground)
			}

			row := fmt.Sprintf("%s%d. %-35s [%s]", cursor, i+1, truncate(t.DisplayName(), 35), t.FormattedDuration())
			builder.WriteString("  ")
			builder.WriteString(style.Render(row))
			builder.WriteString("\n")
		}

		if end < len(m.filteredIndices) {
			builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("    " + iconDown + " ...\n"))
		}

		return builder.String()
	}

	// Default (non-filtering) view: same as before
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

		style := lipgloss.NewStyle()
		switch i {
		case m.playlist.current:
			style = style.Bold(true).Foreground(pink)
		case m.cursorIndex:
			style = style.Foreground(cyan)
		default:
			style = style.Foreground(foreground)
		}

		row := fmt.Sprintf("%s%d. %-35s [%s]", cursor, i+1, truncate(t.DisplayName(), 35), t.FormattedDuration())
		builder.WriteString("  ")
		builder.WriteString(style.Render(row))
		builder.WriteString("\n")
	}

	if end < m.playlist.Length() {
		builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("    " + iconDown + " ...\n"))
	}

	return builder.String()
}

func (m AppModel) renderBrowserPanel() string {
	header := lipgloss.NewStyle().Bold(true).Foreground(cyan).Render(iconFolder + " Explorador de Carpetas: " + m.browserPath)

	var builder strings.Builder
	builder.WriteString(header)
	builder.WriteString("\n")
	builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("  ←/Retroceso: Subir | →/Enter: Entrar | Espacio: Confirmar Directorio | Esc: Cancelar"))
	builder.WriteString("\n\n")

	if len(m.browserEntries) == 0 {
		builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("  (Directorio vacío, sin subcarpetas ni pistas .mp3/.wav)\n"))
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

func (m AppModel) renderHelpPanel() string {
	type binding struct{ key, desc string }
	type category struct {
		title    string
		bindings []binding
	}

	categories := []category{
		{iconPlay + " Reproducción", []binding{{"espacio", "Play / Pausa"}, {"n / N", "Sig / Anterior"}, {"> / <", "Adel. / Atrasar"}, {"0", "Reiniciar"}}},
		{iconNav + " Navegación", []binding{{"↑↓ / jk", "Mover cursor"}, {"enter", "Reproducir"}, {"d", "Eliminar de cola"}, {"l", "Ocultar cola"}, {"o", "Explorar carpetas"}, {"ctrl+f", "Buscar / Filtrar"}}},
		{iconAudio + " Audio & Modos", []binding{{"+ / -", "Volumen"}, {"m", "Silenciar"}, {"r", "Repetir"}, {"s", "Aleatorio"}}},
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

func (m AppModel) renderErrorScreen() string {
	if m.lastError == nil {
		return ""
	}
	return lipgloss.NewStyle().Bold(true).Foreground(foreground).Background(red).Padding(1, 2).Render(
		fmt.Sprintf("CRITICAL ERROR:\n\n%v\n\nEl sistema intentará recuperarse en 5 segundos o presiona 'q' para salir.", m.lastError),
	)
}
