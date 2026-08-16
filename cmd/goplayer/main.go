// Package main is the entry point for the GoPlayer application.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fonta81/Reproductor-go/internal/player"
)

// main es el punto de entrada de la aplicación.
func main() {
	// Definir y parsear el flag para el directorio inicial.
	dirFlag := flag.String("dir", "", "Directorio inicial de música a escanear")
	flag.Parse()

	// Crear el modelo de la aplicación y asegurar el cierre del motor de audio al finalizar.
	if *dirFlag == "" {
		if saved, err := player.LoadConfig(); err == nil && saved != "" {
			*dirFlag = saved
		}
	}

	model := player.NewRootModel(*dirFlag)
	defer model.Close()

	// Iniciar el programa Bubble Tea con pantalla alternativa.
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Printf("Error al iniciar el reproductor: %v\n", err)
		os.Exit(1)
	}
}
