package ui

import (
	"fmt"
	"image/color"
	"sort"

	"eurovote/models"
	"eurovote/scraper"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// NewSemiResultsScreen shows the results of a semi-final and compares them with
// the user's saved favorites. Results are fetched asynchronously from Wikipedia.
func NewSemiResultsScreen(allActs []models.Act, semiNum, year int, w fyne.Window) fyne.CanvasObject {
	var acts []models.Act
	for _, a := range allActs {
		if a.SemiGroup == semiNum {
			acts = append(acts, a)
		}
	}

	userOrder := loadSemiSelections(semiNum)
	userPicks := topTenFavorites(userOrder)

	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Eurovision %d — Semi-final %d Results", year, semiNum),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	contentArea := container.NewStack(loadingScreen())

	go func() {
		qualifiers, err := scraper.FetchSemiQualifiers(year, semiNum)

		fyne.Do(func() {
			var content fyne.CanvasObject
			if err != nil {
				lbl := widget.NewLabel("Results not yet available.\nCheck back after the show airs.")
				lbl.Alignment = fyne.TextAlignCenter
				content = container.NewCenter(lbl)
			} else {
				content = buildSemiResultRows(acts, qualifiers, userPicks)
			}
			contentArea.Objects = []fyne.CanvasObject{content}
			contentArea.Refresh()
		})
	}()

	header := container.NewVBox(title, widget.NewSeparator())
	return container.NewBorder(header, nil, nil, nil, container.NewVScroll(contentArea))
}

var (
	colorLightGreen = color.NRGBA{R: 82, G: 247, B: 129, A: 80}
	colorLightRed   = color.NRGBA{R: 255, G: 86, B: 70, A: 80}
	colorGreen      = color.NRGBA{R: 19, G: 161, B: 59, A: 80}
	colorRed    = color.NRGBA{R: 200, G: 0, B: 0, A: 80}
)

func topTenFavorites(order []string) map[string]bool {
	favs := make(map[string]bool)
	limit := 10
	if len(order) < limit {
		limit = len(order)
	}
	for _, c := range order[:limit] {
		favs[c] = true
	}
	return favs
}

func rowPriority(picked, qualified bool) int {
	switch {
	case picked && qualified:
		return 0 // Gold: user's favorite that qualified
	case !picked && qualified:
		return 1 // Green: qualified but not a favorite
	case picked && !qualified:
		return 2 // Orange: user's favorite that was eliminated
	default:
		return 3 // Red: eliminated and not a favorite
	}
}

func buildSemiResultRows(acts []models.Act, qualifiers []string, userPicks map[string]bool) fyne.CanvasObject {
	qualMap := make(map[string]bool, len(qualifiers))
	for _, q := range qualifiers {
		qualMap[q] = true
	}

	favCount, favQualified := 0, 0
	for _, act := range acts {
		if userPicks[act.Country] {
			favCount++
			if qualMap[act.Country] {
				favQualified++
			}
		}
	}

	scoreLbl := widget.NewLabelWithStyle(
		fmt.Sprintf("%d of your %d favorites qualified", favQualified, favCount),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	sorted := make([]models.Act, len(acts))
	copy(sorted, acts)
	sort.SliceStable(sorted, func(i, j int) bool {
		pi := rowPriority(userPicks[sorted[i].Country], qualMap[sorted[i].Country])
		pj := rowPriority(userPicks[sorted[j].Country], qualMap[sorted[j].Country])
		return pi < pj
	})

	rows := container.NewVBox()
	for _, act := range sorted {
		qualified := qualMap[act.Country]
		picked := userPicks[act.Country]

		var rowColor color.Color
		switch rowPriority(picked, qualified) {
		case 0:
			rowColor = colorLightGreen
		case 1:
			rowColor = colorGreen
		case 2:
			rowColor = colorLightRed
		default:
			rowColor = colorRed
		}

		result := "Qualified"
		if !qualified {
			result = "Eliminated"
		}

		actLbl := widget.NewLabel(fmt.Sprintf("%s: %s — %s", act.Country, act.Artist, act.Song))
		actLbl.Wrapping = fyne.TextWrapWord

		resultLbl := widget.NewLabel(result)
		resultLbl.Alignment = fyne.TextAlignTrailing

		bg := canvas.NewRectangle(rowColor)
		row := container.NewBorder(nil, nil, nil, resultLbl, actLbl)
		rows.Add(container.NewStack(bg, row))
	}

	summary := container.NewVBox(scoreLbl, widget.NewSeparator())
	return container.NewBorder(summary, nil, nil, nil, rows)
}
