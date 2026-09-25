# GoPlayer

[Leer en inglés](readme.md)

> Un reproductor de música TUI (Interfaz de Usuario de Terminal) completo, escrito en Go y potenciado por **Bubble Tea**, **Lipgloss** y **Beep**.

GoPlayer ofrece una experiencia de reproducción de audio moderna y elegante directamente en tu terminal. Construido siguiendo **The Elm Architecture** (Model-View-Update), proporciona una navegación fluida, exploración dinámica de directorios, controles de volumen, modos de reproducción personalizables y un estilo visual inspirado en Dracula.

---

## Características Principales

- **Interfaz de Usuario de Terminal (TUI)**: Interfaz de terminal hermosa y responsiva con indicadores de estado, barras de progreso y controles de pista personalizados.
- **Menú Principal Interactivo**: Menú principal intuitivo para iniciar acciones (Reproducir pista, Pausar/Detener, Siguiente/Anterior canción, Biblioteca musical, Configuración y Salir).
- **Explorador de Archivos Dinámico**: Selector visual de directorios integrado (`o` / `ctrl+o` o desde el menú) que permite explorar subcarpetas y cambiar el directorio de música sobre la marcha (restringido de forma segura dentro del directorio de usuario `$HOME`).
- **Configuración Persistente**: Recuerda automáticamente la última carpeta seleccionada guardándola en `~/.config/goplayer/config.json`.
- **Soporte para Múltiples Formatos**: Decodifica y reproduce archivos de audio **MP3**, **WAV**, **FLAC** y **OGG** sin problemas.
- **Control de Audio y Motor**:
  - Control de volumen preciso con escala logarítmica, efecto limitador y silenciado instantáneo.
  - Capacidad de búsqueda (saltar hacia adelante/atrás mediante intervalos configurables).
- **Gestión de Lista de Reproducción y Cola**:
  - Lista de reproducción interactiva que muestra las pistas actuales, futuras y anteriores.
  - Capacidad de eliminar pistas individuales de la sesión activa.
- **Extracción de Metadatos**: Extrae automáticamente metadatos de **Título**, **Artista** y **Álbum** de las etiquetas de audio.
- **Filtro Rápido (Quick Filter)**: Presiona `Ctrl+F` para abrir una barra de búsqueda en línea (fuzzy/substring) que filtra la lista por Título o Artista en tiempo real. Navega coincidencias con ↑/↓ o j/k, presiona `Enter` para reproducir la pista seleccionada y `Esc` para cerrar/limpiar el filtro.
- **Modos de Reproducción**:
  - **Aleatorio (Shuffle)**: Orden de lista de reproducción aleatorio.
  - **Modos de Repetición**: Repetición desactivada, Repetir una (pista única) o Repetir todo (lista completa).
- **Fallbacks Elegantes y Auto-escaneo**: Escanea automáticamente carpetas locales (`./music`, `./songs`, `~/Music`, `~/Música`), la configuración del usuario o acepta un flag de directorio personalizado en la línea de comandos.

---

## Estructura del Proyecto

```text
.
├── cmd
│   └── goplayer
│       └── main.go       # Punto de entrada
├── internal
│   └── player
│       ├── app.go        # Modelo de UI/Aplicación del reproductor
│       ├── audio.go      # Motor de audio (Beep)
│       ├── config.go     # Gestión de configuración persistente
│       ├── constants.go  # Constantes, colores y estilos
│       ├── models.go     # Estructuras de datos (Track, Playlist, etc.)
│       ├── root.go       # Enrutador de pantallas (Menú <-> Reproductor)
│       └── utils.go      # Funciones auxiliares y extracción de metadatos
├── menu
│   └── menu.go           # Modelo TUI del menú principal
├── flake.nix             # Flake para entorno de desarrollo en Nix
└── flake.lock
```

---

## Stack Tecnológico y Arquitectura

