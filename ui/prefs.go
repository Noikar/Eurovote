package ui

import (
	"encoding/json"
	"fmt"

	"fyne.io/fyne/v2"
)

func saveSemiSelections(group int, order []string) {
	data, _ := json.Marshal(order)
	fyne.CurrentApp().Preferences().SetString(fmt.Sprintf("sf%d_selections", group), string(data))
}

func loadSemiSelections(group int) []string {
	var out []string
	raw := fyne.CurrentApp().Preferences().String(fmt.Sprintf("sf%d_selections", group))
	if raw == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func saveFinalVotes(votes map[string]int) {
	data, _ := json.Marshal(votes)
	fyne.CurrentApp().Preferences().SetString("final_votes", string(data))
}

func loadFinalVotes() map[string]int {
	out := make(map[string]int)
	raw := fyne.CurrentApp().Preferences().String("final_votes")
	if raw == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}
