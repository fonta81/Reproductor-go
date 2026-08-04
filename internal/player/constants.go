package player

import (
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/faiface/beep"
)

const (
	appName         = "GoPlayer"
	appSubtitle     = "Reproductor de música TUI"
	defaultDir      = "./music" // Default directory to scan for music files
	seekSeconds     = 10        // Number of seconds to jump forward/backward
	progressWidth   = 50
	defaultDuration = 3 * time.Minute // Fallback duration if metadata is missing

	standardSampleRate = beep.SampleRate(44100)
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
