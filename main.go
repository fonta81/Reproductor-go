package main

import (
	"fmt"
	"math/rand"
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
	appName         = "󰎆 GoPlayer"
	appSubtitle     = "Reproductor de música TUI"
	defaultDir      = "./music"
	seekSeconds     = 10
	volumeStep      = 0.5
	maxVolume       = 0.0   // 0.0 = 100% amplitud original (Sin distorsión)
	minVolume       = -10.0 // Límite inferior absoluto
	progressWidth   = 50
	defaultDuration = 3 * time.Minute // fallback cuando no hay metadata

	// Tasa de muestreo estándar para evitar aceleración (Chipmunk effect)
	standardSampleRate = beep.SampleRate(44100)
)

// Paleta de colores: tema Dracula optimizado para legibilidad en terminal
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

	// Estilos para el panel de ayuda moderno
	helpContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(selection).
				Padding(1, 2).
				MarginLeft(2).
				MarginTop(1)

	helpCategoryStyle = lipgloss.NewStyle().
				Foreground(purple).
				Bold(true).
				MarginBottom(1)

	helpKeyStyle = lipgloss.NewStyle().
			Background(selection).
			Foreground(cyan).
			Bold(true).
			Align(lipgloss.Center).
			Width(10).
			MarginRight(1).
			MarginBottom(1)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(comment).
			MarginBottom(1)
)

// ═══════════════════════════════════════════════════════════════════════════════
// DOMINIO: MODELOS DE DATOS
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
		return "󰐊"
	case StatePaused:
		return "󰏤"
	default:
		return "󰓛"
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
		return "󰑣"
	case RepeatAll:
		return "󰑘"
	default:
		return "󰑶"
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
// DOMINIO: GESTIÓN DE LA LISTA DE REPRODUCCIÓN
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

	if p.shuffle {
		if len(p.tracks) <= 1 {
			return p.tracks[p.current], true
		}

		nextIndex := p.current
		for nextIndex == p.current {
			nextIndex = rand.Intn(len(p.tracks))
		}

		p.current = nextIndex
		return p.tracks[p.current], true
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

func (ae *AudioEngine) Load(track Track) (time.Duration, error) {
	ae.Stop()

	file, err := os.Open(track.Path)
	if err != nil {
		return 0, fmt.Errorf("abrir archivo: %w", err)
	}

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

	realDuration := format.SampleRate.D(streamer.Len())

	if !ae.isInit {
		speaker.Init(standardSampleRate, standardSampleRate.N(time.Second/10))
		ae.isInit = true
	}

	ae.streamer = streamer
	ae.format = format

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
		duration time.Duration
	}
	playbackEndedMsg struct {
		sessionID int
	}
	tickMsg           time.Time
	libraryScannedMsg struct {
		tracks []Track
	}
	errorMsg struct {
		err error
	}
)

// ═══════════════════════════════════════════════════════════════════════════════
// APLICACIÓN: MODELO DE LA UI (Bubble Tea)
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
				})
			}
		}
		return libraryScannedMsg{tracks: found}
	}
}

func (m AppModel) loadTrackCmd(track Track) tea.Cmd {
	return func() tea.Msg {
		duration, err := m.audio.Load(track)
		if err != nil {
			return errorMsg{err}
		}
		track.Duration = duration
		return trackLoadedMsg{track: track, duration: duration}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// APLICACIÓN: ACTUALIZACIÓN DE ESTADO (Update)
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

	m.state = StatePlaying
	m.totalTime = msg.duration
	if m.totalTime <= 0 {
		m.totalTime = defaultDuration
	}

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
// APLICACIÓN: ACCIONES DEL REPRODUCTOR
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
// APLICACIÓN: VISTA (View) — RENDERIZADO POR COMPONENTES
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
			lipgloss.NewStyle().Bold(true).Foreground(red).Render("󰓛 Sin canciones"),
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
		Render(bar) + lipgloss.NewStyle().Foreground(cyan).Render(timeInfo)
}

func (m AppModel) renderMetadataLine() string {
	volDisplay := ""
	if m.audio.IsMuted() {
		volDisplay = lipgloss.NewStyle().Foreground(red).Render("󰝟 Mut")
	} else {
		pct := int((m.volumeLevel - minVolume) / (maxVolume - minVolume) * 100)
		volIcon := "󰕾"
		if pct < 30 {
			volIcon = "󰕿"
		} else if pct < 70 {
			volIcon = "󰖀"
		}
		volDisplay = fmt.Sprintf("%s %d%%", volIcon, pct)
	}

	queueDisplay := fmt.Sprintf("%d/%d", m.playlist.current+1, m.playlist.Length())
	if m.playlist.Length() == 0 {
		queueDisplay = "0/0"
	}

	metaStyle := lipgloss.NewStyle().Foreground(comment).MarginLeft(2)

	shuffleIcon := " 󰒑 "
	if m.playlist.shuffle {
		shuffleIcon = " 󰒎 "
	}

	repeatIcon := " " + m.playlist.repeat.Icon() + " "

	return metaStyle.Render(
		fmt.Sprintf("%s  |  󰲸 %s  |%s|%s",
			volDisplay,
			queueDisplay,
			shuffleIcon,
			repeatIcon,
		),
	)
}

func (m AppModel) renderPlaylistPanel() string {
	if m.playlist.IsEmpty() {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(orange).Render("  Próximamente:\n"))

	start := m.cursorIndex - 3
	if start < 0 {
		start = 0
	}
	end := start + 7
	if end > m.playlist.Length() {
		end = m.playlist.Length()
	}

	if start > 0 {
		builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("    ↑ ...\n"))
	}

	for i := start; i < end; i++ {
		t := m.playlist.tracks[i]

		cursor := "  "
		if i == m.cursorIndex {
			cursor = "󰐊 "
		}

		style := lipgloss.NewStyle()
		if i == m.playlist.current {
			style = style.Bold(true).Foreground(pink)
		} else if i == m.cursorIndex {
			style = style.Foreground(cyan)
		} else {
			style = style.Foreground(foreground)
		}

		row := fmt.Sprintf("%s%d. %-35s [%s]",
			cursor,
			i+1,
			truncate(t.DisplayName(), 35),
			t.FormattedDuration(),
		)
		builder.WriteString("  " + style.Render(row) + "\n")
	}

	if end < m.playlist.Length() {
		builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("    ↓ ...\n"))
	}

	return builder.String()
}

