#  Reproductor-go



##  Features

-  **Play MP3 & WAV** — Automatic format detection and decoding
-  **Resampling** — All audio files are resampled to a fixed sample rate (44100 Hz) so they always play at the correct speed
-  **Playback Controls** — Play, pause, next, previous, seek forward/backward
-  **Queue Management** — Navigate, select, and remove songs from the playlist
-  **Repeat Modes** — None, repeat one, repeat all
-  **Shuffle Mode** — Random playback order
-  **Volume & Mute** — Adjust volume with `+`/`-` or mute with `m`
-  **Auto-Scan** — Automatically scans `./music/`, `./songs/`, `~/Music/`, and `~/Música/` for audio files
-  **Beautiful TUI** — Dracula-themed interface built with Lipgloss
-  **Session-Safe** — Changing songs does not crash the player; old playback sessions are cleanly cancelled

---

##  Installation

### Prerequisites

- [Go](https://go.dev/dl/) 1.21 or higher
- Audio files (`.mp3` or `.wav`) in a music folder

### Clone & Build

```bash
# Clone the repository
git clone https://github.com/yourusername/goplayer.git
cd goplayer

# Download dependencies
go mod tidy

# Build the binary
go build -o goplayer main.go

# Or run directly
go run main.go
```

### Dependencies

```bash
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/bubbles
go get github.com/charmbracelet/lipgloss
go get github.com/faiface/beep
go get github.com/faiface/beep/mp3
go get github.com/faiface/beep/wav
go get github.com/faiface/beep/speaker
go get github.com/faiface/beep/effects
```

---

##  Music Directory

GoPlayer automatically scans the following directories for `.mp3` and `.wav` files:

| Priority | Path | Description |
|----------|------|-------------|
| 1 | `./music/` | Local `music` folder in the project directory |
| 2 | `./songs/` | Local `songs` folder |
| 3 | `~/Music/` | Standard user music folder |
| 4 | `~/Música/` | Spanish/Portuguese user music folder |

> **Tip:** Create a `music` folder in the same directory as the binary and drop your audio files there.

---

##  Controls

| Key | Action |
|-----|--------|
| `Space` | Play / Pause |
| `n` | Next song |
| `N` (Shift+N) | Previous song |
| `↑` / `↓` or `j` / `k` | Navigate playlist |
| `Enter` | Play selected song |
| `d` | Remove song from queue |
| `+` / `-` | Volume up / down |
| `m` | Toggle mute |
| `>` / `<` | Seek forward / backward 10 seconds |
| `0` | Restart current song |
| `r` | Cycle repeat mode (None → One → All) |
| `s` | Toggle shuffle mode |
| `h` / `?` | Toggle help panel |
| `q` | Quit |

---

##  License

![License](https://img.shields.io/badge/License-MIT-green)
