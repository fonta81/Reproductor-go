// Package main constituye el punto de entrada ejecutable de GoPlayer, un reproductor
// de audio para terminal (TUI) desarrollado en Go utilizando el framework Bubble Tea.
// Coordina el análisis de argumentos por línea de comandos, la recuperación de la configuración
// persistente del usuario y el ciclo de vida de la ejecución en pantalla alternativa (alt-screen).
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fonta81/Reproductor-go/internal/player"
)

// main es la función principal que inicializa el entorno de ejecución, procesa los parámetros
// de invocación, restaura la última ruta musical guardada en caso de no especificarse una bandera,
// instancia el modelo raíz de la interfaz y arranca el bucle de eventos del programa Bubble Tea.
func main() {
	// Declaración y procesamiento de las banderas de línea de comandos
	dirFlag := flag.String("dir", "", "Directorio inicial de música a escanear")
	flag.Parse()

	// Si no se proporcionó un directorio explícito por flag, intentar recuperar la ruta previa guardada
	if *dirFlag == "" {
		if saved, err := player.LoadConfig(); err == nil && saved != "" {
			*dirFlag = saved
		}
	}

	// Instanciación del modelo raíz y registro diferido de la liberación de recursos de audio
	model := player.NewRootModel(*dirFlag)
	defer model.Close()

	// Configuración y lanzamiento del bucle de eventos en pantalla alternativa del terminal
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Printf("Error al iniciar el reproductor: %v\n", err)
		os.Exit(1)
	}
}
