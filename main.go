/*
================================================================================
GOPLAYER - TERMINAL USER INTERFACE (TUI) MUSIC PLAYER
================================================================================

PURPOSE & ARCHITECTURE:
This program is a fully functional music player built for the terminal. It uses
The Elm Architecture (Model-View-Update or MVU) implemented via the Bubble Tea framework.
The architecture strictly separates state (Model), logic/events (Update), and
rendering (View).

BASIC FLOW:
1. Init(): Fires off initial commands (scanning the library, starting the UI tick timer).
2. Update(): Listens for messages (tea.Msg) such as keystrokes, tick events, or audio
   state changes. It returns an updated Model and potentially new commands (tea.Cmd).
3. View(): Takes the current Model and purely renders it to a string using Lipgloss,
   which is then painted to the terminal.

KEY EXTERNAL LIBRARIES:
- github.com/charmbracelet/bubbletea: The core TUI framework (MVU architecture).
- github.com/charmbracelet/bubbles: Pre-built TUI components (used here for the progress bar).
- github.com/charmbracelet/lipgloss: A styling library that acts like CSS for the
  terminal (colors, margins, borders, layouts).
- github.com/faiface/beep: The core audio engine used to decode and play .mp3/.wav files.
================================================================================
*/

package main

import (
	"fmt"
	"math"
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
// CONFIGURATION & CONSTANTS
// ═══════════════════════════════════════════════════════════════════════════════

const (
	appName         = "GoPlayer"
	appSubtitle     = "Reproductor de música TUI"
	defaultDir      = "./music" // Default directory to scan for music files
	seekSeconds     = 10        // Number of seconds to jump forward/backward
	progressWidth   = 50
	defaultDuration = 3 * time.Minute // Fallback duration if metadata is missing

	// standardSampleRate sets a fixed playback frequency.
	// Resampling all audio to this rate prevents playback speed issues (Chipmunk/Darth Vader effects)
	// when playing tracks with varying original sample rates.
	standardSampleRate = beep.SampleRate(44100)
)

// ── Nerd Font Icons (Material Design) ────────────────────────────────────────
// These require a patched Nerd Font installed in the user's terminal to render correctly.
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
)

// ── Volume Control ───────────────────────────────────────────────────────────
const (
	volumeStep = 3.0   // 3 dB step represents a clearly perceptible change in volume.
	maxVolume  = 0.0   // 0 dBFS is the maximum digital volume before clipping occurs.
	minVolume  = -30.0 // -30 dB is considered the volume floor (virtually silent).
)

// Lipgloss Color Palette: Dracula Theme
// Lipgloss allows defining reusable, CSS-like styles for terminal rendering.
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

	// Pre-compiled Lipgloss styles for the Help Menu to save on render performance.
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

// PlaybackState defines the current transport state of the audio engine.
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

// Track represents a single audio file and its metadata.
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

