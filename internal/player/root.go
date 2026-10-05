// Package player proporciona la máquina de estados y el modelo raíz (RootModel) de Bubble Tea,
// encargado de coordinar la navegación e intercambio de vistas entre el menú interactivo
// y la pantalla del reproductor de audio, además de centralizar la telemetría y sincronización.
package player

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fonta81/Reproductor-go/menu"
)

// Screen identifica de manera unívoca las pantallas o vistas disponibles en la interfaz gráfica del terminal.
type Screen int

// Identificadores de las pantallas del sistema de navegación.
const (
	ScreenMenu   Screen = iota // ScreenMenu presenta el menú de bienvenida, opciones generales y estado resumido.
	ScreenPlayer               // ScreenPlayer presenta la vista interactiva completa del reproductor y la biblioteca.
)

// RootModel es el modelo principal de Bubble Tea que orquesta la arquitectura visual de la aplicación.
// Encapsula tanto el modelo del menú como el modelo del reproductor, gestionando el enrutamiento
// de eventos de teclado, redimensionamientos de terminal y la sincronización de telemetría de audio.
type RootModel struct {
	currentScreen Screen     // Identificador de la pantalla que se está mostrando actualmente al usuario
	menuModel     menu.Model // Instancia del modelo secundario que gobierna la pantalla del menú
	playerModel   AppModel   // Instancia del modelo secundario que gestiona el reproductor de audio y cola
}

// NewRootModel construye e inicializa una nueva instancia de RootModel configurando
// la pantalla inicial en el menú principal, inicializando los submódulos correspondientes
// y realizando la primera sincronización del estado de audio hacia el menú.
func NewRootModel(initialDir string) RootModel {
	r := RootModel{
		currentScreen: ScreenMenu,
		menuModel:     menu.New(),
		playerModel:   NewAppModel(initialDir),
	}
	r.syncMenuState()
	return r
}

// syncMenuState recopila el estado actual del reproductor (conteo de pistas en la lista,
// volumen en escala porcentual, estado de silencio, estado de reproducción y metadatos
// de la canción activa) y lo proyecta sobre el modelo del menú para su renderizado en la tarjeta informativa.
func (r *RootModel) syncMenuState() {
	var trackCount int
	if r.playerModel.playlist != nil {
		trackCount = r.playerModel.playlist.Length()
	}

	// Normalizar el nivel de volumen en decibelios a un porcentaje legible 0-100%
	volPct := VolumePercentageInt(r.playerModel.volumeLevel, minVolume, maxVolume)

	isMuted := false
	if r.playerModel.Audio != nil {
		isMuted = r.playerModel.Audio.IsMuted()
	}

	state := menu.PlayerState{
		TrackCount: trackCount,
		Volume:     volPct,
		IsMuted:    isMuted,
	}

	// Mapear el estado operativo del reproductor a etiquetas legibles
	switch r.playerModel.state {
	case StatePlaying:
		state.State = "Reproduciendo"
		state.IsPlaying = true
	case StatePaused:
		state.State = "Pausado"
		state.IsPaused = true
	default:
		state.State = "Detenido"
	}

	// Adjuntar metadatos de la pista en curso si existe una selección activa
	if r.playerModel.playlist != nil {
		if track, ok := r.playerModel.playlist.Current(); ok {
			state.HasTrack = true
			state.CurrentTrackTitle = track.Title
			state.CurrentTrackArtist = track.Artist
			if track.Duration > 0 {
				state.CurrentTrackDuration = formatDuration(track.Duration)
			}
		}
	}

	r.menuModel.SetState(state)
}

// Init inicializa simultáneamente los ciclos de vida del menú y del reproductor,
// agrupando sus comandos de inicialización mediante tea.Batch conforme a la arquitectura Elm.
func (r RootModel) Init() tea.Cmd {
	r.syncMenuState()
	return tea.Batch(
		r.menuModel.Init(),
		r.playerModel.Init(),
	)
}

