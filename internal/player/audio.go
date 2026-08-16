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
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	"github.com/faiface/beep/wav"
)

// Limiter limita los niveles de audio para prevenir saturación (clipping).
type Limiter struct {
	Streamer beep.Streamer
}

// Stream implementa la interfaz beep.Streamer aplicando un limitador a los samples.
func (l *Limiter) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = l.Streamer.Stream(samples)
	for i := range samples[:n] {
		for ch := 0; ch < 2; ch++ { // Canales Izquierdo y Derecho
			if samples[i][ch] > 1.0 {
				samples[i][ch] = 1.0
			} else if samples[i][ch] < -1.0 {
				samples[i][ch] = -1.0
			}
		}
	}
	return n, ok
}

// Err devuelve el error de la interfaz Streamer subyacente.
func (l *Limiter) Err() error {
	if se, ok := l.Streamer.(interface{ Err() error }); ok {
		return se.Err()
	}
	return nil
}

// AudioEngine gestiona la reproducción de archivos de audio.
type AudioEngine struct {
	streamer   beep.StreamSeekCloser
	ctrl       *beep.Ctrl
	volume     *effects.Volume
	format     beep.Format
	isInit     bool
	sessionID  int
	cancelChan chan struct{}
}

// NewAudioEngine crea una nueva instancia del motor de audio.
func NewAudioEngine() *AudioEngine {
	return &AudioEngine{}
}

// Load carga un archivo de audio para su reproducción.
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
	case ".flac":
		streamer, format, err = flac.Decode(file)
	case ".ogg":
		streamer, format, err = vorbis.Decode(file)
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
		Base:     math.Pow(10, 1.0/20.0), // Control de volumen logarítmico
		Volume:   0,
		Silent:   false,
	}

	return realDuration, nil
}

// Play inicia la reproducción del stream actual y devuelve un canal que se cierra al finalizar.
func (ae *AudioEngine) Play() chan struct{} {
	done := make(chan struct{})
	limiter := &Limiter{Streamer: ae.volume}

	speaker.Play(beep.Seq(limiter, beep.Callback(func() {
		close(done)
	})))
	return done
}

// Stop detiene la reproducción actual y limpia los recursos del motor de audio.
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
	speaker.Lock()
	ae.ctrl = nil
	ae.volume = nil
	speaker.Unlock()
}

// Pause pausa la reproducción actual de forma segura.
func (ae *AudioEngine) Pause() {
	speaker.Lock()
	defer speaker.Unlock()
	if ae.ctrl == nil {
		return
	}
	ae.ctrl.Paused = true
}

// Resume reanuda la reproducción previamente pausada de forma segura.
func (ae *AudioEngine) Resume() {
	speaker.Lock()
	defer speaker.Unlock()
	if ae.ctrl == nil {
		return
	}
	ae.ctrl.Paused = false
}

// SetVolume ajusta el volumen del motor de audio de forma segura.
func (ae *AudioEngine) SetVolume(level float64) {
	speaker.Lock()
	defer speaker.Unlock()
	if ae.volume == nil {
		return
	}
	ae.volume.Volume = level
}

// ToggleMute alterna el estado de silencio de forma segura.
func (ae *AudioEngine) ToggleMute() {
	speaker.Lock()
	defer speaker.Unlock()
	if ae.volume == nil {
		return
	}
	ae.volume.Silent = !ae.volume.Silent
}

// IsMuted devuelve verdadero si el audio está silenciado de forma segura.
func (ae *AudioEngine) IsMuted() bool {
	speaker.Lock()
	defer speaker.Unlock()
	return ae.volume != nil && ae.volume.Silent
}

// Position devuelve la posición actual de reproducción de forma segura.
func (ae *AudioEngine) Position() time.Duration {
	speaker.Lock()
	defer speaker.Unlock()
	if ae.streamer != nil {
		return ae.format.SampleRate.D(ae.streamer.Position())
	}
	return 0
}

// Seek mueve la posición de reproducción a una duración específica de forma segura.
func (ae *AudioEngine) Seek(position time.Duration) error {
	speaker.Lock()
	defer speaker.Unlock()
	if ae.streamer == nil {
		return fmt.Errorf("no hay stream activo")
	}

	seeker, ok := ae.streamer.(beep.StreamSeeker)
	if !ok {
		return fmt.Errorf("formato no permite seek")
	}

	samples := ae.format.SampleRate.N(position)
	if err := seeker.Seek(samples); err != nil {
		return fmt.Errorf("seek fallido: %w", err)
	}
	return nil
}

// Close cierra el motor de audio liberando todos los recursos.
func (ae *AudioEngine) Close() {
	ae.Stop()
	if ae.isInit {
		speaker.Close()
	}
}
