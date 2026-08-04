package player

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
)

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
		_ = file.Close()
		return 0, fmt.Errorf("formato no soportado: %s", ext)
	}

	if err != nil {
		_ = file.Close()
		return 0, fmt.Errorf("error al decodificar %s: %w", track.Path, err)
	}

	realDuration := format.SampleRate.D(streamer.Len())

	if !ae.isInit {
		if err := speaker.Init(standardSampleRate, standardSampleRate.N(time.Second/10)); err != nil {
			_ = streamer.Close()
			_ = file.Close()
			return 0, fmt.Errorf("error al inicializar speaker: %w", err)
		}
		ae.isInit = true
	}

	ae.streamer = streamer
	ae.format = format

	resampled := beep.Resample(4, format.SampleRate, standardSampleRate, streamer)

	ae.ctrl = &beep.Ctrl{Streamer: resampled}
	ae.volume = &effects.Volume{
		Streamer: ae.ctrl,
		Base:     math.Pow(10, 1.0/20.0), // Logarithmic volume control
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
		_ = ae.streamer.Close()
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