// Update procesa los mensajes del bucle de eventos de Bubble Tea. Se encarga del enrutamiento
// hacia la pantalla activa, del manejo de atajos de teclado globales (como Ctrl+C o conmutación con Esc/p),
// del despacho de acciones emitidas por el menú y de la redistribución de eventos asíncronos y de redimensionamiento.
func (r RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Notificar el cambio de dimensiones a ambos modelos para ajustar bordes y centrado
		var cmdMenu, cmdPlayer tea.Cmd
		var mm, pm tea.Model

		mm, cmdMenu = r.menuModel.Update(msg)
		r.menuModel = mm.(menu.Model)

		pm, cmdPlayer = r.playerModel.Update(msg)
		r.playerModel = pm.(AppModel)

		r.syncMenuState()
		return r, tea.Batch(cmdMenu, cmdPlayer)

	case menu.SelectMsg:
		// Despachar acciones generadas por la selección interactiva en el menú principal
		switch msg.Action {
		case menu.ActionTogglePlay:
			// Conmutar entre reproducir y pausar según el estado actual
			if r.playerModel.state == StateStopped {
				if r.playerModel.playlist.Length() > 0 {
					var pm tea.Model
					var cmd tea.Cmd
					pm, cmd = r.playerModel.playCurrent()
					r.playerModel = pm.(AppModel)
					r.currentScreen = ScreenPlayer
					r.syncMenuState()
					return r, cmd
				}
				r.syncMenuState()
				return r, nil
			}
			var pm tea.Model
			var cmd tea.Cmd
			pm, cmd = r.playerModel.togglePlayback()
			r.playerModel = pm.(AppModel)
			r.syncMenuState()
			return r, cmd

		case menu.ActionViewPlayer:
			// Cambiar a la vista detallada del reproductor
			r.currentScreen = ScreenPlayer
			r.syncMenuState()
			return r, nil

		case menu.ActionNextTrack:
			// Avanzar a la siguiente pista de la cola
			var pm tea.Model
			var cmd tea.Cmd
			pm, cmd = r.playerModel.playNext()
			r.playerModel = pm.(AppModel)
			r.syncMenuState()
			return r, cmd

		case menu.ActionPrevTrack:
			// Retroceder a la pista anterior
			var pm tea.Model
			var cmd tea.Cmd
			pm, cmd = r.playerModel.playPrevious()
			r.playerModel = pm.(AppModel)
			r.syncMenuState()
			return r, cmd

		case menu.ActionLibrary:
			// Mostrar la cola y biblioteca desactivando exploradores modales
			r.currentScreen = ScreenPlayer
			r.playerModel.isPickingFolder = false
			r.playerModel.isFiltering = false
			r.syncMenuState()
			return r, nil

		case menu.ActionSettings:
			// Abrir el explorador de directorios en el reproductor para configurar la ruta de música
			r.currentScreen = ScreenPlayer
			r.playerModel.isFiltering = false
			cmd := r.playerModel.openBrowser(r.playerModel.musicDir)
			r.syncMenuState()
			return r, cmd

		case menu.ActionQuit:
			// Liberar recursos y finalizar el programa
			if r.playerModel.Audio.HasEQ() {
				_ = SaveEQ(r.playerModel.Audio.EQEnabled(), r.playerModel.Audio.GetEQBands())
			}
			r.playerModel.Audio.Close()
			return r, tea.Quit
		}

	case tea.KeyMsg:
		// Interrupción forzada global mediante Ctrl+C disponible en cualquier pantalla
		if msg.String() == "ctrl+c" {
			if r.playerModel.Audio.HasEQ() {
				_ = SaveEQ(r.playerModel.Audio.EQEnabled(), r.playerModel.Audio.GetEQBands())
			}
			r.playerModel.Audio.Close()
			return r, tea.Quit
		}

		if r.currentScreen == ScreenMenu {
			// Atajos directos para saltar del menú a la vista del reproductor
			if msg.String() == "esc" || msg.String() == "p" {
				r.currentScreen = ScreenPlayer
				r.syncMenuState()
				return r, nil
			}

			var menuModel tea.Model
			var cmd tea.Cmd
			menuModel, cmd = r.menuModel.Update(msg)
			r.menuModel = menuModel.(menu.Model)
			r.syncMenuState()
			cmds = append(cmds, cmd)
			return r, tea.Batch(cmds...)
		} else {
			// En la vista del reproductor: si se presiona Esc fuera de los modos modales
			// (búsqueda o explorador de carpetas), retornar a la pantalla del menú
			if msg.String() == "esc" && !r.playerModel.isFiltering && !r.playerModel.isPickingFolder && !r.playerModel.eqActive {
				r.currentScreen = ScreenMenu
				r.syncMenuState()
				return r, nil
			}
			var playerModel tea.Model
			var cmd tea.Cmd
			playerModel, cmd = r.playerModel.Update(msg)
			r.playerModel = playerModel.(AppModel)
			r.syncMenuState()
			cmds = append(cmds, cmd)
			return r, tea.Batch(cmds...)
		}
	}

	// Propagar mensajes del sistema (temporizadores de refresco, finalización de audio, escaneos) a ambos modelos
	var menuModel tea.Model
	var playerModel tea.Model
	var cmdMenu, cmdPlayer tea.Cmd

	menuModel, cmdMenu = r.menuModel.Update(msg)
	r.menuModel = menuModel.(menu.Model)

	playerModel, cmdPlayer = r.playerModel.Update(msg)
	r.playerModel = playerModel.(AppModel)

	r.syncMenuState()

	cmds = append(cmds, cmdMenu, cmdPlayer)
	return r, tea.Batch(cmds...)
}

// View renderiza en la terminal la interfaz visual de la pantalla actualmente seleccionada
// (el menú interactivo o el panel completo del reproductor de audio).
func (r RootModel) View() string {
	if r.currentScreen == ScreenMenu {
		return r.menuModel.View()
	}
	return r.playerModel.View()
}

// Close detiene el motor de sonido y libera de manera definitiva los recursos del subsistema de audio
// antes de la finalización del proceso de la aplicación.
func (r RootModel) Close() {
	r.playerModel.Audio.Close()
}
