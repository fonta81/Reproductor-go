package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fonta81/Reproductor-go/internal/player"
)

func main() {
	dirFlag := flag.String("dir", "", "Directorio inicial de música a escanear")
	flag.Parse()

	model := player.NewAppModel(*dirFlag)
	defer model.Audio.Close()

	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Printf("Error al iniciar el reproductor: %v\n", err)
		os.Exit(1)
	}
}
