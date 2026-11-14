package engine

import (
	"path/filepath"
	"testing"
)

func TestNewLeaguePicks(t *testing.T) {
	lg := GenerateLeague(3)
	if len(lg.Picks) != 30*2*PickYearsAhead {
		t.Fatalf("got %d picks", len(lg.Picks))
	}
	for _, p := range lg.Picks {
		if p.Owner != p.Original || p.Year < lg.Year || p.Year >= lg.Year+PickYearsAhead {
			t.Fatalf("bad pick %+v", p)
		}
	}
	if n := len(lg.PicksOwnedBy(5)); n != 2*PickYearsAhead {
		t.Errorf("team 5 owns %d", n)
	}
}

func TestDraftOrderMatchesStandings(t *testing.T) {
	lg := finishedSeasonLeague(4)
	st := lg.Season.Standings()
	lg.BeginOffseason()
	for i := 0; i < 60; i++ {
		want := st[len(st)-1-i%30]
		if got := lg.DraftOnClock(); got != want {
			t.Fatalf("pick %d: team %d, want %d", i+1, got, want)
		}
		lg.DraftPick(-1)
	}
}

func TestTransferredPickSelects(t *testing.T) {
	lg := finishedSeasonLeague(4)
	st := lg.Season.Standings()
	first, buyer := st[len(st)-1], st[0]
	if err := lg.TransferPick(lg.Year, 1, first, first, buyer); err != nil {
		t.Fatal(err)
	}
	lg.BeginOffseason()
	if got := lg.DraftOnClock(); got != buyer {
		t.Errorf("on clock %d, want new owner %d", got, buyer)
	}
	lg.DraftPick(-1)
	if got := lg.Report.Draft[0].TeamID; got != buyer {
		t.Errorf("result team %d, want %d", got, buyer)
	}
	for i := 1; i < 30; i++ {
		lg.DraftPick(-1)
	}
	if got := lg.DraftOnClock(); got != first {
		t.Errorf("round 2 first pick team %d, want %d", got, first)
	}
}

func TestTransferPickErrors(t *testing.T) {
	lg := GenerateLeague(5)
	if err := lg.TransferPick(lg.Year, 1, 0, 1, 2); err == nil {
		t.Error("wrong owner should fail")
	}
	if err := lg.TransferPick(lg.Year+PickYearsAhead, 1, 0, 0, 2); err == nil {
		t.Error("missing pick should fail")
	}
	if err := lg.TransferPick(lg.Year, 1, 0, 0, 2); err != nil {
		t.Fatal(err)
	}
	if o, ok := lg.PickOwner(lg.Year, 1, 0); !ok || o != 2 {
		t.Errorf("owner %d %v", o, ok)
	}
}

func TestPickWindowAfterOffseason(t *testing.T) {
	lg := finishedSeasonLeague(6)
	start := lg.Year
	lg.Offseason()
	if lg.Year != start+1 {
		t.Fatalf("year %d", lg.Year)
	}
	years := map[int]int{}
	for _, p := range lg.Picks {
		years[p.Year]++
	}
	if len(years) != PickYearsAhead {
		t.Fatalf("years %v", years)
	}
	for y := lg.Year; y < lg.Year+PickYearsAhead; y++ {
		if years[y] != 60 {
			t.Errorf("year %d has %d picks", y, years[y])
		}
	}
}

func TestLoadVersion1SaveGetsPicks(t *testing.T) {
	lg := GenerateLeague(8)
	lg.Picks = nil
	path := filepath.Join(t.TempDir(), "league.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	rewriteVersion(t, path, 1)
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != SaveVersion || len(got.Picks) != 30*2*PickYearsAhead {
		t.Errorf("version %d, picks %d", got.Version, len(got.Picks))
	}
}

func finishedSeasonLeague(seed uint64) *League {
	lg := GenerateLeague(seed)
	lg.StartSeason()
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	return lg
}
