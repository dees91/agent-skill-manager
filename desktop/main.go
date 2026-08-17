package main

import (
	"embed"
	"fmt"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/dees91/agent-skill-manager/internal/gui"
	"github.com/dees91/agent-skill-manager/internal/paths"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed wails.json
var desktopBuildMetadata []byte

//go:embed build/appicon.png
var desktopApplicationIcon []byte

func main() {
	p, err := paths.Default()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	about, err := newDesktopAboutInfo(desktopBuildMetadata, desktopApplicationIcon)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	app := newApp(gui.New(p))
	appMenu := menu.NewMenuFromItems(menu.AppMenu(), menu.EditMenu())

	err = wails.Run(&options.App{
		Title:                    "Skill Manager",
		Width:                    1440,
		Height:                   960,
		MinWidth:                 1024,
		MinHeight:                720,
		BackgroundColour:         options.NewRGB(22, 22, 23),
		Menu:                     appMenu,
		OnStartup:                app.startup,
		OnBeforeClose:            app.beforeClose,
		CSSDragProperty:          "--wails-draggable",
		CSSDragValue:             "drag",
		AssetServer:              &assetserver.Options{Assets: assets},
		Bind:                     []interface{}{app},
		EnableDefaultContextMenu: false,
		Mac: &mac.Options{
			Appearance:                   mac.NSAppearanceNameDarkAqua,
			TitleBar:                     mac.TitleBarHiddenInset(),
			DisableEscapeExitsFullscreen: true,
			About:                        about,
		},
	})
	if err != nil {
		fmt.Println("Error:", err)
	}
}
