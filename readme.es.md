# GoPlayer

[Leer en inglés](readme.md)

> Un reproductor de música TUI (Interfaz de Usuario de Terminal) completo, escrito en Go y potenciado por **Bubble Tea**, **Lipgloss** y **Beep**.

GoPlayer ofrece una experiencia de reproducción de audio moderna y elegante directamente en tu terminal. Construido siguiendo **The Elm Architecture** (Model-View-Update), proporciona una navegación fluida, exploración dinámica de directorios, controles de volumen, modos de reproducción personalizables y un estilo visual inspirado en Dracula.

---

## Características Principales

- **Interfaz de Usuario de Terminal (TUI)**: Interfaz de terminal hermosa y responsiva con indicadores de estado, barras de progreso y controles de pista personalizados.
- **Explorador de Archivos Dinámico**: Selector visual de directorios integrado (`o` / `ctrl+o`) que permite explorar subcarpetas y cambiar el directorio de música sobre la marcha.
- **Soporte para Múltiples Formatos**: Decodifica y reproduce archivos de audio **MP3** y **WAV** sin problemas.
- **Control de Audio y Motor**:
  - Control de volumen preciso con escala logarítmica, efecto limitador y silenciado instantáneo.
  - Capacidad de búsqueda (saltar hacia adelante/atrás mediante intervalos configurables).
- **Gestión de Lista de Reproducción y Cola**:
  - Lista de reproducción interactiva que muestra las pistas actuales, futuras y anteriores.
  - Capacidad de eliminar pistas individuales de la sesión activa.
- **Filtro Rápido (Quick Filter)**: Presiona `Ctrl+F` para abrir una barra de búsqueda en línea (fuzzy/substring) que filtra la lista por Título o Artista en tiempo real. Navega coincidencias con ↑/↓ o j/k, presiona `Enter` para reproducir la pista seleccionada y `Esc` para cerrar/limpiar el filtro.

- **Modos de Reproducción**:
  - **Aleatorio (Shuffle)**: Orden de lista de reproducción aleatorio.
  - **Modos de Repetición**: Repetición desactivada, Repetir una (pista única) o Repetir todo (lista completa).
- **Fallbacks Elegantes y Auto-escaneo**: Escanea automáticamente carpetas locales (`./music`, `./songs`, `~/Music`, `~/Música`) o acepta un flag de directorio personalizado en la línea de comandos.

---

## Estructura del Proyecto

```text
.
├── cmd
│   └── goplayer
│       └── main.go       # Punto de entrada
└── internal
    └── player
        ├── app.go        # Modelo de UI/Aplicación
        ├── audio.go      # Motor de audio
        ├── constants.go  # Constantes y estilos
        ├── models.go     # Estructuras de datos
        └── utils.go      # Funciones auxiliares
```

---

## Stack Tecnológico y Arquitectura

- **Lenguaje**: [Go (Golang)](https://golang.org/)
- **Arquitectura**: The Elm Architecture / Model-View-Update (MVU)
- **Frameworks y Librerías**:
  - [Charm Bubble Tea](https://github.com/charmbracelet/bubbletea) — Framework TUI basado en Elm.
  - [Charm Lipgloss](https://github.com/charmbracelet/lipgloss) — Definiciones de estilo y diseños de terminal.
  - [Charm Bubbles Progress](https://github.com/charmbracelet/bubbles) — Componente de barra de progreso.
  - [Faiface Beep](https://github.com/faiface/beep) — Librería de audio para Go (decodificación, remuestreo, control de volumen y reproducción).

---

## Prerrequisitos

Antes de ejecutar GoPlayer, asegúrate de tener instalado en tu sistema:

- **Go** (se recomienda la versión 1.22 o superior para la sintaxis moderna de rangos)

## Instalación y Configuración

1. **Clona el repositorio**:
   ```bash
   git clone https://github.com/fonta81/Reproductor-go.git
   cd Reproductor-go
   ```

2. **Instala las dependencias de Go**:
   ```bash
   go mod tidy
   ```
3. **Ejecuta el código**:

   ```bash
   go run ./cmd/goplayer
   ```

---

## Uso

### Ejecución Predeterminada
Por defecto, GoPlayer escanea `./music`, `./songs` y la carpeta de música predeterminada de tu sistema en busca de archivos `.mp3` y `.wav`:

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

### Reproducción
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
| `←` / `Backspace` | Navegar al directorio padre |
| `→` / `Enter` | Entrar al directorio seleccionado |
| `Space` | Confirmar y escanear el directorio seleccionado |
| `Esc` / `q` | Cancelar selección de directorio |

### Sistema
| Tecla | Acción |
| :--- | :--- |
| `h` o `?` | Alternar visibilidad del panel de ayuda |
| `q` o `Ctrl+C` | Salir de GoPlayer |

---

## Licencia

Este proyecto está bajo la Licencia MIT.
