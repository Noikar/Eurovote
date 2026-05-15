package ui

import (
	"fmt"

	"eurovote/models"
	"eurovote/scraper"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// NewFinalResultsScreen shows the Grand Final rankings alongside the user's
// saved vote distribution. Results are fetched asynchronously from Wikipedia.
func NewFinalResultsScreen(allActs []models.Act, year int, w fyne.Window) fyne.CanvasObject {
	userVotes := loadFinalVotes()

	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Eurovision %d — Grand Final Results", year),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	contentArea := container.NewStack(loadingScreen())

	go func() {
		rankings, err := scraper.FetchFinalRankings(year)

		fyne.Do(func() {
			var content fyne.CanvasObject
			if err != nil {
				lbl := widget.NewLabel("Results not yet available.\nCheck back after the show airs.")
				lbl.Alignment = fyne.TextAlignCenter
				content = container.NewCenter(lbl)
			} else {
				content = buildFinalResultRows(allActs, rankings, userVotes)
			}
			contentArea.Objects = []fyne.CanvasObject{content}
			contentArea.Refresh()
		})
	}()

	header := container.NewVBox(title, widget.NewSeparator())
	return container.NewBorder(header, nil, nil, nil, container.NewVScroll(contentArea))
}

func buildFinalResultRows(allActs []models.Act, rankings []models.FinalEntry, userVotes map[string]int) fyne.CanvasObject {
	actMap := make(map[string]models.Act, len(allActs))
	for _, a := range allActs {
		actMap[a.Country] = a
	}

	// Count how many of the user's top-voted countries placed in the top 10.
	top10 := make(map[string]bool)
	for _, e := range rankings {
		if e.Place <= 10 {
			top10[e.Country] = true
		}
	}
	userTop10Hits := 0
	totalUserVotes := 0
	for country, v := range userVotes {
		if v > 0 {
			totalUserVotes++
			if top10[country] {
				userTop10Hits++
			}
		}
	}

	var summaryText string
	if totalUserVotes > 0 {
		summaryText = fmt.Sprintf("%d of the countries you voted for placed in the Top 10", userTop10Hits)
	} else {
		summaryText = "You hadn't cast any votes before the Final."
	}

	scoreLbl := widget.NewLabelWithStyle(summaryText, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	rows := container.NewVBox()
	for _, entry := range rankings {
		act := actMap[entry.Country]
		votes := userVotes[entry.Country]

		placeLbl := widget.NewLabelWithStyle(
			fmt.Sprintf("%d.", entry.Place),
			fyne.TextAlignLeading,
			fyne.TextStyle{Bold: true},
		)

		name := entry.Country
		if act.Artist != "" {
			name = fmt.Sprintf("%s: %s — %s", entry.Country, act.Artist, act.Song)
		}
		actLbl := widget.NewLabel(name)
		actLbl.Wrapping = fyne.TextWrapWord

		var voteLbl *widget.Label
		if votes > 0 {
			voteLbl = widget.NewLabel(fmt.Sprintf("Your votes: %d", votes))
		} else {
			voteLbl = widget.NewLabel("—")
			voteLbl.TextStyle = fyne.TextStyle{Italic: true}
		}
		voteLbl.Alignment = fyne.TextAlignTrailing

		row := container.NewBorder(nil, nil, container.NewPadded(placeLbl), voteLbl, actLbl)
		rows.Add(row)
	}

	summary := container.NewVBox(scoreLbl, widget.NewSeparator())
	return container.NewBorder(summary, nil, nil, nil, rows)
}
