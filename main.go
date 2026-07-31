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

// ═══════════════════════════════════════════════════════════════
//  ESTADOS DEL REPRODUCTOR
// ═══════════════════════════════════════════════════════════════

type PlayerState int

const (
	StateStopped PlayerState = iota
	StatePlaying
	StatePaused
)

func (s PlayerState) String() string {
	switch s {
	case StatePlaying:
		return "▶ Reproduciendo"
	case StatePaused:
		return "⏸ Pausado"
	default:
		return "⏹ Detenido"
	}
}

// ═══════════════════════════════════════════════════════════════
//  MODOS DE REPETICIÓN
// ═══════════════════════════════════════════════════════════════

type RepeatMode int

const (
	RepeatNone RepeatMode = iota
	RepeatOne
	RepeatAll
)

func (r RepeatMode) String() string {
	switch r {
	case RepeatOne:
		return "🔂 Repetir una"
	case RepeatAll:
		return "🔁 Repetir todo"
	default:
		return "➡ Sin repetir"
	}
}

// ═══════════════════════════════════════════════════════════════
//  CANCIÓN
// ═══════════════════════════════════════════════════════════════

type Song struct {
	ID       string
	Title    string
	Artist   string
	Album    string
	Duration time.Duration
	Path     string
}

func (s Song) String() string {
	if s.Artist == "" {
		return s.Title
	}
	return s.Artist + " - " + s.Title
}

func (s Song) DurationStr() string {
	if s.Duration <= 0 {
		return "?:??"
	}
	m := int(s.Duration.Minutes())
	sec := int(s.Duration.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, sec)
}

// ═══════════════════════════════════════════════════════════════
//  COLA DE REPRODUCCIÓN
// ═══════════════════════════════════════════════════════════════

type Queue struct {
	songs   []Song
	current int
	shuffle bool
	repeat  RepeatMode
}

func NewQueue() *Queue {
	return &Queue{
		songs:   make([]Song, 0),
		current: -1,
	}
}

func (q *Queue) Add(song Song) {
	q.songs = append(q.songs, song)
	if q.current == -1 {
		q.current = 0
	}
}

func (q *Queue) Remove(index int) {
	if index < 0 || index >= len(q.songs) {
		return
	}
	q.songs = append(q.songs[:index], q.songs[index+1:]...)
	if q.current >= len(q.songs) {
		q.current = len(q.songs) - 1
	}
	if len(q.songs) == 0 {
		q.current = -1
	}
}

func (q *Queue) Current() (Song, bool) {
	if q.current < 0 || q.current >= len(q.songs) {
		return Song{}, false
	}
	return q.songs[q.current], true
}

func (q *Queue) Next() (Song, bool) {
	if len(q.songs) == 0 {
		return Song{}, false
	}

	switch q.repeat {
	case RepeatOne:
		return q.songs[q.current], true
	case RepeatAll:
		q.current = (q.current + 1) % len(q.songs)
		return q.songs[q.current], true
	default:
		if q.current+1 >= len(q.songs) {
			return Song{}, false
		}
		q.current++
		return q.songs[q.current], true
	}
}

func (q *Queue) Prev() (Song, bool) {
	if len(q.songs) == 0 {
		return Song{}, false
	}
	if q.current <= 0 {
		return Song{}, false
	}
	q.current--
	return q.songs[q.current], true
}

func (q *Queue) SetCurrent(index int) bool {
	if index < 0 || index >= len(q.songs) {
		return false
	}
	q.current = index
	return true
}

func (q *Queue) Len() int {
	return len(q.songs)
}

func (q *Queue) IsLast() bool {
	if q.repeat != RepeatNone {
		return false
	}
	return q.current >= len(q.songs)-1
}

// ═══════════════════════════════════════════════════════════════
//  MENSAJES PERSONALIZADOS
// ═══════════════════════════════════════════════════════════════

type (
	songLoadedMsg struct {
		streamer beep.StreamSeekCloser
		format   beep.Format
		song     Song
	}

	playbackFinishedMsg struct{}

	tickMsg struct{}

	errMsg struct {
		err error
	}

	scanDirMsg struct {
		songs []Song
	}
)