// NUEVO: renderHelpPanel () — Versión en Grid / Rejilla Responsiva
func (m AppModel) renderHelpPanel() string {
	type binding struct {
		key  string
		desc string
	}

	type category struct {
		title    string
		bindings []binding
	}

	// 1. Agrupamos lógicamente los atajos
	categories := []category{
		{
			title: "▶ Reproducción",
			bindings: []binding{
				{"espacio", "Play / Pausa"},
				{"n / N", "Sig / Anterior"},
				{"> / <", "Adel. / Atrasar"},
				{"0", "Reiniciar"},
			},
		},
		{
			title: "🧭 Navegación",
			bindings: []binding{
				{"↑↓ / jk", "Mover cursor"},
				{"enter", "Reproducir"},
				{"d", "Eliminar de cola"},
				{"l", "Ocultar cola"},
			},
		},
		{
			title: "🎛 Audio & Modos",
			bindings: []binding{
				{"+ / -", "Volumen"},
				{"m", "Silenciar"},
				{"r", "Repetir"},
				{"s", "Aleatorio"},
			},
		},
		{
			title: "⚙️ Sistema",
			bindings: []binding{
				{"h / ?", "Ocultar ayuda"},
				{"q", "Salir"},
			},
		},
	}

	// 2. Renderizamos cada categoría como un bloque vertical
	var blocks []string
	for _, cat := range categories {
		var lines []string
		lines = append(lines, helpCategoryStyle.Render(cat.title))

		for _, b := range cat.bindings {
			keyView := helpKeyStyle.Render(b.key)
			descView := helpDescStyle.Render(b.desc)

			// Unimos la tecla (badge) con su descripción horizontalmente
			row := lipgloss.JoinHorizontal(lipgloss.Left, keyView, descView)
			lines = append(lines, row)
		}

		// Unimos todas las filas de esta categoría
		block := lipgloss.JoinVertical(lipgloss.Left, lines...)
		blocks = append(blocks, block)
	}

	// 3. Lógica Responsiva (Rejilla / Grid)
	var rows []string
	var colsPerRow int

	// Ajustamos cuántas columnas mostrar según el ancho de la terminal
	if m.width > 120 {
		colsPerRow = 4 // Todo en una sola fila horizontal
	} else if m.width > 75 {
		colsPerRow = 2 // Rejilla 2x2
	} else {
		colsPerRow = 1 // Vertical clásico para terminales estrechas
	}

	// Chunking: Agrupamos los bloques en filas dependiendo de colsPerRow
	for i := 0; i < len(blocks); i += colsPerRow {
		end := i + colsPerRow
		if end > len(blocks) {
			end = len(blocks)
		}

		var rowBlocks []string
		for _, block := range blocks[i:end] {
			// Forzamos un ancho mínimo por bloque (32) para que las columnas
			// respiren y no colisionen los textos.
			paddedBlock := lipgloss.NewStyle().Width(32).Render(block)
			rowBlocks = append(rowBlocks, paddedBlock)
		}

		// Unimos las columnas de esta fila horizontalmente
		rowView := lipgloss.JoinHorizontal(lipgloss.Top, rowBlocks...)
		rows = append(rows, rowView)

		// Separación adicional si hay múltiples filas de categorías
		if end < len(blocks) {
			rows = append(rows, "")
		}
	}

	// 4. Empaquetamos todo en un contenedor con borde
	grid := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return helpContainerStyle.Render(grid)
}

func (m AppModel) renderErrorScreen() string {
	errStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(foreground).
		Background(red).
		Padding(1, 2)

	if m.lastError == nil {
		return ""
	}

	msg := fmt.Sprintf("CRITICAL ERROR:\n\n%v\n\nPresiona 'q' para salir o espera a recuperarte.", m.lastError)
	return errStyle.Render(msg)
}

// ═══════════════════════════════════════════════════════════════════════════════
// UTILIDADES
// ═══════════════════════════════════════════════════════════════════════════════

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	s := (d % time.Minute) / time.Second
	return fmt.Sprintf("%02d:%02d", m, s)
}

func clampDuration(val, min, max time.Duration) time.Duration {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func truncate(str string, max int) string {
	if len(str) > max {
		return str[:max-3] + "..."
	}
	return str
}

// ═══════════════════════════════════════════════════════════════════════════════
// MAIN
// ═══════════════════════════════════════════════════════════════════════════════

func main() {
	p := tea.NewProgram(NewAppModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error al iniciar el reproductor: %v\n", err)
		os.Exit(1)
	}
}
