package ui

import (
	"fmt"
	"image/color"
	"sort"

	"eurovote/models"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// NewSemiFinalScreen shows acts for the given semi-final group as a drag-to-rank list.
// The user's top 10 ranked acts are treated as their favorites.
func NewSemiFinalScreen(allActs []models.Act, group int, w fyne.Window) fyne.CanvasObject {
	var acts []models.Act
	for _, a := range allActs {
		if a.SemiGroup == group {
			acts = append(acts, a)
		}
	}

	savedOrder := loadSemiSelections(group)
	if len(savedOrder) > 0 {
		acts = reorderActsBySaved(acts, savedOrder)
	}

	list := make([]models.Act, len(acts))
	copy(list, acts)

	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Eurovision %d — Semi-final %d", currentYear(), group),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	subtitle := widget.NewLabel("Drag to rank your favorites. Only 10 will advance to the finals.")
	subtitle.Alignment = fyne.TextAlignCenter

	rows := container.NewVBox()
	highlightIdx := -1
	highlightBottom := false
	var drRows []*draggableRow
	findTarget := makeRowFinder(&drRows)

	refreshHighlight := func() {
		for _, dr := range drRows {
			dr.Refresh()
		}
	}

	favColor := color.NRGBA{R: 255, G: 200, B: 0, A: 50}

	var refresh func()
	refresh = func() {
		order := make([]string, len(list))
		for i, act := range list {
			order[i] = act.Country
		}
		saveSemiSelections(group, order)

		rows.Objects = nil
		drRows = nil
		for i, act := range list {
			i, act := i, act
			labelText := fmt.Sprintf("%s: %s - %s", act.Country, act.Artist, act.Song)
			handle := widget.NewLabel("≡")
			label := widget.NewLabel(fmt.Sprintf("%d.  %s", i+1, labelText))
			label.Wrapping = fyne.TextWrapWord
			rowContent := container.NewBorder(nil, nil, handle, nil, label)

			var rowWidget fyne.CanvasObject
			if i < 10 {
				bg := canvas.NewRectangle(favColor)
				rowWidget = container.NewStack(bg, rowContent)
			} else {
				rowWidget = rowContent
			}

			dr := newDraggableRow(rowWidget, labelText, i, &list, &highlightIdx, &highlightBottom, w, refresh, refreshHighlight, findTarget)
			drRows = append(drRows, dr)
			rows.Add(dr)
		}
		rows.Refresh()
	}

	refresh()

	if len(acts) == 0 {
		rows.Add(widget.NewLabel("No acts found for this semi-final."))
	}

	scroll := container.NewVScroll(rows)
	header := container.NewVBox(title, subtitle, widget.NewSeparator())
	return container.NewBorder(header, nil, nil, nil, scroll)
}

// reorderActsBySaved reorders acts to match a previously saved country order,
// appending any acts not in the saved list at the end.
func reorderActsBySaved(acts []models.Act, order []string) []models.Act {
	rankOf := make(map[string]int, len(order))
	for i, c := range order {
		rankOf[c] = i
	}
	sort.SliceStable(acts, func(i, j int) bool {
		ri, iSaved := rankOf[acts[i].Country]
		rj, jSaved := rankOf[acts[j].Country]
		if iSaved && jSaved {
			return ri < rj
		}
		return iSaved
	})
	return acts
}