// ═══════════════════════════════════════════════════════════════
//  ESTILOS
// ═══════════════════════════════════════════════════════════════

var (
	// Colores Dracula
	colorPink    = lipgloss.Color("#FF79C6")
	colorCyan    = lipgloss.Color("#8BE9FD")
	colorGreen   = lipgloss.Color("#50FA7B")
	colorYellow  = lipgloss.Color("#F1FA8C")
	colorOrange  = lipgloss.Color("#FFB86C")
	colorRed     = lipgloss.Color("#FF5555")
	colorPurple  = lipgloss.Color("#BD93F9")
	colorFg      = lipgloss.Color("#F8F8F2")
	colorComment = lipgloss.Color("#6272A4")
	colorBg      = lipgloss.Color("#282A36")
	colorCurrent = lipgloss.Color("#44475A")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPink).
			MarginLeft(2).
			MarginTop(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorComment).
			MarginLeft(2)

	playingStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorGreen)

	pausedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorYellow)

	stoppedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorRed)

	infoStyle = lipgloss.NewStyle().
			Foreground(colorFg).
			MarginLeft(2)

	controlsStyle = lipgloss.NewStyle().
			Foreground(colorComment).
			MarginLeft(2).
			MarginBottom(1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorComment)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true).
			MarginLeft(2)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPurple).
			Padding(1, 2).
			MarginLeft(2).
			MarginRight(2)
)

// ═══════════════════════════════════════════════════════════════
//  MODELO PRINCIPAL
// ═══════════════════════════════════════════════════════════════

type Model struct {
	// Estado
	state    PlayerState
	queue    *Queue
	lastSong Song

	// Audio
	streamer beep.StreamSeekCloser
	format   beep.Format
	ctrl     *beep.Ctrl
	volCtrl  *effects.Volume
	volLevel float64 // -5 a +5

	// UI
	width     int
	height    int
	cursor    int
	progress  progress.Model
	showHelp  bool
	showQueue bool

	// Tiempos
	elapsed  time.Duration
	duration time.Duration

	// Errores
	err error
}

func NewModel() Model {
	p := progress.New(progress.WithDefaultGradient())
	p.Width = 50
	p.ShowPercentage = false

	return Model{
		queue:    NewQueue(),
		state:    StateStopped,
		progress: p,
		volLevel: 0,
		showHelp: true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.scanMusicDir(),
		m.tickCmd(),
	)
}

// ═══════════════════════════════════════════════════════════════
//  COMANDOS
// ═══════════════════════════════════════════════════════════════

func (m Model) scanMusicDir() tea.Cmd {
	return func() tea.Msg {
		home, _ := os.UserHomeDir()
		dirs := []string{
			"./music",
			"./songs",
			filepath.Join(home, "Music"),
			filepath.Join(home, "Música"),
		}

		var songs []Song
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
				if ext != ".mp3" && ext != ".wav" && ext != ".flac" && ext != ".ogg" {
					continue
				}
				path := filepath.Join(dir, entry.Name())
				name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
				songs = append(songs, Song{
					ID:     fmt.Sprintf("%d", len(songs)),
					Title:  name,
					Artist: "Desconocido",
					Path:   path,
				})
			}
		}
		return scanDirMsg{songs: songs}
	}
}

