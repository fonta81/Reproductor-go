/*
================================================================================
GOPLAYER - TERMINAL USER INTERFACE (TUI) MUSIC PLAYER
================================================================================

PURPOSE & ARCHITECTURE:
This program is a fully functional music player built for the terminal. It uses
The Elm Architecture (Model-View-Update or MVU) implemented via the Bubble Tea framework.
The architecture strictly separates state (Model), logic/events (Update), and
rendering (View).
================================================================================
*/

package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
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
// CONFIGURATION & CONSTANTS
// ═══════════════════════════════════════════════════════════════════════════════

const (
	appName         = "GoPlayer"
	appSubtitle     = "Reproductor de música TUI"
	defaultDir      = "./music" // Default directory to scan for music files
	seekSeconds     = 10        // Number of seconds to jump forward/backward
	progressWidth   = 50
	defaultDuration = 3 * time.Minute // Fallback duration if metadata is missing

	standardSampleRate = beep.SampleRate(44100) //
)

const (
	iconPlay      = "󰐊"
	iconPause     = "󰏤"
	iconStop      = "󰓛"
	iconNext      = "󰒭"
	iconPrev      = "󰒮"
	iconRepeatOne = "󰑘"
	iconRepeatAll = "󰑗"
	iconRepeatOff = "󰁔"
	iconShuffle   = "󰒟"
	iconVolume    = "󰕾"
	iconMute      = "󰖁"
	iconQueue     = "󰉖"
	iconUp        = "󰅂"
	iconDown      = "󰅀"
	iconNav       = "󰍉"
	iconAudio     = "󰕾"
	iconSystem    = "󰒓"
	iconFolder    = "󰉋"
)

const (
	volumeStep = 3.0   // 3 dB step represents a clearly perceptible change in volume.
	maxVolume  = 0.0   // 0 dBFS is the maximum digital volume before clipping occurs.
	minVolume  = -30.0 // -30 dB is considered the volume floor (virtually silent).
)

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
// DOMAIN: DATA MODELS
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
		return iconPlay
	case StatePaused:
		return iconPause
	default:
		return iconStop
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
		return iconRepeatOne
	case RepeatAll:
		return iconRepeatAll
	default:
		return iconRepeatOff
	}
}

type Track struct {
	ID       string
	Title    string
	Artist   string
	Album    string
	Duration time.Duration
	Path     string // Absolute or relative path to the physical audio file
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
// DOMAIN: PLAYLIST MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

type Playlist struct {
	tracks          []Track
	current         int
	shuffle         bool
	repeat          RepeatMode
	shuffleOrder    []int
	shuffleIdx      int
	shuffleStartIdx int
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
	if p.shuffle {
		p.regenerateShuffle()
	}
}

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

func (p *Playlist) Clear() {
	p.tracks = make([]Track, 0)
	p.current = -1
	p.shuffleOrder = nil
	p.shuffleIdx = 0
	p.shuffleStartIdx = 0
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

func (p *Playlist) Length() int   { return len(p.tracks) }
func (p *Playlist) IsEmpty() bool { return len(p.tracks) == 0 }

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

func (p *Playlist) isLastSequential() bool {
	return p.current >= len(p.tracks)-1
}

func (p *Playlist) isValidIndex(index int) bool {
	return index >= 0 && index < len(p.tracks)
}

func (p *Playlist) regenerateShuffle() {
	n := len(p.tracks)
	if n == 0 {
		return
	}
	p.shuffleOrder = make([]int, n)
	for i := 0; i < n; i++ {
		p.shuffleOrder[i] = i
	}
	for i := n - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		p.shuffleOrder[i], p.shuffleOrder[j] = p.shuffleOrder[j], p.shuffleOrder[i]
	}
	p.shuffleIdx = p.findInShuffleOrder(p.current)
	p.shuffleStartIdx = p.shuffleIdx
}

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

func (p *Playlist) findInShuffleOrder(trackIndex int) int {
	for i, idx := range p.shuffleOrder {
		if idx == trackIndex {
			return i
		}
	}
	return 0
}

// ═══════════════════════════════════════════════════════════════════════════════
// INFRASTRUCTURE: AUDIO ENGINE
// ═══════════════════════════════════════════════════════════════════════════════

type Limiter struct {
	Streamer beep.Streamer
}

func (l *Limiter) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = l.Streamer.Stream(samples)
	for i := range samples[:n] {
		for ch := 0; ch < 2; ch++ { // Left and Right channels
			if samples[i][ch] > 1.0 {
				samples[i][ch] = 1.0
			} else if samples[i][ch] < -1.0 {
				samples[i][ch] = -1.0
			}
		}
	}
	return n, ok
}

