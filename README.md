# GoPlayer

[Read in Spanish](readme.es.md)

> A feature-rich, terminal-based User Interface (TUI) music player written in Go, powered by **Bubble Tea**, **Lipgloss**, and **Beep**.

GoPlayer brings a modern, sleek audio playback experience directly to your terminal. Built following **The Elm Architecture** (Model-View-Update), it provides smooth navigation, dynamic directory browsing, volume controls, customizable playback modes, and Dracula-themed visual styling.

---

## Key Features

- **Terminal User Interface (TUI)**: Beautiful and responsive terminal interface with custom status indicators, progress bars, and track controls.
- **Dynamic File Explorer**: Built-in visual directory picker (`o` / `ctrl+o`) allowing you to browse subfolders and change your music directory on the fly.
- **Multiple Format Support**: Decodes and plays **MP3**, **WAV**, **FLAC**, and **OGG** audio files seamlessly.
- **Audio Control & Engine**:
  - Fine-grained volume control with logarithmic scaling, limiter effect, and instant muting.
  - Seeking capability (jump forward/backward by configurable intervals).
- **Playlist & Queue Management**:
  - Interactive playlist displaying current, upcoming, and previous tracks.
  - Ability to remove individual tracks from the active session.
- **Metadata Extraction**: Automatically extracts **Title**, **Artist**, and **Album** metadata from audio tags.
- **Quick Filter**: Press `Ctrl+F` to open an inline fuzzy/substring search bar that filters the playlist by Title or Artist in real time. Navigate matches with ↑/↓ or j/k, press `Enter` to play the highlighted track, and `Esc` to clear/close the filter.

- **Playback Modes**:
  - **Shuffle**: Randomized playlist order.
  - **Repeat Modes**: Repeat Off, Repeat One (single track), or Repeat All (entire playlist).
- **Graceful Fallbacks & Auto-Scanning**: Automatically scans local folders (`./music`, `./songs`, `~/Music`, `~/Música`) or accepts a custom command-line directory flag.

---

## Project Structure

```text
.
├── cmd
│   └── goplayer
│       └── main.go       # Entry point
└── internal
    └── player
        ├── app.go        # UI/Application model
        ├── audio.go      # Audio engine
        ├── constants.go  # Constants & Styles
        ├── models.go     # Data structures
        └── utils.go      # Helper functions
```

---

## Tech Stack & Architecture

- **Language**: [Go (Golang)](https://golang.org/)
- **Architecture**: The Elm Architecture / Model-View-Update (MVU)
- **Frameworks & Libraries**:
  - [Charm Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework based on Elm.
  - [Charm Lipgloss](https://github.com/charmbracelet/lipgloss) — Style definitions and terminal layouts.
  - [Charm Bubbles Progress](https://github.com/charmbracelet/bubbles) — Progress bar component.
  - [Faiface Beep](https://github.com/faiface/beep) — Audio library for Go (decoding, resampling, volume control, and audio playback).

---

## Prerequisites

Before running GoPlayer, ensure you have the following installed on your system:

- **Go** (version 1.22 or higher recommended for modern range syntax)


## Installation & Setup

1. **Clone the repository**:
   ```bash
   git clone https://github.com/fonta81/Reproductor-go.git
   cd Reproductor-go
   ```

2. **Install Go dependencies**:
   ```bash
   go mod tidy
   ```
3. **Run the code**:

   ```bash
   go run ./cmd/goplayer
   ```

---

## Usage

### Default Execution
By default, GoPlayer scans `./music`, `./songs`, and your system's default Music folder for `.mp3`, `.wav`, `.flac`, and `.ogg` files. If you previously selected a directory via the file browser (`o`), GoPlayer will automatically load it on startup from its configuration file (`~/.config/goplayer/config.json`):

```bash
go run ./cmd/goplayer
```

### Specify a Custom Music Directory
Pass the `-dir` flag to target a specific folder at startup:

```bash
go run ./cmd/goplayer -dir /path/to/your/music
```

---

## Keybindings & Controls

### Playback
| Key | Action |
| :--- | :--- |
| `Space` | Toggle Play / Pause |
| `n` | Next Track |
| `N` | Previous Track (or restart current track if elapsed > 3s) |
| `.` or `>` | Seek forward (10 seconds) |
| `,` or `<` | Seek backward (10 seconds) |
| `0` | Restart current track |

### Navigation & Queue
| Key | Action |
| :--- | :--- |
| `↑` / `k` | Move selection cursor up |
| `↓` / `j` | Move selection cursor down |
| `Enter` | Play selected track |
| `d` | Remove selected track from queue |
| `l` | Toggle visibility of queue panel |
| `o` / `Ctrl+O` | Open visual directory browser |
| `Ctrl+F` | Open quick fuzzy/substring filter (search Title or Artist) |

### Audio & Modes
| Key | Action |
| :--- | :--- |
| `+` / `=` | Increase volume |
| `-` | Decrease volume |
| `m` | Toggle Mute |
| `s` | Toggle Shuffle mode |
| `r` | Cycle Repeat mode (Off → One → All) |

### Directory Browser (Active Mode)
| Key | Action |
| :--- | :--- |
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `←` / `Backspace` | Navigate to parent directory |
| `→` / `Enter` | Open selected directory |
| `Space` | Confirm and scan selected directory |
| `Esc` / `q` | Cancel directory selection |

### System
| Key | Action |
| :--- | :--- |
| `h` or `?` | Toggle Help panel visibility |
| `q` or `Ctrl+C` | Quit GoPlayer |

---

## License

This project is licensed under the MIT License.
