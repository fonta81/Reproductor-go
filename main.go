package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
)

// ═══════════════════════════════════════════════════════════════════════════════
// CONFIGURACIÓN Y CONSTANTES
// ═══════════════════════════════════════════════════════════════════════════════

const (
	appName         = "🎵 GoPlayer"
	appSubtitle     = "Reproductor de música TUI"
	defaultDir      = "./music"
	seekSeconds     = 10
	volumeStep      = 0.5
	maxVolume       = 5.0
	minVolume       = -5.0
	progressWidth   = 50
	defaultDuration = 3 * time.Minute // fallback cuando no hay metadata

	// NUEVO: Tasa de muestreo estándar para evitar aceleración (Chipmunk effect)
	standardSampleRate = beep.SampleRate(44100)
)

// Paleta de colores: tema Dracula optimizado para legibilidad en terminal[cite: 1]
var (
	pink       = lipgloss.Color("#FF79C6")
	cyan       = lipgloss.Color("#8BE9FD")
	green      = lipgloss.Color("#50FA7B")
	yellow     = lipgloss.Color("#F1FA8C")
	orange     = lipgloss.Color("#FFB86C")
	red        = lipgloss.Color("#FF5555")
	purple     = lipgloss.Color("#BD93F9")
	foreground = lipgloss.Color("#F8F8F2")
	comment    = lipgloss.Color("#6272A4")
	selection  = lipgloss.Color("#44475A")
	background = lipgloss.Color("#282A36")
)

// ═══════════════════════════════════════════════════════════════════════════════
// DOMINIO: MODELOS DE DATOS[cite: 1]
// ═══════════════════════════════════════════════════════════════════════════════

type PlaybackState int

const (
	StateStopped PlaybackState = iota
	StatePlaying
	StatePaused
)

func (s PlaybackState) Icon() string {
	switch s {
	case StatePlaying:
		return "▶"
	case StatePaused:
		return "⏸"
	default:
		return "⏹"
	}
}

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

type RepeatMode int

const (
	RepeatOff RepeatMode = iota
	RepeatOne
	RepeatAll
)

func (r RepeatMode) Icon() string {
	switch r {
	case RepeatOne:
		return "🔂"
	case RepeatAll:
		return "🔁"
	default:
		return "➡"
	}
}

func (r RepeatMode) Label() string {
	switch r {
	case RepeatOne:
		return "Repetir una"
	case RepeatAll:
		return "Repetir todo"
	default:
		return "Sin repetir"
	}
}

type Track struct {
	ID       string
	Title    string
	Artist   string
	Album    string
	Duration time.Duration
	Path     string
}

func (t Track) DisplayName() string {
	if t.Artist == "" {
		return t.Title
	}
	return fmt.Sprintf("%s — %s", t.Artist, t.Title)
}

func (t Track) FormattedDuration() string {
	if t.Duration <= 0 {
		return "?:??"
	}
	return formatDuration(t.Duration)
}

// ═══════════════════════════════════════════════════════════════════════════════
// DOMINIO: GESTIÓN DE LA LISTA DE REPRODUCCIÓN[cite: 1]
// ═══════════════════════════════════════════════════════════════════════════════

type Playlist struct {
	tracks  []Track
	current int
	shuffle bool
	repeat  RepeatMode
}

func NewPlaylist() *Playlist {
	return &Playlist{
		tracks:  make([]Track, 0),
		current: -1,
	}
}

func (p *Playlist) Add(track Track) {
	p.tracks = append(p.tracks, track)
	if p.current == -1 {
		p.current = 0
	}
}

func (p *Playlist) Remove(index int) {
	if !p.isValidIndex(index) {
		return
	}
	p.tracks = append(p.tracks[:index], p.tracks[index+1:]...)

	if p.current >= len(p.tracks) {
		p.current = len(p.tracks) - 1
	}
	if len(p.tracks) == 0 {
		p.current = -1
	}
}