func (m Model) loadSongCmd(song Song) tea.Cmd {
	return func() tea.Msg {
		f, err := os.Open(song.Path)
		if err != nil {
			return errMsg{err}
		}

		var streamer beep.StreamSeekCloser
		var format beep.Format

		// Intentar MP3
		streamer, format, err = mp3.Decode(f)
		if err != nil {
			f.Seek(0, 0)
			streamer, format, err = wav.Decode(f)
			if err != nil {
				f.Close()
				return errMsg{fmt.Errorf("formato no soportado: %s", song.Path)}
			}
		}

		return songLoadedMsg{
			streamer: streamer,
			format:   format,
			song:     song,
		}
	}
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

// ═══════════════════════════════════════════════════════════════
//  UPDATE
// ═══════════════════════════════════════════════════════════════

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		height := msg.Height
		m.height = height
		m.progress.Width = msg.Width - 20
		if m.progress.Width < 30 {
			m.progress.Width = 30
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.cleanup()
			return m, tea.Quit

		case " ":
			return m.togglePlayPause()

		case "n":
			return m.playNext()

		case "N":
			return m.playPrevious()

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < m.queue.Len()-1 {
				m.cursor++
			}

		case "enter":
			if m.cursor >= 0 && m.cursor < m.queue.Len() {
				m.queue.SetCurrent(m.cursor)
				return m.playCurrent()
			}

		case "d":
			if m.cursor >= 0 && m.cursor < m.queue.Len() {
				wasCurrent := m.cursor == m.queue.current
				m.queue.Remove(m.cursor)
				if m.cursor >= m.queue.Len() && m.cursor > 0 {
					m.cursor--
				}
				if wasCurrent {
					m.cleanupAudio()
					m.state = StateStopped
					m.elapsed = 0
					m.duration = 0
					if m.queue.Len() > 0 {
						return m.playCurrent()
					}
				}
			}

		case "+", "=":
			m.volumeUp()

		case "-":
			m.volumeDown()

		case "m":
			m.toggleMute()

		case "s":
			m.queue.shuffle = !m.queue.shuffle

		case "r":
			m.queue.repeat = (m.queue.repeat + 1) % 3

		case "l":
			m.showQueue = !m.showQueue

		case "h", "?":
			m.showHelp = !m.showHelp

		case "0":
			m.seekTo(0)

		case ".", ">":
			m.seekForward(10)

		case ",", "<":
			m.seekBackward(10)
		}

	case scanDirMsg:
		for _, s := range msg.songs {
			m.queue.Add(s)
		}
		if m.queue.Len() > 0 && m.state == StateStopped {
			m.queue.current = 0
		}

	case songLoadedMsg:
		m.cleanupAudio()

		m.streamer = msg.streamer
		m.format = msg.format
		m.lastSong = msg.song

		speaker.Init(msg.format.SampleRate, msg.format.SampleRate.N(time.Second/10))

		m.ctrl = &beep.Ctrl{Streamer: msg.streamer}
		m.volCtrl = &effects.Volume{
			Streamer: m.ctrl,
			Base:     2,
			Volume:   m.volLevel,
			Silent:   false,
		}

		done := make(chan struct{})
		speaker.Play(beep.Seq(m.volCtrl, beep.Callback(func() {
			close(done)
		})))

		m.state = StatePlaying
		m.duration = msg.song.Duration
		if m.duration <= 0 {
			// Estimar duración si no está disponible
			m.duration = time.Minute * 3
		}
		m.elapsed = 0

		cmds = append(cmds, m.tickCmd())
		cmds = append(cmds, func() tea.Msg {
			<-done
			return playbackFinishedMsg{}
		})

	case tickMsg:
		if m.state == StatePlaying {
			m.elapsed += time.Second
			if m.elapsed > m.duration {
				m.elapsed = m.duration
			}
			cmds = append(cmds, m.tickCmd())
		}

	case playbackFinishedMsg:
		if m.queue.IsLast() && m.queue.repeat == RepeatNone {
			m.state = StateStopped
			m.elapsed = 0
			m.cleanupAudio()
		} else {
			return m.playNext()
		}

	case errMsg:
		m.err = msg.err
		m.state = StateStopped
	}

	return m, tea.Batch(cmds...)
}

// ═══════════════════════════════════════════════════════════════
//  ACCIONES DEL REPRODUCTOR
// ═══════════════════════════════════════════════════════════════

func (m Model) togglePlayPause() (Model, tea.Cmd) {
	if m.ctrl == nil {
		return m.playCurrent()
	}

	switch m.state {
	case StatePlaying:
		m.ctrl.Paused = true
		m.state = StatePaused
		speaker.Lock()
		speaker.Unlock()

	case StatePaused:
		m.ctrl.Paused = false
		m.state = StatePlaying
		return m, tea.Batch(m.tickCmd())

	case StateStopped:
		return m.playCurrent()
	}

	return m, nil
}

