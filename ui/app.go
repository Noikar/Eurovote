package ui

import (
	"fmt"
	"time"

	"eurovote/models"
	"eurovote/scraper"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Run builds and shows the main application window.
func Run(app fyne.App) {
	w := app.NewWindow("Eurovote")
	w.Resize(fyne.NewSize(700, 600))

	root := container.NewStack(loadingScreen())
	w.SetContent(root)
	w.Show()

	go func() {
		year := time.Now().Year()
		acts, dates, err := scraper.FetchContest(year)

		var content fyne.CanvasObject
		if err != nil {
			content = errorScreen(err)
		} else {
			content = wrapWithDebug(acts, dates, w)
		}

		fyne.Do(func() {
			root.Objects = []fyne.CanvasObject{content}
			root.Refresh()
		})
	}()
}

func wrapWithDebug(acts []models.Act, dates models.ContestDates, w fyne.Window) fyne.CanvasObject {
	year := dates.SF1.Year()
	contentArea := container.NewStack(routeByDate(acts, dates, w))

	debugBtn := widget.NewButton("Debug", func() {
		modes := []string{
			"Semi-Final 1", "Semi-Final 2", "Grand Final", "Rankings",
			"SF1 Results", "SF2 Results", "Final Results",
		}
		radio := widget.NewRadioGroup(modes, nil)
		dlg := dialog.NewCustomConfirm("Debug Mode", "Switch", "Cancel", radio, func(ok bool) {
			if !ok || radio.Selected == "" {
				return
			}
			var screen fyne.CanvasObject
			switch radio.Selected {
			case "Semi-Final 1":
				screen = NewSemiFinalScreen(acts, 1, w)
			case "Semi-Final 2":
				screen = NewSemiFinalScreen(acts, 2, w)
			case "Grand Final":
				screen = NewFinalScreen(acts, w)
			case "SF1 Results":
				screen = NewSemiResultsScreen(acts, 1, year, w)
			case "SF2 Results":
				screen = NewSemiResultsScreen(acts, 2, year, w)
			case "Final Results":
				screen = NewFinalResultsScreen(acts, year, w)
			default:
				screen = NewRankingScreen(acts, w)
			}
			contentArea.Objects = []fyne.CanvasObject{screen}
			contentArea.Refresh()
		}, w)
		dlg.Show()
	})

	overlay := container.NewBorder(
		nil,
		container.NewHBox(layout.NewSpacer(), container.NewPadded(debugBtn)),
		nil, nil,
	)
	return container.NewStack(contentArea, overlay)
}

// routeByDate returns the appropriate screen based on the scraped contest dates.
// On the day of each event it shows the voting screen; the day after it switches
// to the results screen for that event.
func routeByDate(acts []models.Act, dates models.ContestDates, w fyne.Window) fyne.CanvasObject {
	today := todayDate()
	year := dates.SF1.Year()

	switch {
	case sameDay(today, dates.SF1):
		return NewSemiFinalScreen(acts, 1, w)
	case sameDay(today, dates.SF2):
		return NewSemiFinalScreen(acts, 2, w)
	case sameDay(today, dates.Final):
		return NewFinalScreen(acts, w)
	case today.After(dates.Final):
		return NewFinalResultsScreen(acts, year, w)
	case today.After(dates.SF2):
		return NewSemiResultsScreen(acts, 2, year, w)
	case today.After(dates.SF1):
		return NewSemiResultsScreen(acts, 1, year, w)
	default:
		return NewRankingScreen(acts, w)
	}
}

func todayDate() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func loadingScreen() fyne.CanvasObject {
	spinner := widget.NewProgressBarInfinite()
	label := widget.NewLabel("Fetching Eurovision data from Wikipedia...")
	label.Alignment = fyne.TextAlignCenter
	return container.NewCenter(
		container.NewVBox(label, spinner),
	)
}

func errorScreen(err error) fyne.CanvasObject {
	msg := widget.NewLabel(fmt.Sprintf("Could not load data:\n%v\n\nCheck your internet connection and try again.", err))
	msg.Alignment = fyne.TextAlignCenter
	msg.Wrapping = fyne.TextWrapWord
	return container.NewCenter(msg)
}
