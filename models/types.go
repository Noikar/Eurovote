package models

import "time"

// ContestDates holds the specific calendar dates for each Eurovision event.
type ContestDates struct {
	SF1   time.Time
	SF2   time.Time
	Final time.Time
}

// Act represents a single Eurovision entry.
type Act struct {
	Country   string
	Artist    string
	Song      string
	SemiGroup int // 1 = SF1, 2 = SF2, 0 = auto-qualifier (Big 5 + host)
	RunOrder  int // Grand Final running order draw number (0 = unknown)
}

// FinalEntry holds a country's placement in the Grand Final results.
type FinalEntry struct {
	Place   int
	Country string
}

// VoteState tracks the user's vote allocation for the Grand Final.
type VoteState struct {
	Votes     map[string]int // keyed by Country
	Remaining int
}

func NewVoteState(total int) *VoteState {
	return &VoteState{
		Votes:     make(map[string]int),
		Remaining: total,
	}
}

func (v *VoteState) Add(country string) bool {
	if v.Remaining <= 0 {
		return false
	}
	v.Votes[country]++
	v.Remaining--
	return true
}

func (v *VoteState) Remove(country string) bool {
	if v.Votes[country] <= 0 {
		return false
	}
	v.Votes[country]--
	v.Remaining++
	return true
}