func (m Model) playCurrent() (Model, tea.Cmd) {
	song, ok := m.queue.Current()
	if !ok {
		return m, nil
	}
	return m, m.loadSongCmd(song)
}

func (m Model) playNext() (Model, tea.Cmd) {
	song, ok := m.queue.Next()
	if !ok {
		m.state = StateStopped
		m.cleanupAudio()
		m.elapsed = 0
		return m, nil
	}
	m.cursor = m.queue.current
	return m, m.loadSongCmd(song)
}

func (m Model) playPrevious() (Model, tea.Cmd) {
	if m.elapsed > 3*time.Second && m.streamer != nil {
		m.seekTo(0)
		return m, nil
	}

	song, ok := m.queue.Prev()
	if !ok {
		return m, nil
	}
	m.cursor = m.queue.current
	return m, m.loadSongCmd(song)
}

func (m *Model) volumeUp() {
	m.volLevel += 0.5
	if m.volLevel > 5 {
		m.volLevel = 5
	}
	if m.volCtrl != nil {
		m.volCtrl.Volume = m.volLevel
	}
}

func (m *Model) volumeDown() {
	m.volLevel -= 0.5
	if m.volLevel < -5 {
		m.volLevel = -5
	}
	if m.volCtrl != nil {
		m.volCtrl.Volume = m.volLevel
	}
}

func (m *Model) toggleMute() {
	if m.volCtrl != nil {
		m.volCtrl.Silent = !m.volCtrl.Silent
	}
}

func (m *Model) seekTo(pos time.Duration) {
	if m.streamer == nil {
		return
	}
	sampleRate := int(m.format.SampleRate)
	samples := int(pos.Seconds()) * sampleRate * m.format.NumChannels
	m.streamer.Seek(samples)
	m.elapsed = pos
}

func (m *Model) seekForward(seconds int) {
	newPos := m.elapsed + time.Duration(seconds)*time.Second
	if newPos > m.duration {
		newPos = m.duration
	}
	m.seekTo(newPos)
}

func (m *Model) seekBackward(seconds int) {
	newPos := m.elapsed - time.Duration(seconds)*time.Second
	if newPos < 0 {
		newPos = 0
	}
	m.seekTo(newPos)
}

func (m *Model) cleanupAudio() {
	if m.streamer != nil {
		m.streamer.Close()
		m.streamer = nil
	}
	m.ctrl = nil
	m.volCtrl = nil
}

func (m *Model) cleanup() {
	m.cleanupAudio()
	speaker.Close()
}

// ═══════════════════════════════════════════════════════════════
//  VISTA
// ═══════════════════════════════════════════════════════════════

