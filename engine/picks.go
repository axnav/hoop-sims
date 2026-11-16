package engine

import (
	"fmt"
	"sort"
)

// PickYearsAhead is how many drafts' worth of picks exist at any time: the
// upcoming draft and the ones after it.
const PickYearsAhead = 4

// DraftPick is one tradable draft selection. Year is the league Year during
// whose offseason the draft happens (the offseason runs before Year advances,
// so the draft that follows the 2025 season has Year 2025). Round is 1 or 2.
// Original is the team whose record decides the slot; Owner is the team that
// currently holds the pick and makes the selection.
type DraftPick struct {
	Year     int
	Round    int
	Original int
	Owner    int
}

// newPicksForYear returns both rounds of picks for one draft year, each owned
// by its original team.
func (lg *League) newPicksForYear(year int) []DraftPick {
	var out []DraftPick
	for round := 1; round <= 2; round++ {
		for _, t := range lg.Teams {
			out = append(out, DraftPick{Year: year, Round: round, Original: t.ID, Owner: t.ID})
		}
	}
	return out
}

// initPicks creates the full window of picks, starting at the current Year.
func (lg *League) initPicks() {
	lg.Picks = nil
	for y := lg.Year; y < lg.Year+PickYearsAhead; y++ {
		lg.Picks = append(lg.Picks, lg.newPicksForYear(y)...)
	}
}

// PicksOwnedBy returns the picks a team owns, sorted by year, round, then
// original team.
func (lg *League) PicksOwnedBy(teamID int) []DraftPick {
	var out []DraftPick
	for _, p := range lg.Picks {
		if p.Owner == teamID && !lg.pickUsed(p.Year, p.Round, p.Original) {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Year != b.Year {
			return a.Year < b.Year
		}
		if a.Round != b.Round {
			return a.Round < b.Round
		}
		return a.Original < b.Original
	})
	return out
}

func (lg *League) findPick(year, round, original int) int {
	for i, p := range lg.Picks {
		if p.Year == year && p.Round == round && p.Original == original {
			return i
		}
	}
	return -1
}

// PickOwner returns the current owner of a pick, and false if it does not exist.
func (lg *League) PickOwner(year, round, original int) (int, bool) {
	i := lg.findPick(year, round, original)
	if i < 0 {
		return 0, false
	}
	return lg.Picks[i].Owner, true
}

// TransferPick moves a pick from one team to another. It fails if the pick
// does not exist or fromTeam does not own it.
func (lg *League) TransferPick(year, round, original, fromTeam, toTeam int) error {
	i := lg.findPick(year, round, original)
	if i < 0 {
		return fmt.Errorf("no such pick: year %d round %d original team %d", year, round, original)
	}
	if lg.pickUsed(year, round, original) {
		return fmt.Errorf("that pick has already been used")
	}
	if lg.Picks[i].Owner != fromTeam {
		return fmt.Errorf("team %d does not own that pick", fromTeam)
	}
	if toTeam < 0 || toTeam >= len(lg.Teams) {
		return fmt.Errorf("no such team %d", toTeam)
	}
	lg.Picks[i].Owner = toTeam
	return nil
}

// rollPicks drops the picks of a finished draft year and adds the picks for
// the new furthest year, keeping the window at PickYearsAhead years.
func (lg *League) rollPicks(doneYear int) {
	kept := lg.Picks[:0]
	for _, p := range lg.Picks {
		if p.Year != doneYear {
			kept = append(kept, p)
		}
	}
	lg.Picks = append(kept, lg.newPicksForYear(doneYear+PickYearsAhead)...)
}
