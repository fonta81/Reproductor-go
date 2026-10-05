// Package player proporciona la abstracción y control del motor de audio de bajo nivel
// sobre la biblioteca beep y el backend de sonido del sistema operativo.
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

// Limiter es un streamer envoltorio que acota las muestras de audio en el rango [-1.0, 1.0].
// Su propósito es prevenir la saturación acústica y distorsión por recorte (clipping)
// cuando se aplican ganancias altas de volumen.
type Limiter struct {
	Streamer beep.Streamer
}

// Stream procesa las muestras del streamer subyacente y restringe la amplitud de cada canal
// (estéreo: canal 0 izquierdo, canal 1 derecho) al intervalo [-1.0, 1.0].
func (l *Limiter) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = l.Streamer.Stream(samples)
	for i := range samples[:n] {
		for ch := 0; ch < 2; ch++ { // Canales Izquierdo (0) y Derecho (1)
			if samples[i][ch] > 1.0 {
				samples[i][ch] = 1.0
			} else if samples[i][ch] < -1.0 {
				samples[i][ch] = -1.0
			}
		}
	}
	return n, ok
}

// Err propaga cualquier error generado por el Streamer subyacente si este implementa
// la interfaz de comprobación de errores.
func (l *Limiter) Err() error {
	if se, ok := l.Streamer.(interface{ Err() error }); ok {
		return se.Err()
	}
	return nil
}

// AudioEngine centraliza la carga, decodificación, efectos y reproducción de archivos de audio.
// Controla el ciclo de vida del altavoz (speaker) y la sincronización segura frente a accesos concurrentes.
type AudioEngine struct {
	streamer   beep.StreamSeekCloser // Decodificador activo del archivo de audio con capacidad de seek y cierre
	ctrl       *beep.Ctrl            // Controlador para pausar y reanudar el flujo de datos
	volume     *effects.Volume       // Efecto de ganancia para control de volumen logarítmico
	eq         *Equalizer            // Ecualizador paramétrico de 5 bandas
	format     beep.Format           // Formato original del archivo (frecuencia de muestreo y precisión)
	isInit     bool                  // Bandera para asegurar inicialización única del altavoz
	sessionID  int                   // Identificador de la sesión de reproducción actual (invalida eventos de pistas pasadas)
	cancelChan chan struct{}         // Canal para abortar de forma limpia la espera de término si se salta de pista
}

// NewAudioEngine inicializa y devuelve una nueva instancia del motor de audio.
func NewAudioEngine() *AudioEngine {
	return &AudioEngine{}
}

// Load detiene cualquier reproducción en curso, abre el archivo en disco, detecta su formato
// por extensión (.mp3, .wav, .flac, .ogg), decodifica el stream, lo remuestrea a la frecuencia
// estándar de 44.1 kHz y configura la cadena de efectos (control de pausa y volumen).
// Devuelve la duración real del archivo o un error en caso de fallo.
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

	// Inicializar el subsistema del speaker una sola vez con la frecuencia estándar y un buffer de 100ms
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

	// Remuestreo de alta calidad a 44100 Hz para evitar desajustes de velocidad en archivos con diferente frecuencia
	resampled := beep.Resample(4, format.SampleRate, standardSampleRate, streamer)

	ae.ctrl = &beep.Ctrl{Streamer: resampled}
	ae.eq = NewEqualizer(ae.ctrl, standardSampleRate)

	// Restaurar los ajustes de EQ guardados en la configuración de usuario,
	// si existen, para conservarlos entre sesiones.
	if enabled, gains, err := LoadEQ(); err == nil {
		ae.eq.Enabled = enabled
		for i := range gains {
			ae.eq.SetGain(i, gains[i])
		}
	}

	ae.volume = &effects.Volume{
		Streamer: ae.eq,
		Base:     math.Pow(10, 1.0/20.0), // Base para atenuación/ganancia logarítmica en decibelios (dB)
		Volume:   0,
		Silent:   false,
	}

	return realDuration, nil
}