func (m Model) View() string {
	if m.err != nil {
		return "\n" + errorStyle.Render("✖ Error: "+m.err.Error()) +
			"\n\n" + controlsStyle.Render("Presiona 'q' para salir.")
	}

	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Panel de reproducción
	sections = append(sections, m.renderPlayerPanel())

	// Separador
	sections = append(sections, "")

	// Lista de canciones
	sections = append(sections, m.renderQueue())

	// Ayuda
	if m.showHelp {
		sections = append(sections, "")
		sections = append(sections, m.renderHelp())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderHeader() string {
	return titleStyle.Render("🎵 GoPlayer") + "  " +
		subtitleStyle.Render("— Reproductor de música TUI")
}

func (m Model) renderPlayerPanel() string {
	var b strings.Builder

	// Estado y canción actual
	song, ok := m.queue.Current()
	if ok {
		var statusStr string
		switch m.state {
		case StatePlaying:
			statusStr = playingStyle.Render("▶ ") + song.String()
		case StatePaused:
			statusStr = pausedStyle.Render("⏸ ") + song.String()
		default:
			statusStr = stoppedStyle.Render("⏹ ") + song.String()
		}
		b.WriteString(infoStyle.Render(statusStr))
		b.WriteString("\n")

		// Barra de progreso
		progressPercent := 0.0
		if m.duration > 0 {
			progressPercent = float64(m.elapsed) / float64(m.duration)
		}
		bar := m.progress.ViewAs(progressPercent)
		b.WriteString(infoStyle.Render(bar))

		// Tiempos
		elapsedStr := formatDuration(m.elapsed)
		durationStr := song.DurationStr()
		if m.duration > 0 {
			durationStr = formatDuration(m.duration)
		}
		timeStr := fmt.Sprintf(" %s / %s", elapsedStr, durationStr)
		b.WriteString(lipgloss.NewStyle().Foreground(colorComment).Render(timeStr))
		b.WriteString("\n")

		// Info adicional
		shuffleStr := ""
		if m.queue.shuffle {
			shuffleStr = " 🔀"
		}
		muteStr := ""
		if m.volCtrl != nil && m.volCtrl.Silent {
			muteStr = " 🔇"
		}
		info := fmt.Sprintf("Volumen: %.0f%%  %s%s%s",
			(1.0+m.volLevel/10)*100,
			m.queue.repeat.String(),
			shuffleStr,
			muteStr,
		)
		b.WriteString(subtitleStyle.Render(info))
	} else {
		b.WriteString(stoppedStyle.Render("⏹ No hay canciones en la cola"))
		b.WriteString("\n")
		b.WriteString(subtitleStyle.Render("Coloca archivos .mp3 o .wav en ./music/ o ~/Music/"))
	}

	return boxStyle.Render(b.String())
}

func (m Model) renderQueue() string {
	if m.queue.Len() == 0 {
		return subtitleStyle.Render("📂 Cola vacía — Escaneando directorios de música...")
	}

	var lines []string
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPurple).
		Render(fmt.Sprintf("📋 Cola de reproducción (%d canciones)", m.queue.Len()))
	lines = append(lines, header)

	for i, song := range m.queue.songs {
		prefix := "  "
		if i == m.queue.current {
			prefix = "▶ "
		}

		num := fmt.Sprintf("%2d.", i+1)
		name := song.String()
		dur := song.DurationStr()

		line := fmt.Sprintf("%s%s %-40s %8s", prefix, num, truncate(name, 38), dur)

		if i == m.cursor {
			line = lipgloss.NewStyle().
				Background(colorCurrent).
				Foreground(colorGreen).
				Bold(true).
				Render(" > " + line)
		} else if i == m.queue.current {
			line = lipgloss.NewStyle().
				Foreground(colorCyan).
				Render(line)
		} else {
			line = lipgloss.NewStyle().
				Foreground(colorFg).
				Render(line)
		}

		lines = append(lines, line)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Model) renderHelp() string {
	help := [][]string{
		{"space", "Reproducir / Pausar"},
		{"n", "Siguiente canción"},
		{"N", "Canción anterior"},
		{"↑/↓ o j/k", "Navegar lista"},
		{"enter", "Reproducir seleccionada"},
		{"d", "Eliminar de la cola"},
		{"+ / -", "Subir / Bajar volumen"},
		{"m", "Silenciar"},
		{"> / <", "Adelantar / Retroceder 10s"},
		{"0", "Reiniciar canción"},
		{"r", "Modo repetición"},
		{"s", "Modo aleatorio"},
		{"h / ?", "Mostrar / ocultar ayuda"},
		{"q", "Salir"},
	}

	var lines []string
	lines = append(lines, lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Render("⌨️  Controles:"))

	for _, h := range help {
		key := helpKeyStyle.Render(fmt.Sprintf("%-12s", h[0]))
		desc := helpDescStyle.Render(h[1])
		lines = append(lines, "  "+key+" "+desc)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// ═══════════════════════════════════════════════════════════════
//  UTILIDADES
// ═══════════════════════════════════════════════════════════════

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// ═══════════════════════════════════════════════════════════════
//  MAIN
// ═══════════════════════════════════════════════════════════════

func main() {
	m := NewModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error al iniciar: %v\n", err)
		os.Exit(1)
	}
}