func (p *Playlist) Current() (Track, bool) {
	if !p.isValidIndex(p.current) {
		return Track{}, false
	}
	return p.tracks[p.current], true
}

func (p *Playlist) Next() (Track, bool) {
	if len(p.tracks) == 0 {
		return Track{}, false
	}

	switch p.repeat {
	case RepeatOne:
		return p.tracks[p.current], true
	case RepeatAll:
		p.current = (p.current + 1) % len(p.tracks)
		return p.tracks[p.current], true
	default:
		if p.isLast() {
			return Track{}, false
		}
		p.current++
		return p.tracks[p.current], true
	}
}

func (p *Playlist) Previous() (Track, bool) {
	if len(p.tracks) == 0 || p.current <= 0 {
		return Track{}, false
	}
	p.current--
	return p.tracks[p.current], true
}

func (p *Playlist) JumpTo(index int) bool {
	if !p.isValidIndex(index) {
		return false
	}
	p.current = index
	return true
}

func (p *Playlist) Length() int {
	return len(p.tracks)
}

func (p *Playlist) IsEmpty() bool {
	return len(p.tracks) == 0
}

func (p *Playlist) isLast() bool {
	if p.repeat != RepeatOff {
		return false
	}
	return p.current >= len(p.tracks)-1
}

func (p *Playlist) isValidIndex(index int) bool {
	return index >= 0 && index < len(p.tracks)
}

// ═══════════════════════════════════════════════════════════════════════════════
// INFRAESTRUCTURA: MOTOR DE AUDIO
// ═══════════════════════════════════════════════════════════════════════════════

type AudioEngine struct {
	streamer   beep.StreamSeekCloser
	ctrl       *beep.Ctrl
	volume     *effects.Volume
	format     beep.Format
	isInit     bool
	sessionID  int
	cancelChan chan struct{}
}

func NewAudioEngine() *AudioEngine {
	return &AudioEngine{}
}

// Load decodifica un archivo de audio, calcula la duración real y remuestrea.
// CORRECCIÓN: Ahora devuelve la duración calculada matemáticamente.
func (ae *AudioEngine) Load(track Track) (time.Duration, error) {
	ae.Stop() // Limpieza preventiva[cite: 1]

	file, err := os.Open(track.Path)
	if err != nil {
		return 0, fmt.Errorf("abrir archivo: %w", err)
	}

	// Intentar MP3 primero, fallback a WAV[cite: 1]
	streamer, format, err := mp3.Decode(file)
	if err != nil {
		file.Close()
		file, err = os.Open(track.Path)
		if err != nil {
			return 0, fmt.Errorf("reabrir archivo: %w", err)
		}
		streamer, format, err = wav.Decode(file)
		if err != nil {
			file.Close()
			return 0, fmt.Errorf("formato no soportado: %s", track.Path)
		}
	}

	// CORRECCIÓN: Calcular duración exacta usando el número de frames y la tasa original.
	realDuration := format.SampleRate.D(streamer.Len())

	// CORRECCIÓN: Inicializar speaker una sola vez con la tasa de muestreo ESTÁNDAR.
	if !ae.isInit {
		speaker.Init(standardSampleRate, standardSampleRate.N(time.Second/10))
		ae.isInit = true
	}

	ae.streamer = streamer
	ae.format = format

	// CORRECCIÓN: Remuestrear el audio original a nuestra tasa estándar para evitar cambios de velocidad.
	resampled := beep.Resample(4, format.SampleRate, standardSampleRate, streamer)

	ae.ctrl = &beep.Ctrl{Streamer: resampled}
	ae.volume = &effects.Volume{
		Streamer: ae.ctrl,
		Base:     2,
		Volume:   0,
		Silent:   false,
	}

	return realDuration, nil
}

func (ae *AudioEngine) Play() chan struct{} {
	done := make(chan struct{})
	speaker.Play(beep.Seq(ae.volume, beep.Callback(func() {
		close(done)
	})))
	return done
}

