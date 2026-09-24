# GoPlayer

[Read in Spanish](readme.es.md)

> A feature-rich, terminal-based User Interface (TUI) music player written in Go, powered by **Bubble Tea**, **Lipgloss**, and **Beep**.

GoPlayer brings a modern, sleek audio playback experience directly to your terminal. Built following **The Elm Architecture** (Model-View-Update), it provides smooth navigation, dynamic directory browsing, volume controls, customizable playback modes, and Dracula-themed visual styling.

---

## Key Features

- **Terminal User Interface (TUI)**: Beautiful and responsive terminal interface with custom status indicators, progress bars, and track controls.
- **Interactive Main Menu**: Intuitive main menu to trigger actions (Play track, Pause/Stop, Next/Previous song, Music library, Settings, Quit).
- **Dynamic File Explorer**: Built-in visual directory picker (`o` / `ctrl+o` or via menu) allowing you to browse subfolders and change your music directory on the fly (confined securely to your user home directory).
- **Persistent Configuration**: Automatically remembers the last selected music folder in `~/.config/goplayer/config.json`.
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
- **Graceful Fallbacks & Auto-Scanning**: Automatically scans local folders (`./music`, `./songs`, `~/Music`, `~/Música`), user config, or accepts a custom command-line directory flag.

---

## Project Structure

```text
.
├── cmd
│   └── goplayer
│       └── main.go       # Entry point
├── internal
│   └── player
│       ├── app.go        # UI/Application model for the player
│       ├── audio.go      # Audio engine (Beep)
│       ├── config.go     # Persistent configuration management
│       ├── constants.go  # Colors, constants & styles
│       ├── models.go     # Data structures (Track, Playlist, etc.)
│       ├── root.go       # Screen router (Menu <-> Player)
│       └── utils.go      # Helper functions & metadata extraction
├── menu
│   └── menu.go           # Main menu TUI model
├── flake.nix             # Nix development environment flake
└── flake.lock
```

---

## Tech Stack & Architecture

- **Language**: [Go (Golang)](https://golang.org/)
- **Architecture**: The Elm Architecture / Model-View-Update (MVU)
- **Frameworks & Libraries**:
  - [Charm Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework based on Elm.
  - [Charm Lipgloss](https://github.com/charmbracelet/lipgloss) — Style definitions and terminal layouts.
  - [Charm Bubbles Progress & TextInput](https://github.com/charmbracelet/bubbles) — Progress bar and input components.
  - [Faiface Beep](https://github.com/faiface/beep) — Audio library for Go (decoding, resampling, volume control, and audio playback).
- **Development Environment**: [Nix Flakes](https://nixos.wiki/wiki/Flakes) (provides Go, gopls, pkg-config, alsa-lib).

---

## Prerequisites

Before running GoPlayer, ensure you have the following installed on your system:

- **Go** (version 1.22 or higher recommended for modern range syntax)
- System audio development libraries (e.g. `alsa-lib` / `pkg-config` on Linux if building with CGO audio backends). Alternatively, use **Nix**.

---

## Installation & Setup

### Standard Setup

1. **Clone the repository**:
   ```bash
   git clone https://github.com/fonta81/Reproductor-go.git
   cd Reproductor-go
   ```

2. **Install Go dependencies**:
   ```bash
   go mod tidy
   ```

3. **Run the application**:
   ```bash
   go run ./cmd/goplayer
   ```

### Using Nix Flake

If you use Nix with flakes enabled:

```bash
nix develop
go run ./cmd/goplayer
```

---

## Usage

### Default Execution
When started, GoPlayer displays the main menu. It scans `./music`, `./songs`, the system's default Music folder, or the directory saved in `~/.config/goplayer/config.json`:

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

### Main Menu
| Key | Action |
| :--- | :--- |
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `Enter` / `Space` | Select menu option |
| `q` / `Ctrl+C` | Quit |

### Playback Screen
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
| `Esc` | Return to Main Menu (when not in filter or directory browser) |

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
| `←` / `Backspace` | Navigate to parent directory (within `$HOME`) |
| `→` / `Enter` | Open selected directory |
| `Space` | Confirm and scan selected directory |
| `Esc` / `q` | Cancel directory selection |

### Quick Filter (Active Mode)
| Key | Action |
| :--- | :--- |
| `↑` / `k` | Move selection up in suggestions / matches |
| `↓` / `j` | Move selection down in suggestions / matches |
| `Enter` | Play selected matching track and exit filter |
| `Esc` | Cancel and close filter |

### System
| Key | Action |
| :--- | :--- |
| `h` or `?` | Toggle Help panel visibility |
| `q` or `Ctrl+C` | Quit GoPlayer |

---

## License

This project is licensed under the MIT License.