- **Lenguaje**: [Go (Golang)](https://golang.org/)
- **Arquitectura**: The Elm Architecture / Model-View-Update (MVU)
- **Frameworks y Librerías**:
  - [Charm Bubble Tea](https://github.com/charmbracelet/bubbletea) — Framework TUI basado en Elm.
  - [Charm Lipgloss](https://github.com/charmbracelet/lipgloss) — Definiciones de estilo y diseños de terminal.
  - [Charm Bubbles Progress & TextInput](https://github.com/charmbracelet/bubbles) — Componentes de barra de progreso y campo de texto.
  - [Faiface Beep](https://github.com/faiface/beep) — Librería de audio para Go (decodificación, remuestreo, control de volumen y reproducción).
- **Entorno de desarrollo**: [Nix Flakes](https://nixos.wiki/wiki/Flakes) (suministra Go, gopls, pkg-config, alsa-lib).

---

## Prerrequisitos

Antes de ejecutar GoPlayer, asegúrate de tener instalado en tu sistema:

- **Go** (se recomienda la versión 1.22 o superior para la sintaxis moderna de rangos)
- Librerías de desarrollo de audio del sistema (ej. `alsa-lib` / `pkg-config` en Linux si se compila con soporte de audio CGO). Como alternativa, puedes usar **Nix**.

---

## Instalación y Configuración

### Configuración Estándar

1. **Clona el repositorio**:
   ```bash
   git clone https://github.com/fonta81/Reproductor-go.git
   cd Reproductor-go
   ```

2. **Instala las dependencias de Go**:
   ```bash
   go mod tidy
   ```

3. **Ejecuta la aplicación**:
   ```bash
   go run ./cmd/goplayer
   ```

### Usando Nix Flake

Si utilizas Nix con soporte para flakes habilitado:

```bash
nix develop
go run ./cmd/goplayer
```

---

## Uso

### Ejecución Predeterminada
Al iniciar, GoPlayer muestra el menú principal y busca música en `./music`, `./songs`, la carpeta de música predeterminada del sistema o la guardada en `~/.config/goplayer/config.json`:

```bash
go run ./cmd/goplayer
```

### Especificar un Directorio de Música Personalizado
Pasa el flag `-dir` para apuntar a una carpeta específica al iniciar:

```bash
go run ./cmd/goplayer -dir /ruta/a/tu/musica
```

---

## Atajos de Teclado y Controles

### Menú Principal
| Tecla | Acción |
| :--- | :--- |
| `↑` / `k` | Mover selección hacia arriba |
| `↓` / `j` | Mover selección hacia abajo |
| `Enter` / `Space` | Seleccionar opción del menú |
| `q` / `Ctrl+C` | Salir |

### Pantalla de Reproducción
| Tecla | Acción |
| :--- | :--- |
| `Space` | Alternar Play / Pausa |
| `n` | Pista siguiente |
| `N` | Pista anterior (o reiniciar la pista actual si el tiempo transcurrido > 3s) |
| `.` o `>` | Adelantar (10 segundos) |
| `,` o `<` | Atrasar (10 segundos) |
| `0` | Reiniciar pista actual |

### Navegación y Cola
| Tecla | Acción |
| :--- | :--- |
| `↑` / `k` | Mover cursor de selección hacia arriba |
| `↓` / `j` | Mover cursor de selección hacia abajo |
| `Enter` | Reproducir pista seleccionada |
| `d` | Eliminar pista seleccionada de la cola |
| `l` | Alternar visibilidad del panel de cola |
| `o` / `Ctrl+O` | Abrir explorador visual de directorios |
| `Ctrl+F` | Abrir filtro rápido (buscar Título o Artista) |
| `Esc` | Volver al Menú Principal (cuando no se está en el filtro ni en el explorador) |

### Audio y Modos
| Tecla | Acción |
| :--- | :--- |
| `+` / `=` | Subir volumen |
| `-` | Bajar volumen |
| `m` | Alternar Mute |
| `s` | Alternar modo Aleatorio |
| `r` | Ciclar modo de Repetición (Desactivado → Una → Todo) |

### Explorador de Directorios (Modo Activo)
| Tecla | Acción |
| :--- | :--- |
| `↑` / `k` | Mover cursor hacia arriba |
| `↓` / `j` | Mover cursor hacia abajo |
| `←` / `Backspace` | Navegar al directorio padre (dentro de `$HOME`) |
| `→` / `Enter` | Entrar al directorio seleccionado |
| `Space` | Confirmar y escanear el directorio seleccionado |
| `Esc` / `q` | Cancelar selección de directorio |

### Filtro Rápido (Modo Activo)
| Tecla | Acción |
| :--- | :--- |
| `↑` / `k` | Mover selección en sugerencias / coincidencias |
| `↓` / `j` | Mover selección en sugerencias / coincidencias |
| `Enter` | Reproducir pista coincidente seleccionada y cerrar filtro |
| `Esc` | Cancelar y cerrar filtro |

### Sistema
| Key | Action |
| :--- | :--- |
| `h` o `?` | Alternar visibilidad del panel de ayuda |
| `q` o `Ctrl+C` | Salir de GoPlayer |

---

## Licencia

Este proyecto está bajo la Licencia MIT.
