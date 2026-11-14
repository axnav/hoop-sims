package engine

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saves", "league.json")
	a := GenerateLeague(7)
	a.UserTeamID = 4
	a.StartSeason()
	a.SimDays(10)
	if err := a.Save(path); err != nil {
		t.Fatal(err)
	}
	b, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a.SimDays(20)
	b.SimDays(20)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("loaded league diverged from the unsaved one after 20 days")
	}
	if b.LastBox == nil {
		t.Fatal("expected a stored box score for the user team")
	}
}

func TestSteppedDraftAndHistory(t *testing.T) {
	lg := GenerateLeague(3)
	lg.UserTeamID = 5
	lg.StartSeason()
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	rep := lg.BeginOffseason()
	lg.DraftAutoUntilUser()
	if lg.DraftDone() || lg.DraftOnClock() != 5 {
		t.Fatal("expected the user team on the clock")
	}
	if err := lg.DraftPick(lg.Draft.Pool[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := lg.DraftPick(-1); err != nil && lg.DraftDone() {
		t.Fatal(err)
	}
	lg.DraftAutoAll()
	lg.FinishOffseason()
	if len(rep.Draft) != Progression.DraftSize || lg.Draft != nil || lg.Year != 2026 {
		t.Fatalf("draft picks %d, year %d", len(rep.Draft), lg.Year)
	}
	for _, tm := range lg.Teams {
		if len(tm.Roster) != RosterSize {
			t.Fatalf("team %d has %d players", tm.ID, len(tm.Roster))
		}
	}
	if len(lg.Records) != 1 || lg.Records[0].Champion < 0 {
		t.Fatal("season record missing")
	}
	p := lg.Teams[0].Roster[0]
	if p.Experience > 0 && len(lg.Careers[p.ID]) != 1 {
		t.Fatalf("career rows %d", len(lg.Careers[p.ID]))
	}
	for _, rp := range lg.Retired {
		if lg.FindPlayer(rp.ID) == nil {
			t.Fatal("retired player not findable")
		}
	}
}