func (l *Limiter) Err() error {
	if se, ok := l.Streamer.(interface{ Err() error }); ok {
		return se.Err()
	}
	return nil
}

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

	var streamer beep.StreamSeekCloser
	var format beep.Format
	ext := strings.ToLower(filepath.Ext(track.Path))

	switch ext {
	case ".mp3":
		streamer, format, err = mp3.Decode(file)
	case ".wav":
		streamer, format, err = wav.Decode(file)
	default:
		file.Close()
		return 0, fmt.Errorf("formato no soportado: %s", ext)
	}

	if err != nil {
		file.Close()
		return 0, fmt.Errorf("error al decodificar %s: %w", track.Path, err)
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
		Base:     math.Pow(10, 1.0/20.0), // Setup logarithmic volume control
		Volume:   0,
		Silent:   false,
	}

	return realDuration, nil
}

func (ae *AudioEngine) Play() chan struct{} {
	done := make(chan struct{})
	limiter := &Limiter{Streamer: ae.volume}

	speaker.Play(beep.Seq(limiter, beep.Callback(func() {
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

	samples := int(position.Seconds()) * int(ae.format.SampleRate)
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
// APPLICATION: UI MODEL (Bubble Tea)
// ═══════════════════════════════════════════════════════════════════════════════

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
	errorMsg struct{ err error }
)

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

	// Dynamic directory loading features
	musicDir        string
	isPickingFolder bool
	browserPath     string
	browserEntries  []browserEntry
	browserCursor   int
}

// browserEntry represents a single row in the visual directory picker: either
// a navigable subdirectory or an audio file (.mp3 / .wav) shown for context.
type browserEntry struct {
	name  string
	isDir bool
}

func NewAppModel(initialDir string) AppModel {
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
		musicDir:    initialDir,
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(m.scanLibraryCmd(m.musicDir), m.tick())
}

func (m AppModel) tick() tea.Cmd {
	return tea.Tick(time.Millisecond*250, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// scanLibraryCmd handles both default fallback scanning and explicit directory targeting.
func (m AppModel) scanLibraryCmd(targetDir string) tea.Cmd {
	return func() tea.Msg {
		var dirs []string

		// Fallback scanning logic if no explicit directory was specified
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

			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}

			// If successfully resolved a directory in fallback mode, register it as the target
			if targetDir == "" {
				targetDir = dir
			}

			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if ext != ".mp3" && ext != ".wav" {
					continue
				}

				found = append(found, Track{
					ID:     fmt.Sprintf("track-%d", len(found)),
					Title:  strings.TrimSuffix(entry.Name(), ext),
					Artist: "Desconocido",
					Path:   filepath.Join(dir, entry.Name()),
				})
			}

			// Stop searching other fallback directories if we successfully found files in one
			if len(found) > 0 {
				break
			}
		}

		if len(found) == 0 {
			return errorMsg{fmt.Errorf("no se encontraron archivos de audio en: %s", targetDir)}
		}

		return libraryScannedMsg{tracks: found, dir: targetDir}
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

func (m *AppModel) loadBrowserDir(target string) {
	entries, err := os.ReadDir(target)
	if err != nil {
		m.lastError = err
		return
	}

	m.browserPath = target

	var dirs, files []browserEntry
	for _, e := range entries {
		name := e.Name()
		isDir := e.IsDir()

		// e.IsDir() reflects the raw directory-entry type returned by the OS
		// and does NOT follow symlinks, so a symlink pointing at a directory
		// is reported as a non-directory and would otherwise vanish from the
		// picker. Resolve the entry with os.Stat (which follows symlinks) to
		// get an accurate answer for symlinks and other edge cases.
		if e.Type()&os.ModeSymlink != 0 {
			if info, statErr := os.Stat(filepath.Join(target, name)); statErr == nil {
				isDir = info.IsDir()
			} else {
				// Broken symlink or no permission to resolve it: skip just
				// this one entry rather than failing the whole directory.
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

	// Directories first (for navigation), then audio files (for context on
	// what tracks live in the current folder).
	m.browserEntries = append(dirs, files...)
	m.browserCursor = 0
}

// ═══════════════════════════════════════════════════════════════════════════════
// APPLICATION: STATE UPDATE (Update)
// ═══════════════════════════════════════════════════════════════════════════════

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.progressBar.Width = int(math.Max(float64(msg.Width)*0.7, 30))
		return m, nil

	case tea.KeyMsg:
		// Divert keystrokes to the visual directory picker when active
		if m.isPickingFolder {
			return m.handleBrowserInput(msg)
		}
		return m.handleKeyInput(msg)

	case libraryScannedMsg:
		// Reset the player state for the new directory
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
		return m, nil

	case trackLoadedMsg:
		return m.handleTrackLoaded(msg)

	case tickMsg:
		if m.state == StatePlaying {
			m.elapsed = m.audio.Position()
			if m.elapsed > m.totalTime {
				m.elapsed = m.totalTime
			}
			return m, m.tick()
		}
		return m, nil

	case playbackEndedMsg:
		if msg.sessionID != m.audio.sessionID {
			return m, nil
		}
		if m.playlist.isLast() && m.playlist.repeat == RepeatOff {
			m.state, m.elapsed = StateStopped, 0
			m.audio.Stop()
			return m, nil
		}
		return m.playNext()

	case errorMsg:
		m.lastError = msg.err
		// Clear the error message after 5 seconds via a delayed command
		return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg { return errorMsg{nil} })
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
		if m.browserCursor > 0 {
			m.browserCursor--
		}
	case "down", "j":
		if m.browserCursor < len(m.browserEntries)-1 {
			m.browserCursor++
		}
	case "left", "backspace":
		parent := filepath.Dir(m.browserPath)
		m.loadBrowserDir(parent)
	case "right", "enter":
		if len(m.browserEntries) > 0 {
			selected := m.browserEntries[m.browserCursor]
			if selected.isDir {
				newPath := filepath.Join(m.browserPath, selected.name)
				m.loadBrowserDir(newPath)
			}
		}
	case " ":
		// Confirm the currently highlighted subfolder as the new music
		// directory. If an audio file (not a folder) is highlighted, or the
		// listing is empty, confirm the directory currently being browsed.
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

func (m AppModel) handleKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.audio.Close()
		return m, tea.Quit
	case "o", "ctrl+o":
		m.isPickingFolder = true
		initialDir := m.musicDir
		if initialDir == "" {
			initialDir = "."
		}
		m.loadBrowserDir(initialDir)
		return m, nil
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
		m.playlist.ToggleShuffle()
	case "r":
		m.playlist.repeat = (m.playlist.repeat + 1) % 3
	case "l":
		m.showQueue = !m.showQueue
	case "h", "?":
		m.showHelp = !m.showHelp
	}
	return m, nil
}

func (m AppModel) handleTrackLoaded(msg trackLoadedMsg) (tea.Model, tea.Cmd) {
	m.audio.sessionID++
	sessionID := m.audio.sessionID
	m.audio.cancelChan = make(chan struct{})

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
		case <-m.audio.cancelChan:
			return nil
		}
	}
	return m, tea.Batch(m.tick(), waitCmd)
}

// ═══════════════════════════════════════════════════════════════════════════════
// APPLICATION: PLAYER ACTIONS
// ═══════════════════════════════════════════════════════════════════════════════

func (m AppModel) togglePlayback() (tea.Model, tea.Cmd) {
	switch m.state {
	case StatePlaying:
		m.audio.ctrl.Paused, m.state = true, StatePaused
	case StatePaused:
		m.audio.ctrl.Paused, m.state = false, StatePlaying
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
		m.seekTo(0)
		return m, nil
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
	m.cursorIndex = int(math.Max(0, math.Min(float64(m.cursorIndex+delta), float64(m.playlist.Length()-1))))
}

func (m *AppModel) adjustVolume(delta float64) {
	m.volumeLevel = math.Max(minVolume, math.Min(maxVolume, m.volumeLevel+delta))
	m.audio.SetVolume(m.volumeLevel)
}

func (m *AppModel) seekTo(position time.Duration) {
	if err := m.audio.Seek(position); err != nil {
		m.lastError = err
		return
	}
	m.elapsed = clampDuration(position, 0, m.totalTime)
}

func (m *AppModel) seekForward(seconds int) { m.seekTo(m.elapsed + time.Duration(seconds)*time.Second) }

func (m *AppModel) seekBackward(seconds int) {
	m.seekTo(m.elapsed - time.Duration(seconds)*time.Second)
}

func (m *AppModel) resetPlayback() {
	m.audio.Stop()
	m.state, m.elapsed, m.totalTime = StateStopped, 0, 0
}

// ═══════════════════════════════════════════════════════════════════════════════
// APPLICATION: RENDER LOOP (View)
// ═══════════════════════════════════════════════════════════════════════════════

func (m AppModel) View() string {
	if m.lastError != nil {
		return m.renderErrorScreen()
	}

	sections := []string{m.renderHeader(), m.renderNowPlayingPanel(), ""}

	if m.isPickingFolder {
		sections = append(sections, m.renderBrowserPanel())
	} else {
		if m.showQueue {
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
		content.WriteString(m.renderStatusLine(track) + "\n" + m.renderProgressBar() + "\n" + m.renderMetadataLine())
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

	volBar := renderVolumeBar(m.volumeLevel, minVolume, maxVolume, m.audio.IsMuted())
	return lipgloss.NewStyle().Foreground(comment).MarginLeft(2).Render(
		fmt.Sprintf("%s  |  %s %s  |%s| %s ", volBar, iconQueue, queueDisplay, shuffleIcon, m.playlist.repeat.Icon()),
	)
}

func (m AppModel) renderPlaylistPanel() string {
	if m.playlist.IsEmpty() {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(orange).Render("  " + iconQueue + " Próximamente:\n"))

	start := int(math.Max(0, float64(m.cursorIndex-3)))
	end := int(math.Min(float64(m.playlist.Length()), float64(start+7)))

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
		if i == m.playlist.current {
			style = style.Bold(true).Foreground(pink)
		} else if i == m.cursorIndex {
			style = style.Foreground(cyan)
		} else {
			style = style.Foreground(foreground)
		}

		row := fmt.Sprintf("%s%d. %-35s [%s]", cursor, i+1, truncate(t.DisplayName(), 35), t.FormattedDuration())
		builder.WriteString("  " + style.Render(row) + "\n")
	}

	if end < m.playlist.Length() {
		builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("    " + iconDown + " ...\n"))
	}

	return builder.String()
}

func (m AppModel) renderBrowserPanel() string {
	header := lipgloss.NewStyle().Bold(true).Foreground(cyan).Render(iconFolder + " Explorador de Carpetas: " + m.browserPath)

	var builder strings.Builder
	builder.WriteString(header + "\n")
	builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("  ←/Retroceso: Subir | →/Enter: Entrar | Espacio: Confirmar Directorio | Esc: Cancelar\n\n"))

	if len(m.browserEntries) == 0 {
		builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("  (Directorio vacío, sin subcarpetas ni pistas .mp3/.wav)\n"))
	}

	start := int(math.Max(0, float64(m.browserCursor-5)))
	end := int(math.Min(float64(len(m.browserEntries)), float64(start+10)))

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

		builder.WriteString(style.Render(cursor+icon+" "+entry.name) + "\n")
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
		{iconNav + " Navegación", []binding{{"↑↓ / jk", "Mover cursor"}, {"enter", "Reproducir"}, {"d", "Eliminar de cola"}, {"l", "Ocultar cola"}, {"o", "Explorar carpetas"}}},
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
		end := int(math.Min(float64(i+colsPerRow), float64(len(blocks))))
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

// ═══════════════════════════════════════════════════════════════════════════════
// UTILITIES
// ═══════════════════════════════════════════════════════════════════════════════

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	return fmt.Sprintf("%02d:%02d", d/time.Minute, (d%time.Minute)/time.Second)
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
	runes := []rune(str)
	if len(runes) > max {
		return string(runes[:max-3]) + "..."
	}
	return str
}

func renderVolumeBar(level, min, max float64, muted bool) string {
	if muted {
		return lipgloss.NewStyle().Foreground(red).Bold(true).Render(iconMute + " MUTE")
	}
	if max == min {
		return fmt.Sprintf("%s ░░░░░░░░░░ 0%%", iconVolume)
	}

	pct := math.Max(0, math.Min(1, (level-min)/(max-min)))
	filled := int(pct * 10)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)

	color := green
	if pct > 0.85 {
		color = orange
	} else if pct > 0.5 {
		color = yellow
	}
	return lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf("%s %s %3.0f%%", iconVolume, bar, pct*100))
}

// ═══════════════════════════════════════════════════════════════════════════════
// MAIN
// ═══════════════════════════════════════════════════════════════════════════════

func main() {
	// Expose a command line flag to bypass the default scanning paths
	dirFlag := flag.String("dir", "", "Directorio inicial de música a escanear")
	flag.Parse()

	if _, err := tea.NewProgram(NewAppModel(*dirFlag), tea.WithAltScreen()).Run(); err != nil {
		fmt.Printf("Error al iniciar el reproductor: %v\n", err)
		os.Exit(1)
	}
}