// Playlist manages the queue of tracks.
// Note on Idiomatic Go: Pointer receivers (e.g., `func (p *Playlist)`) are used here
// because these methods mutate the playlist's internal state (tracks, indices).
type Playlist struct {
	tracks          []Track
	current         int // Index of the currently playing track in the tracks slice
	shuffle         bool
	repeat          RepeatMode
	shuffleOrder    []int // A randomized permutation of track indices for the shuffle state
	shuffleIdx      int   // Current position navigating through the shuffleOrder slice
	shuffleStartIdx int   // Remembers where the shuffle cycle began to detect when a full loop finishes
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

	// Idiomatic Go slice removal: append elements before index with elements after index.
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

// Current returns the active track. The boolean indicates if a track actually exists.
func (p *Playlist) Current() (Track, bool) {
	if !p.isValidIndex(p.current) {
		return Track{}, false
	}
	return p.tracks[p.current], true
}

// Next calculates and returns the next track to play based on shuffle and repeat rules.
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

		// Stop if we completed a randomized cycle and repeat is off
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

// ── Shuffle Internals ────────────────────────────────────────────────────────

// regenerateShuffle uses the Fisher-Yates algorithm for unbiased, true-random shuffling.
// It shuffles indices rather than tracks to preserve the original playlist sequence.
func (p *Playlist) regenerateShuffle() {
	n := len(p.tracks)
	if n == 0 {
		return
	}

	p.shuffleOrder = make([]int, n)
	for i := 0; i < n; i++ {
		p.shuffleOrder[i] = i
	}

	// Fisher-Yates shuffle
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

// Limiter acts as a hard limiter, preventing audio samples from exceeding [-1.0, 1.0]
// to prevent digital distortion (clipping). Beep processes audio in float64 ranges.
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

// AudioEngine wraps the beep library components to manage playback, volume, and lifecycle.
type AudioEngine struct {
	streamer   beep.StreamSeekCloser // The decoded audio stream
	ctrl       *beep.Ctrl            // Controls pausing/resuming
	volume     *effects.Volume       // Controls gain (decibels)
	format     beep.Format           // Sample rate and channel info of the loaded file
	isInit     bool                  // Tracks if the OS speaker is initialized
	sessionID  int                   // Used to invalidate obsolete playbackEndedMsg events
	cancelChan chan struct{}         // Idiomatic Go channel for signaling goroutine cancellation
}

func NewAudioEngine() *AudioEngine {
	return &AudioEngine{}
}

// Load decodes audio files based on explicit extension checking for efficiency.
func (ae *AudioEngine) Load(track Track) (time.Duration, error) {
	ae.Stop() // Stop any current playback

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
		file.Close() // Ensure proper resource cleanup on failure
		return 0, fmt.Errorf("error al decodificar %s: %w", track.Path, err)
	}

	// Calculate absolute duration based on sample rate and total samples
	realDuration := format.SampleRate.D(streamer.Len())

	// Initialize speaker only once using the predefined standard rate
	if !ae.isInit {
		speaker.Init(standardSampleRate, standardSampleRate.N(time.Second/10))
		ae.isInit = true
	}

	ae.streamer = streamer
	ae.format = format

	// Resample the audio file to standardSampleRate so it plays at the correct speed
	resampled := beep.Resample(4, format.SampleRate, standardSampleRate, streamer)

	// Wrap the streamer in control wrappers: Ctrl (pause) -> Volume -> Streamer
	ae.ctrl = &beep.Ctrl{Streamer: resampled}
	ae.volume = &effects.Volume{
		Streamer: ae.ctrl,
		Base:     math.Pow(10, 1.0/20.0), // Setup logarithmic volume control
		Volume:   0,                      // 0 = no gain, max clean volume
		Silent:   false,
	}

	return realDuration, nil
}

// Play starts playback in a separate goroutine handled by `speaker.Play`.
// It returns a `chan struct{}` which is an idiomatic zero-memory channel used
// strictly as a signaling mechanism (to notify when the track finishes).
func (ae *AudioEngine) Play() chan struct{} {
	done := make(chan struct{})
	limiter := &Limiter{Streamer: ae.volume}

	// beep.Seq chains streamers. Here, it plays the song, then executes the Callback
	// which closes the `done` channel, notifying the main app that the song ended.
	speaker.Play(beep.Seq(limiter, beep.Callback(func() {
		close(done)
	})))
	return done
}

func (ae *AudioEngine) Stop() {
	speaker.Clear() // Immediately stops the OS audio output

	if ae.cancelChan != nil {
		close(ae.cancelChan) // Signal any waiting Bubble Tea commands to abort
		ae.cancelChan = nil
	}
	if ae.streamer != nil {
		ae.streamer.Close() // Release file handles
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

	// Convert requested time to exact number of samples
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

// ── Messages (tea.Msg) ───────────────────────────────────────────────────────
// Messages are discrete events that flow into the Update() function. They decouple
// the action's occurrence from the state mutation.
type (
	// trackLoadedMsg triggers after an audio file is successfully decoded and ready
	trackLoadedMsg struct {
		track    Track
		duration time.Duration
	}
	// playbackEndedMsg triggers when an audio stream hits EOF
	playbackEndedMsg struct{ sessionID int }
	// tickMsg triggers periodically to update the progress bar and timers
	tickMsg time.Time
	// libraryScannedMsg returns a slice of tracks found on the hard drive
	libraryScannedMsg struct{ tracks []Track }
	// errorMsg encapsulates fatal application errors for display
	errorMsg struct{ err error }
)

// AppModel holds the entire global state of the application.
// In Bubble Tea, the main model is usually passed by value to the Update and View functions.
// Note: `playlist` and `audio` are pointers. This is a pragmatic design choice: while `AppModel`
// is copied by value, it shares the underlying heavy structures (buffers, hardware handles)
// without copying them.
type AppModel struct {
	playlist *Playlist
	audio    *AudioEngine

	state       PlaybackState
	elapsed     time.Duration
	totalTime   time.Duration
	cursorIndex int // Highlights a track in the UI queue without playing it
	volumeLevel float64
	lastError   error

	width       int
	height      int
	progressBar progress.Model // A pre-built TUI component from charmbracelet/bubbles
	showHelp    bool
	showQueue   bool
}

// NewAppModel initializes the default state.
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

// ── 1. Init(): The Startup Function ──────────────────────────────────────────
// Init is called once when Bubble Tea starts. It returns an initial tea.Cmd to execute.
// tea.Batch runs multiple commands concurrently.
func (m AppModel) Init() tea.Cmd {
	return tea.Batch(m.scanLibrary(), m.tick())
}

// Commands (tea.Cmd): Functions that perform side effects (I/O, timers, decoding)
// asynchronously and return a tea.Msg when finished, which is fed back into Update().

// tick returns a command that waits 250ms, then emits a tickMsg.
func (m AppModel) tick() tea.Cmd {
	return tea.Tick(time.Millisecond*250, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// scanLibrary performs file I/O to find audio files. It returns a libraryScannedMsg.
func (m AppModel) scanLibrary() tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			home = ""
		}

		dirs := []string{defaultDir, "./songs"}
		if home != "" {
			dirs = append(dirs, filepath.Join(home, "Music"), filepath.Join(home, "Música"))
		}

		// Pre-allocate slice to reduce memory allocations during scanning loop.
		found := make([]Track, 0, 50)
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

				found = append(found, Track{
					ID:     fmt.Sprintf("track-%d", len(found)),
					Title:  strings.TrimSuffix(entry.Name(), ext),
					Artist: "Desconocido",
					Path:   filepath.Join(dir, entry.Name()),
				})
			}
		}
		return libraryScannedMsg{tracks: found}
	}
}

