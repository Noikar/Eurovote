package main

import (
	"eurovote/ui"

	"fyne.io/fyne/v2/app"
)

func main() {
	a := app.NewWithID("com.eurovote")
	ui.Run(a)
	a.Run()
}
