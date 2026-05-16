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

// applyRunOrder fetches the Grand Final draw order from Wikipedia and sorts
// acts by their running number. Falls back to alphabetical if unavailable.
func applyRunOrder(acts []models.Act) []models.Act {
	order, err := scraper.FetchFinalRunOrder(currentYear())
	if err == nil {
		for i := range acts {
			if n, ok := order[acts[i].Country]; ok {
				acts[i].RunOrder = n
			}
		}
	}
	sort.Slice(acts, func(i, j int) bool {
		ri, rj := acts[i].RunOrder, acts[j].RunOrder
		if ri == 0 && rj == 0 {
			return acts[i].Country < acts[j].Country
		}
		if ri == 0 {
			return false
		}
		if rj == 0 {
			return true
		}
		return ri < rj
	})
	return acts
}

const totalVotes = 20

// NewFinalScreen resolves the qualifying acts (from saved semi-final rankings or
// Wikipedia) then shows the Grand Final voting screen.
func NewFinalScreen(allActs []models.Act, w fyne.Window) fyne.CanvasObject {
	wrapper := container.NewStack(
		container.NewCenter(widget.NewProgressBarInfinite()),
	)

	go func() {
		acts := applyRunOrder(resolveQualifiedActs(allActs))
		screen := buildFinalScreen(acts, w)
		fyne.Do(func() {
			wrapper.Objects = []fyne.CanvasObject{screen}
			wrapper.Refresh()
		})
	}()

	return wrapper
}

// resolveQualifiedActs returns only the acts that qualify for the Grand Final:
// auto-qualifiers (SemiGroup == 0), plus the top 10 from each semi-final.
// If no saved rankings exist for a semi, it falls back to Wikipedia.
func resolveQualifiedActs(allActs []models.Act) []models.Act {
	var result []models.Act

	for _, a := range allActs {
		if a.SemiGroup == 0 {
			result = append(result, a)
		}
	}

	for _, group := range []int{1, 2} {
		saved := loadSemiSelections(group)
		if len(saved) > 0 {
			cutoff := 10
			if len(saved) < cutoff {
				cutoff = len(saved)
			}
			qualSet := make(map[string]bool, cutoff)
			for _, c := range saved[:cutoff] {
				qualSet[c] = true
			}
			for _, a := range allActs {
				if a.SemiGroup == group && qualSet[a.Country] {
					result = append(result, a)
				}
			}
		} else {
			// No user rankings saved — pull real qualifiers from Wikipedia.
			qs, err := scraper.FetchSemiQualifiers(currentYear(), group)
			if err == nil && len(qs) > 0 {
				qualSet := make(map[string]bool, len(qs))
				for _, c := range qs {
					qualSet[c] = true
				}
				for _, a := range allActs {
					if a.SemiGroup == group && qualSet[a.Country] {
						result = append(result, a)
					}
				}
			} else {
				// Wikipedia unavailable — show all acts from this semi as a fallback.
				for _, a := range allActs {
					if a.SemiGroup == group {
						result = append(result, a)
					}
				}
			}
		}
	}

	return result
}

func buildFinalScreen(acts []models.Act, w fyne.Window) fyne.CanvasObject {
	// Restore any previously saved votes.
	state := models.NewVoteState(totalVotes)
	for country, v := range loadFinalVotes() {
		for i := 0; i < v; i++ {
			state.Add(country)
		}
	}

	remainingLabel := widget.NewLabelWithStyle(
		remainingText(state.Remaining),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	// Results panel — updated live.
	resultsBox := container.NewVBox()

	var refreshResults func()
	refreshResults = func() {
		resultsBox.Objects = nil

		type entry struct {
			country string
			votes   int
		}
		var entries []entry
		for country, v := range state.Votes {
			if v > 0 {
				entries = append(entries, entry{country, v})
			}
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].votes > entries[j].votes
		})

		if len(entries) == 0 {
			resultsBox.Add(widget.NewLabel("No votes assigned yet."))
		} else {
			for rank, e := range entries {
				resultsBox.Add(widget.NewLabel(
					fmt.Sprintf("%d. %s — %d vote(s)", rank+1, e.country, e.votes),
				))
			}
		}
		resultsBox.Refresh()
	}

	// Vote rows — one per act.
	voteRows := container.NewVBox()
	countLabels := make(map[string]*widget.Label)
	plusBtns := make(map[string]*widget.Button)
	minusBtns := make(map[string]*widget.Button)

	refreshVoteControls := func() {
		remainingLabel.SetText(remainingText(state.Remaining))
		for country, plusBtn := range plusBtns {
			if state.Remaining <= 0 {
				plusBtn.Disable()
			} else {
				plusBtn.Enable()
			}
			if state.Votes[country] <= 0 {
				minusBtns[country].Disable()
			} else {
				minusBtns[country].Enable()
			}
			countLabels[country].SetText(fmt.Sprintf("%d", state.Votes[country]))
		}
		refreshResults()
		w.Canvas().Refresh(w.Content())
	}

	for _, act := range acts {
		act := act

		countLbl := widget.NewLabel("0")
		countLbl.Alignment = fyne.TextAlignCenter
		countLabels[act.Country] = countLbl

		plusBtn := widget.NewButton("+", func() {
			state.Add(act.Country)
			saveFinalVotes(state.Votes)
			refreshVoteControls()
		})
		minusBtn := widget.NewButton("−", func() {
			state.Remove(act.Country)
			saveFinalVotes(state.Votes)
			refreshVoteControls()
		})
		minusBtn.Disable() // starts at 0, enabled by refreshVoteControls if votes loaded

		plusBtns[act.Country] = plusBtn
		minusBtns[act.Country] = minusBtn

		var actText string
		if act.RunOrder > 0 {
			actText = fmt.Sprintf("%d. %s: %s - %s", act.RunOrder, act.Country, act.Artist, act.Song)
		} else {
			actText = fmt.Sprintf("%s: %s - %s", act.Country, act.Artist, act.Song)
		}
		actLabel := widget.NewLabel(actText)
		actLabel.Wrapping = fyne.TextWrapWord

		scrollPad := canvas.NewRectangle(color.Transparent)
		scrollPad.SetMinSize(fyne.NewSize(14, 0))
		controls := container.NewHBox(minusBtn, countLbl, plusBtn, scrollPad)
		row := container.NewBorder(nil, nil, nil, controls, actLabel)
		voteRows.Add(row)
	}

	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Eurovision %d — Grand Final", currentYear()),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	subtitle := widget.NewLabel("Distribute your 20 votes across the acts.")
	subtitle.Alignment = fyne.TextAlignCenter

	resultsTitle := widget.NewLabelWithStyle("Your Scoreboard", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	// Sync UI with any restored vote state.
	refreshVoteControls()

	voteScroll := container.NewVScroll(voteRows)
	resultsScroll := container.NewVScroll(container.NewVBox(resultsTitle, resultsBox))

	split := container.NewHSplit(voteScroll, resultsScroll)
	split.SetOffset(0.6)

	header := container.NewVBox(title, subtitle, remainingLabel, widget.NewSeparator())
	return container.NewBorder(header, nil, nil, nil, split)
}

func remainingText(remaining int) string {
	return fmt.Sprintf("Votes remaining: %d / %d", remaining, totalVotes)
}