func (ae *AudioEngine) Stop() {
	speaker.Clear()

	if ae.cancelChan != nil {
		close(ae.cancelChan)
		ae.cancelChan = nil
	}
	if ae.streamer != nil {
		ae.streamer.Close()
		ae.streamer = nil
	}
	ae.ctrl = nil
	ae.volume = nil
}

func (ae *AudioEngine) SetVolume(level float64) {
	if ae.volume != nil {
		ae.volume.Volume = level
	}
}

func (ae *AudioEngine) ToggleMute() {
	if ae.volume != nil {
		ae.volume.Silent = !ae.volume.Silent
	}
}

func (ae *AudioEngine) IsMuted() bool {
	return ae.volume != nil && ae.volume.Silent
}

// CORRECCIÓN: Obtener la posición real del stream de audio para la barra de progreso.
func (ae *AudioEngine) Position() time.Duration {
	if ae.streamer != nil {
		return ae.format.SampleRate.D(ae.streamer.Position())
	}
	return 0
}

func (ae *AudioEngine) Seek(position time.Duration) error {
	if ae.streamer == nil {
		return fmt.Errorf("no hay stream activo")
	}

	seeker, ok := ae.streamer.(beep.StreamSeeker)
	if !ok {
		return fmt.Errorf("formato no permite seek")
	}

	// Usamos la tasa original del archivo para el seek, no la estándar
	sampleRate := int(ae.format.SampleRate)
	samples := int(position.Seconds()) * sampleRate

	if err := seeker.Seek(samples); err != nil {
		return fmt.Errorf("seek fallido: %w", err)
	}
	return nil
}

func (ae *AudioEngine) Close() {
	ae.Stop()
	if ae.isInit {
		speaker.Close()
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// APLICACIÓN: MENSAJES PERSONALIZADOS
// ═══════════════════════════════════════════════════════════════════════════════

type (
	trackLoadedMsg struct {
		track    Track
		duration time.Duration // CORRECCIÓN: Incluir la duración real
	}

	playbackEndedMsg struct {
		sessionID int
	}

	tickMsg time.Time

	libraryScannedMsg struct {
		tracks []Track
	}

	errorMsg struct {
		err error
	}
)

// ═══════════════════════════════════════════════════════════════════════════════
// APLICACIÓN: MODELO DE LA UI (Bubble Tea)[cite: 1]
// ═══════════════════════════════════════════════════════════════════════════════

type AppModel struct {
	playlist *Playlist
	audio    *AudioEngine

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
}

func NewAppModel() AppModel {
	bar := progress.New(progress.WithDefaultGradient())
	bar.Width = progressWidth
	bar.ShowPercentage = false

	return AppModel{
		playlist:    NewPlaylist(),
		audio:       NewAudioEngine(),
		state:       StateStopped,
		progressBar: bar,
		volumeLevel: 0,
		showHelp:    true,
		showQueue:   true,
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.scanLibrary(),
		m.tick(),
	)
}

// ═══════════════════════════════════════════════════════════════════════════════
// APLICACIÓN: COMANDOS (tea.Cmd)
// ═══════════════════════════════════════════════════════════════════════════════

func (m AppModel) tick() tea.Cmd {
	// CORRECCIÓN: Ticks más rápidos para una barra suave, ya que ahora consultamos al motor de audio.
	return tea.Tick(time.Millisecond*250, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m AppModel) scanLibrary() tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			home = ""
		}

		dirs := []string{defaultDir, "./songs"}
		if home != "" {
			dirs = append(dirs,
				filepath.Join(home, "Music"),
				filepath.Join(home, "Música"),
			)
		}

		var found []Track
		for _, dir := range dirs {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				continue
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}

			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if ext != ".mp3" && ext != ".wav" {
					continue
				}

				path := filepath.Join(dir, entry.Name())
				name := strings.TrimSuffix(entry.Name(), ext)

				found = append(found, Track{
					ID:     fmt.Sprintf("track-%d", len(found)),
					Title:  name,
					Artist: "Desconocido",
					Path:   path,
					// Nota: La duración real ahora se obtiene al hacer Load() en el motor de audio.
				})
			}
		}

		return libraryScannedMsg{tracks: found}
	}
}

