// Command tradeloom is the desktop application (the only package besides internal/app that imports Wails).
package main

import (
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"tradeloom/frontend"
	"tradeloom/internal/app"
)

func main() {
	a := app.New()
	err := wails.Run(&options.App{
		Title:       "Tradeloom",
		Width:       1440,
		Height:      900,
		MinWidth:    1280, // FR-SHELL layouts are designed for 1280 px; P0-4 relies on this floor
		MinHeight:   720,
		AssetServer: &assetserver.Options{Assets: frontend.Assets()},
		OnStartup:   a.OnStartup,
		Bind:        []interface{}{a},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "tradeloom:", err)
		os.Exit(1)
	}
}