// loadTrackCmd attempts to decode a file via the audio engine. Returns trackLoadedMsg on success.
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
// APPLICATION: STATE UPDATE (Update)
// ═══════════════════════════════════════════════════════════════════════════════

// ── 2. Update(msg tea.Msg): The Event Loop ───────────────────────────────────
// This function handles the message-driven architecture. It receives messages,
// modifies the state (Model), and returns the new Model along with any subsequent Commands.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Idiomatic Go type assertion switch to handle different events dynamically.
	switch msg := msg.(type) {

	// Handles terminal window resizing
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Dynamically adjust the progress bar based on terminal width.
		m.progressBar.Width = int(math.Max(float64(msg.Width)*0.7, 30))
		return m, nil

	// Handles keyboard input
	case tea.KeyMsg:
		return m.handleKeyInput(msg)

	// Initial library payload injection
	case libraryScannedMsg:
		for _, track := range msg.tracks {
			m.playlist.Add(track)
		}
		if !m.playlist.IsEmpty() && m.state == StateStopped {
			m.playlist.current = 0
		}
		return m, nil // Returning nil command means we just update UI state

	// Proceed with playing a successfully loaded track
	case trackLoadedMsg:
		return m.handleTrackLoaded(msg)

	// The heartbeat timer for UI updates
	case tickMsg:
		if m.state == StatePlaying {
			m.elapsed = m.audio.Position() // Fetch exact hardware position
			if m.elapsed > m.totalTime {
				m.elapsed = m.totalTime // Prevent UI overflow
			}
			return m, m.tick() // Dispatch another timer command to keep the loop alive
		}
		return m, nil

	// Triggered when the audio file finishes naturally
	case playbackEndedMsg:
		// Stale event prevention: Ignore if the user manually changed tracks
		// before this event fired, determined by matching sessionIDs.
		if msg.sessionID != m.audio.sessionID {
			return m, nil
		}
		// If playlist finished and no repeat
		if m.playlist.isLast() && m.playlist.repeat == RepeatOff {
			m.state, m.elapsed = StateStopped, 0
			m.audio.Stop()
			return m, nil
		}
		return m.playNext()

	// Handle errors, display briefly, then attempt to recover
	case errorMsg:
		m.lastError, m.state = msg.err, StateStopped
		// Clear the error message after 5 seconds via a delayed command
		return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg { return errorMsg{nil} })
	default:
		return m, nil
	}
}

// Router for user key presses
func (m AppModel) handleKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.audio.Close()
		return m, tea.Quit // Special core command to terminate the application gracefully
	case " ":
		return m.togglePlayback()
	case "n":
		return m.playNext()
	case "N":
		return m.playPrevious()
	case "enter":
		return m.playSelected()
	case "up", "k": // vim motions for navigation
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
	m.audio.sessionID++ // Increment to invalidate previous session's events
	sessionID := m.audio.sessionID
	m.audio.cancelChan = make(chan struct{})

	// Starts playback hardware side, returns a channel that signals EOF
	done := m.audio.Play()
	m.state = StatePlaying
	m.totalTime = msg.duration
	if m.totalTime <= 0 {
		m.totalTime = defaultDuration
	}

	m.playlist.tracks[m.playlist.current].Duration = msg.duration
	m.elapsed = 0

	// waitCmd blocks in a background goroutine (managed by TEA) until playback ends
	// or the track is skipped/cancelled.
	waitCmd := func() tea.Msg {
		select {
		case <-done: // EOF reached
			return playbackEndedMsg{sessionID: sessionID}
		case <-m.audio.cancelChan: // User skipped the track; abort this tracker
			return nil
		}
	}
	// Return the timer tick AND the background tracker simultaneously
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
		return m, m.tick() // Restart the UI tick timer upon resume
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
	// Standard player UX: If past 3 seconds, previous simply restarts the track.
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