func (m AppModel) loadTrackCmd(track Track) tea.Cmd {
	return func() tea.Msg {
		// CORRECCIÓN: Extraer la duración real desde Load()
		duration, err := m.audio.Load(track)
		if err != nil {
			return errorMsg{err}
		}
		// Actualizar el track temporalmente para mostrarlo en la UI antes del Update
		track.Duration = duration
		return trackLoadedMsg{track: track, duration: duration}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// APLICACIÓN: ACTUALIZACIÓN DE ESTADO (Update)[cite: 1]
// ═══════════════════════════════════════════════════════════════════════════════

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg)
	case tea.KeyMsg:
		return m.handleKeyInput(msg)
	case libraryScannedMsg:
		return m.handleLibraryScanned(msg)
	case trackLoadedMsg:
		return m.handleTrackLoaded(msg)
	case tickMsg:
		return m.handleTick()
	case playbackEndedMsg:
		return m.handlePlaybackEnded(msg)
	case errorMsg:
		return m.handleError(msg)
	default:
		return m, nil
	}
}

func (m AppModel) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height

	newWidth := int(float64(msg.Width) * 0.7)
	if newWidth < 30 {
		newWidth = 30
	}
	m.progressBar.Width = newWidth

	return m, nil
}

func (m AppModel) handleKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.shutdown()
		return m, tea.Quit
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
		m.audio.ToggleMute()
	case "0":
		m.seekTo(0)
	case ".", ">":
		m.seekForward(seekSeconds)
	case ",", "<":
		m.seekBackward(seekSeconds)
	case "s":
		m.playlist.shuffle = !m.playlist.shuffle
	case "r":
		m.playlist.repeat = (m.playlist.repeat + 1) % 3
	case "l":
		m.showQueue = !m.showQueue
	case "h", "?":
		m.showHelp = !m.showHelp
	}
	return m, nil
}

func (m AppModel) handleLibraryScanned(msg libraryScannedMsg) (tea.Model, tea.Cmd) {
	for _, track := range msg.tracks {
		m.playlist.Add(track)
	}
	if !m.playlist.IsEmpty() && m.state == StateStopped {
		m.playlist.current = 0
	}
	return m, nil
}

func (m AppModel) handleTrackLoaded(msg trackLoadedMsg) (tea.Model, tea.Cmd) {
	m.audio.sessionID++
	sessionID := m.audio.sessionID
	m.audio.cancelChan = make(chan struct{})
	cancel := m.audio.cancelChan

	done := m.audio.Play()

	// CORRECCIÓN: Asignar la duración 100% real de la pista
	m.state = StatePlaying
	m.totalTime = msg.duration
	if m.totalTime <= 0 {
		m.totalTime = defaultDuration
	}

	// Actualizamos la duración en el playlist para que se vea en la cola
	m.playlist.tracks[m.playlist.current].Duration = msg.duration

	m.elapsed = 0

	waitCmd := func() tea.Msg {
		select {
		case <-done:
			return playbackEndedMsg{sessionID: sessionID}
		case <-cancel:
			return nil
		}
	}

	return m, tea.Batch(m.tick(), waitCmd)
}

func (m AppModel) handleTick() (tea.Model, tea.Cmd) {
	if m.state == StatePlaying {
		// CORRECCIÓN: El elapsed se basa en la posición del audio, no en la suma de ticks.
		m.elapsed = m.audio.Position()

		if m.elapsed > m.totalTime {
			m.elapsed = m.totalTime
		}
		return m, m.tick()
	}
	return m, nil
}

func (m AppModel) handlePlaybackEnded(msg playbackEndedMsg) (tea.Model, tea.Cmd) {
	if msg.sessionID != m.audio.sessionID {
		return m, nil
	}

	if m.playlist.isLast() && m.playlist.repeat == RepeatOff {
		m.state = StateStopped
		m.elapsed = 0
		m.audio.Stop()
	} else {
		return m.playNext()
	}
	return m, nil
}

