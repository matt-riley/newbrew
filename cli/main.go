// main.go
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-riley/newbrew/tui"
)

func main() {
	if _, err := tea.NewProgram(tui.InitialModel()).Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