// ── 3. View(): The Rendering Pipeline ────────────────────────────────────────
// The View function returns a single string representing the entire terminal interface.
// State variables dictate what gets concatenated here.
// Lipgloss is heavily used to compose layouts (e.g., JoinVertical) and apply styles.
func (m AppModel) View() string {
	if m.lastError != nil {
		return m.renderErrorScreen()
	}

	// We gather "blocks" of UI components and join them vertically
	sections := []string{m.renderHeader(), m.renderNowPlayingPanel(), ""}
	if m.showQueue {
		sections = append(sections, m.renderPlaylistPanel())
	}
	if m.showHelp {
		sections = append(sections, "", m.renderHelpPanel())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m AppModel) renderHeader() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(pink).MarginLeft(2).MarginTop(1).Render(appName)
	return title + "  " + lipgloss.NewStyle().Foreground(comment).Render(appSubtitle)
}

func (m AppModel) renderNowPlayingPanel() string {
	track, hasTrack := m.playlist.Current()
	var content strings.Builder // High-performance string concatenation in Go

	if hasTrack {
		content.WriteString(m.renderStatusLine(track) + "\n" + m.renderProgressBar() + "\n" + m.renderMetadataLine())
	} else {
		content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(red).Render(iconStop + " Sin canciones\n"))
		content.WriteString(lipgloss.NewStyle().Foreground(comment).Render("Coloca archivos .mp3 o .wav en ./music/ o ~/Music/"))
	}

	// Box styling wrap
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(purple).Padding(1, 2).Margin(0, 2).Render(content.String())
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

	// Ask the pre-built Bubbles component to render itself based on the percentage
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

	// Calculation to keep the visible UI window focused around the cursor (pagination)
	start := int(math.Max(0, float64(m.cursorIndex-3)))
	end := int(math.Min(float64(m.playlist.Length()), float64(start+7)))

	if start > 0 {
		builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("    " + iconUp + " ...\n"))
	}

	for i := start; i < end; i++ {
		t := m.playlist.tracks[i]
		cursor := "   "
		if i == m.cursorIndex {
			cursor = "→  " // UI indicator for selection
		}

		style := lipgloss.NewStyle()
		if i == m.playlist.current {
			style = style.Bold(true).Foreground(pink)
		} else if i == m.cursorIndex {
			style = style.Foreground(cyan)
		} else {
			style = style.Foreground(foreground)
		}

		// %-35s left-aligns strings. Truncate protects layout structure.
		row := fmt.Sprintf("%s%d. %-35s [%s]", cursor, i+1, truncate(t.DisplayName(), 35), t.FormattedDuration())
		builder.WriteString("  " + style.Render(row) + "\n")
	}

	if end < m.playlist.Length() {
		builder.WriteString(lipgloss.NewStyle().Foreground(comment).Render("    " + iconDown + " ...\n"))
	}

	return builder.String()
}

func (m AppModel) renderHelpPanel() string {
	type binding struct{ key, desc string }
	type category struct {
		title    string
		bindings []binding
	}

	categories := []category{
		{iconPlay + " Reproducción", []binding{{"espacio", "Play / Pausa"}, {"n / N", "Sig / Anterior"}, {"> / <", "Adel. / Atrasar"}, {"0", "Reiniciar"}}},
		{iconNav + " Navegación", []binding{{"↑↓ / jk", "Mover cursor"}, {"enter", "Reproducir"}, {"d", "Eliminar de cola"}, {"l", "Ocultar cola"}}},
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
	// Responsive layout logic: changes columns based on terminal width
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
		fmt.Sprintf("CRITICAL ERROR:\n\n%v\n\nPresiona 'q' para salir o espera a recuperarte.", m.lastError),
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

// Truncate handles slicing strings safely based on Rune counts instead of Bytes
// preventing multi-byte characters (like emojis or accents) from corrupting the terminal view
// or panicking out of bounds.
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
	// Initialize the TUI program. tea.WithAltScreen() puts the app in a secondary
	// full-screen buffer, preserving the user's normal terminal shell history behind it.
	if _, err := tea.NewProgram(NewAppModel(), tea.WithAltScreen()).Run(); err != nil {
		fmt.Printf("Error al iniciar el reproductor: %v\n", err)
		os.Exit(1)
	}
}