func (m AppModel) handleError(msg errorMsg) (tea.Model, tea.Cmd) {
	m.lastError = msg.err
	m.state = StateStopped
	return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return errorMsg{err: nil}
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// APLICACIÓN: ACCIONES DEL REPRODUCTOR[cite: 1]
// ═══════════════════════════════════════════════════════════════════════════════

func (m AppModel) togglePlayback() (tea.Model, tea.Cmd) {
	switch m.state {
	case StatePlaying:
		m.audio.ctrl.Paused = true
		m.state = StatePaused
	case StatePaused:
		m.audio.ctrl.Paused = false
		m.state = StatePlaying
		return m, m.tick()
	case StateStopped:
		return m.playCurrent()
	}
	return m, nil
}

func (m AppModel) playCurrent() (tea.Model, tea.Cmd) {
	track, ok := m.playlist.Current()
	if !ok {
		return m, nil
	}
	return m, m.loadTrackCmd(track)
}

func (m AppModel) playNext() (tea.Model, tea.Cmd) {
	track, ok := m.playlist.Next()
	if !ok {
		m.resetPlayback()
		return m, nil
	}
	m.cursorIndex = m.playlist.current
	return m, m.loadTrackCmd(track)
}

func (m AppModel) playPrevious() (tea.Model, tea.Cmd) {
	if m.elapsed > 3*time.Second {
		m.seekTo(0)
		return m, nil
	}

	track, ok := m.playlist.Previous()
	if !ok {
		return m, nil
	}
	m.cursorIndex = m.playlist.current
	return m, m.loadTrackCmd(track)
}

func (m AppModel) playSelected() (tea.Model, tea.Cmd) {
	if !m.playlist.isValidIndex(m.cursorIndex) {
		return m, nil
	}
	m.playlist.JumpTo(m.cursorIndex)
	return m.playCurrent()
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
	newIndex := m.cursorIndex + delta
	if newIndex < 0 {
		newIndex = 0
	}
	if newIndex >= m.playlist.Length() {
		newIndex = m.playlist.Length() - 1
	}
	m.cursorIndex = newIndex
}

func (m *AppModel) adjustVolume(delta float64) {
	m.volumeLevel += delta
	if m.volumeLevel > maxVolume {
		m.volumeLevel = maxVolume
	}
	if m.volumeLevel < minVolume {
		m.volumeLevel = minVolume
	}
	m.audio.SetVolume(m.volumeLevel)
}

func (m *AppModel) seekTo(position time.Duration) {
	if err := m.audio.Seek(position); err != nil {
		m.lastError = err
		return
	}
	m.elapsed = clampDuration(position, 0, m.totalTime)
}

func (m *AppModel) seekForward(seconds int) {
	m.seekTo(m.elapsed + time.Duration(seconds)*time.Second)
}

func (m *AppModel) seekBackward(seconds int) {
	m.seekTo(m.elapsed - time.Duration(seconds)*time.Second)
}

func (m *AppModel) resetPlayback() {
	m.audio.Stop()
	m.state = StateStopped
	m.elapsed = 0
	m.totalTime = 0
}

func (m *AppModel) shutdown() {
	m.audio.Close()
}

// ═══════════════════════════════════════════════════════════════════════════════
// APLICACIÓN: VISTA (View) — RENDERIZADO POR COMPONENTES[cite: 1]
// ═══════════════════════════════════════════════════════════════════════════════

func (m AppModel) View() string {
	if m.lastError != nil {
		return m.renderErrorScreen()
	}

	sections := []string{
		m.renderHeader(),
		m.renderNowPlayingPanel(),
		"",
	}

	if m.showQueue {
		sections = append(sections, m.renderPlaylistPanel())
	}

	if m.showHelp {
		sections = append(sections, "", m.renderHelpPanel())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m AppModel) renderHeader() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(pink).
		MarginLeft(2).
		MarginTop(1).
		Render(appName)

	subtitle := lipgloss.NewStyle().
		Foreground(comment).
		Render(appSubtitle)

	return title + "  " + subtitle
}

func (m AppModel) renderNowPlayingPanel() string {
	track, hasTrack := m.playlist.Current()

	var content strings.Builder

	if hasTrack {
		content.WriteString(m.renderStatusLine(track))
		content.WriteString("\n")
		content.WriteString(m.renderProgressBar())
		content.WriteString("\n")
		content.WriteString(m.renderMetadataLine())
	} else {
		content.WriteString(
			lipgloss.NewStyle().Bold(true).Foreground(red).Render("⏹ Sin canciones"),
		)
		content.WriteString("\n")
		content.WriteString(
			lipgloss.NewStyle().Foreground(comment).Render(
				"Coloca archivos .mp3 o .wav en ./music/ o ~/Music/",
			),
		)
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(purple).
		Padding(1, 2).
		MarginLeft(2).
		MarginRight(2).
		Render(content.String())
}

func (m AppModel) renderStatusLine(track Track) string {
	icon := m.state.Icon()
	label := m.state.Label()

	var style lipgloss.Style
	switch m.state {
	case StatePlaying:
		style = lipgloss.NewStyle().Bold(true).Foreground(green)
	case StatePaused:
		style = lipgloss.NewStyle().Bold(true).Foreground(yellow)
	default:
		style = lipgloss.NewStyle().Bold(true).Foreground(red)
	}

	return lipgloss.NewStyle().
		MarginLeft(2).
		Render(style.Render(icon+" "+label) + "  " + track.DisplayName())
}

func (m AppModel) renderProgressBar() string {
	percent := 0.0
	if m.totalTime > 0 {
		percent = float64(m.elapsed) / float64(m.totalTime)
	}

	bar := m.progressBar.ViewAs(percent)
	timeInfo := fmt.Sprintf(" %s / %s",
		formatDuration(m.elapsed),
		formatDuration(m.totalTime),
	)

	return lipgloss.NewStyle().
		MarginLeft(2).
		Render(bar) +
		lipgloss.NewStyle().
			Foreground(comment).
			Render(timeInfo)
}

func (m AppModel) renderMetadataLine() string {
	shuffleIcon := ""
	if m.playlist.shuffle {
		shuffleIcon = " 🔀"
	}

	muteIcon := ""
	if m.audio.IsMuted() {
		muteIcon = " 🔇"
	}

	volPercent := int((1.0 + m.volumeLevel/10) * 100)
	volBar := renderVolumeBar(m.volumeLevel, maxVolume, minVolume)

	info := fmt.Sprintf("%s  %s%s  Vol: %s %d%%%s",
		m.playlist.repeat.Icon(),
		m.playlist.repeat.Label(),
		shuffleIcon,
		volBar,
		volPercent,
		muteIcon,
	)

	return lipgloss.NewStyle().
		Foreground(comment).
		MarginLeft(2).
		Render(info)
}

func (m AppModel) renderPlaylistPanel() string {
	if m.playlist.IsEmpty() {
		return lipgloss.NewStyle().
			Foreground(comment).
			MarginLeft(2).
			Render("📂 Cola vacía — Escaneando directorios...")
	}

	var lines []string

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(purple).
		MarginLeft(2).
		Render(fmt.Sprintf("📋 Cola (%d canciones)", m.playlist.Length()))
	lines = append(lines, header)

	visibleStart, visibleEnd := m.calculateVisibleRange(10)

	for i := visibleStart; i < visibleEnd; i++ {
		lines = append(lines, m.renderTrackRow(i))
	}

	if m.playlist.Length() > 10 {
		scrollHint := fmt.Sprintf("... %d más ...", m.playlist.Length()-10)
		lines = append(lines, lipgloss.NewStyle().Foreground(comment).MarginLeft(4).Render(scrollHint))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m AppModel) renderTrackRow(index int) string {
	track := m.playlist.tracks[index]

	prefix := "  "
	if index == m.playlist.current {
		prefix = "▶ "
	}

	number := fmt.Sprintf("%2d.", index+1)
	name := truncate(track.DisplayName(), 36)
	duration := track.FormattedDuration()

	row := fmt.Sprintf("%s%s %-38s %8s", prefix, number, name, duration)

	switch {
	case index == m.cursorIndex:
		return lipgloss.NewStyle().
			Background(selection).
			Foreground(green).
			Bold(true).
			MarginLeft(2).
			Render("▸ " + row)

	case index == m.playlist.current:
		return lipgloss.NewStyle().
			Foreground(cyan).
			MarginLeft(2).
			Render(row)

	default:
		return lipgloss.NewStyle().
			Foreground(foreground).
			MarginLeft(2).
			Render(row)
	}
}

func (m AppModel) calculateVisibleRange(maxVisible int) (int, int) {
	total := m.playlist.Length()
	if total <= maxVisible {
		return 0, total
	}

	start := m.cursorIndex - maxVisible/2
	if start < 0 {
		start = 0
	}
	end := start + maxVisible
	if end > total {
		end = total
		start = total - maxVisible
	}
	return start, end
}

func (m AppModel) renderHelpPanel() string {
	bindings := []struct {
		key  string
		desc string
	}{
		{"space", "Reproducir / Pausar"},
		{"n / N", "Siguiente / Anterior"},
		{"↑↓ / jk", "Navegar lista"},
		{"enter", "Reproducir seleccionada"},
		{"d", "Eliminar de la cola"},
		{"+ / -", "Volumen"},
		{"m", "Silenciar"},
		{"> / <", "Adelantar / Retroceder 10s"},
		{"0", "Reiniciar canción"},
		{"r", "Modo repetición"},
		{"s", "Aleatorio"},
		{"l", "Mostrar/ocultar cola"},
		{"h / ?", "Ayuda"},
		{"q", "Salir"},
	}

	var lines []string
	lines = append(lines, lipgloss.NewStyle().
		Foreground(purple).
		Bold(true).
		MarginLeft(2).
		Render("⌨️  Controles"))

	for _, b := range bindings {
		key := lipgloss.NewStyle().
			Foreground(cyan).
			Bold(true).
			Width(12).
			Render(b.key)
		desc := lipgloss.NewStyle().
			Foreground(comment).
			Render(b.desc)
		lines = append(lines, lipgloss.NewStyle().MarginLeft(4).Render(key+" "+desc))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m AppModel) renderErrorScreen() string {
	msg := lipgloss.NewStyle().
		Foreground(red).
		Bold(true).
		MarginLeft(2).
		Render("✖ Error: " + m.lastError.Error())

	hint := lipgloss.NewStyle().
		Foreground(comment).
		MarginLeft(2).
		Render("Presiona 'q' para salir.")

	return "\n" + msg + "\n\n" + hint
}

// ═══════════════════════════════════════════════════════════════════════════════
// UTILIDADES[cite: 1]
// ═══════════════════════════════════════════════════════════════════════════════

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func clampDuration(value, min, max time.Duration) time.Duration {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func renderVolumeBar(current, max, min float64) string {
	const bars = 10
	rangeSize := max - min
	if rangeSize == 0 {
		return strings.Repeat("░", bars)
	}

	normalized := (current - min) / rangeSize
	filled := int(normalized * bars)
	if filled < 0 {
		filled = 0
	}
	if filled > bars {
		filled = bars
	}

	return lipgloss.NewStyle().Foreground(cyan).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(comment).Render(strings.Repeat("░", bars-filled))
}

// ═══════════════════════════════════════════════════════════════════════════════
// PUNTO DE ENTRADA[cite: 1]
// ═══════════════════════════════════════════════════════════════════════════════

func main() {
	model := NewAppModel()
	program := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error al iniciar GoPlayer: %v\n", err)
		os.Exit(1)
	}
}
