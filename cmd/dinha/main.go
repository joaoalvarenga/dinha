package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"github.com/joaoalvarenga/dinha/internal/db"
	"github.com/joaoalvarenga/dinha/internal/tui"
)

func main() {
	godotenv.Load()

	database := db.GetDB()
	defer database.Close()

	app := tui.New(database)
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
