// Package player proporciona el ecualizador paramétrico de 5 bandas basado en
// filtros biquad (peaking EQ) según el RBJ Audio EQ Cookbook.
package player

import (
	"math"

	"github.com/faiface/beep"
)

// Configuración fija del ecualizador: 5 bandas centradas en frecuencias
// musicales clásicas, factor Q de 1.0 y ganancias limitadas a ±12 dB.
const (
	eqBandCount   = 5
	eqQ           = 1.0
	eqGainMin     = -12.0
	eqGainMax     = 12.0
	eqGainStep    = 1.0
	eqDefaultGain = 0.0
)

// eqFrequencies define las frecuencias centrales (Hz) de cada banda.
var eqFrequencies = [eqBandCount]float64{60, 250, 1000, 4000, 10000}

// biquadCoeffs contiene los coeficientes normalizados de un filtro biquad.
type biquadCoeffs struct {
	b0, b1, b2, a1, a2 float64
}

// peakingCoeffs calcula los coeficientes de un filtro peaking EQ (RBJ cookbook).
func peakingCoeffs(freq, sampleRate, q, gainDB float64) biquadCoeffs {
	A := math.Pow(10, gainDB/40.0)
	w0 := 2 * math.Pi * freq / sampleRate
	alpha := math.Sin(w0) / (2 * q)
	cosW0 := math.Cos(w0)

	a0 := 1 + alpha/A
	return biquadCoeffs{
		b0: (1 + alpha*A) / a0,
		b1: (-2 * cosW0) / a0,
		b2: (1 - alpha*A) / a0,
		a1: (-2 * cosW0) / a0,
		a2: (1 - alpha/A) / a0,
	}
}

// biquadState guarda el historial de muestras por canal para un filtro.
type biquadState struct {
	x1, x2, y1, y2 float64
}

func (s *biquadState) process(c biquadCoeffs, x float64) float64 {
	y := c.b0*x + c.b1*s.x1 + c.b2*s.x2 - c.a1*s.y1 - c.a2*s.y2
	s.x2, s.x1 = s.x1, x
	s.y2, s.y1 = s.y1, y
	return y
}

// Equalizer implementa un ecualizador gráfico de 5 bandas como un
// beep.Streamer: cada banda es un biquad peaking en serie por canal.
type Equalizer struct {
	Streamer beep.Streamer

	SampleRate float64              // Frecuencia de muestreo del flujo de entrada
	Gains      [eqBandCount]float64 // Ganancia por banda en dB
	Enabled    bool                 // Interruptor global: false = bypass exacto

	coeffs [eqBandCount]biquadCoeffs
	states [eqBandCount][2]biquadState // estado por banda y canal (L, R)
}

// NewEqualizer construye un ecualizador con todas las bandas en 0 dB y
// recalcula los coeficientes para la frecuencia de muestreo indicada.
func NewEqualizer(s beep.Streamer, sampleRate beep.SampleRate) *Equalizer {
	eq := &Equalizer{Streamer: s, SampleRate: float64(sampleRate), Enabled: true}
	for i := range eq.Gains {
		eq.Gains[i] = eqDefaultGain
	}
	eq.recalc()
	return eq
}

// recalc recalcula todos los coeficientes a partir de las ganancias actuales.
func (eq *Equalizer) recalc() {
	for i := range eq.coeffs {
		eq.coeffs[i] = peakingCoeffs(eqFrequencies[i], eq.SampleRate, eqQ, eq.Gains[i])
	}
}

// SetGain ajusta la ganancia (dB, limitada a ±12) de una banda y recalcula coeficientes.
func (eq *Equalizer) SetGain(band int, gainDB float64) {
	if band < 0 || band >= eqBandCount {
		return
	}
	eq.Gains[band] = max(eqGainMin, min(eqGainMax, gainDB))
	eq.coeffs[band] = peakingCoeffs(eqFrequencies[band], eq.SampleRate, eqQ, eq.Gains[band])
}

// Stream procesa las muestras de audio aplicando las 5 bandas en serie por canal.
func (eq *Equalizer) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = eq.Streamer.Stream(samples)
	if !eq.Enabled {
		return n, ok
	}
	for i := range samples[:n] {
		for ch := 0; ch < 2; ch++ {
			v := samples[i][ch]
			for b := range eq.coeffs {
				v = eq.states[b][ch].process(eq.coeffs[b], v)
			}
			samples[i][ch] = v
		}
	}
	return n, ok
}

// Err propaga errores del streamer subyacente si los implementa.
func (eq *Equalizer) Err() error {
	if se, ok := eq.Streamer.(interface{ Err() error }); ok {
		return se.Err()
	}
	return nil
}