// Play envía la cadena de audio (Limiter -> Volume -> Ctrl -> Streamer) al subsistema de altavoces
// y devuelve un canal que se cierra automáticamente cuando la reproducción concluye con éxito.
func (ae *AudioEngine) Play() chan struct{} {
	done := make(chan struct{})
	limiter := &Limiter{Streamer: ae.volume}

	speaker.Play(beep.Seq(limiter, beep.Callback(func() {
		close(done)
	})))
	return done
}

// Stop cancela inmediatamente la reproducción activa, limpia la cola de muestras del altavoz
// y cierra el archivo de audio liberando los descriptores de archivo asociados.
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
	ae.eq = nil
	speaker.Unlock()
}

// Pause pausa la reproducción actual de manera segura evitando condiciones de carrera con el hilo de audio.
func (ae *AudioEngine) Pause() {
	speaker.Lock()
	defer speaker.Unlock()
	if ae.ctrl == nil {
		return
	}
	ae.ctrl.Paused = true
}

// Resume reanuda la reproducción previamente pausada de manera segura.
func (ae *AudioEngine) Resume() {
	speaker.Lock()
	defer speaker.Unlock()
	if ae.ctrl == nil {
		return
	}
	ae.ctrl.Paused = false
}

// SetVolume ajusta la ganancia en decibelios (dB) dentro del efecto de volumen de forma segura.
func (ae *AudioEngine) SetVolume(level float64) {
	speaker.Lock()
	defer speaker.Unlock()
	if ae.volume == nil {
		return
	}
	ae.volume.Volume = level
}

// ToggleMute conmuta el estado de silencio (mute) sin perder el nivel de volumen configurado.
func (ae *AudioEngine) ToggleMute() {
	speaker.Lock()
	defer speaker.Unlock()
	if ae.volume == nil {
		return
	}
	ae.volume.Silent = !ae.volume.Silent
}

// SetEQBand ajusta la ganancia (dB) de una banda del ecualizador de forma segura.
func (ae *AudioEngine) SetEQBand(band int, gainDB float64) {
	speaker.Lock()
	defer speaker.Unlock()
	if ae.eq == nil {
		return
	}
	ae.eq.SetGain(band, gainDB)
}

// GetEQBands devuelve una copia de las ganancias actuales por banda en dB.
func (ae *AudioEngine) GetEQBands() [eqBandCount]float64 {
	speaker.Lock()
	defer speaker.Unlock()
	if ae.eq == nil {
		return [eqBandCount]float64{}
	}
	return ae.eq.Gains
}

// ToggleEQ activa o desactiva el ecualizador (bypass) y devuelve el nuevo estado.
func (ae *AudioEngine) ToggleEQ() bool {
	speaker.Lock()
	defer speaker.Unlock()
	if ae.eq == nil {
		return false
	}
	ae.eq.Enabled = !ae.eq.Enabled
	return ae.eq.Enabled
}

// HasEQ indica si hay un stream activo con ecualizador inicializado.
func (ae *AudioEngine) HasEQ() bool {
	speaker.Lock()
	defer speaker.Unlock()
	return ae.eq != nil
}

// EQEnabled indica si el ecualizador está activo.
func (ae *AudioEngine) EQEnabled() bool {
	speaker.Lock()
	defer speaker.Unlock()
	return ae.eq != nil && ae.eq.Enabled
}

// IsMuted indica si el audio está actualmente silenciado de forma segura.
func (ae *AudioEngine) IsMuted() bool {
	speaker.Lock()
	defer speaker.Unlock()
	return ae.volume != nil && ae.volume.Silent
}

// Position calcula y devuelve el tiempo transcurrido de reproducción de la pista actual en formato time.Duration.
func (ae *AudioEngine) Position() time.Duration {
	speaker.Lock()
	defer speaker.Unlock()
	if ae.streamer != nil {
		return ae.format.SampleRate.D(ae.streamer.Position())
	}
	return 0
}

// Seek desplaza la posición de lectura del stream a la duración solicitada de forma segura.
// Retorna un error si el formato del archivo no admite posicionamiento arbitrario.
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

// Close detiene la reproducción y libera de manera definitiva los recursos del subsistema de audio.
func (ae *AudioEngine) Close() {
	ae.Stop()
	if ae.isInit {
		speaker.Close()
	}
}
