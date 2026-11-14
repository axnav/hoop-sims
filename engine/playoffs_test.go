package engine

import (
	"path/filepath"
	"reflect"
	"testing"
)

func readyLeague(seed uint64, user int) *League {
	lg := GenerateLeague(seed)
	lg.UserTeamID = user
	lg.StartSeason()
	lg.SimRegularSeason()
	return lg
}

func TestPlayoffStepEquivalence(t *testing.T) {
	a := readyLeague(11, 3)
	b := readyLeague(11, 3)
	if err := a.SimPlayoffs(); err != nil {
		t.Fatal(err)
	}
	if err := b.StartPlayoffs(); err != nil {
		t.Fatal(err)
	}
	for b.SimPlayoffDay() {
	}
	if !reflect.DeepEqual(a.Season, b.Season) {
		t.Fatal("day-by-day playoffs differ from simming all at once")
	}
	// mixed controls
	c := readyLeague(11, 3)
	c.StartPlayoffs()
	c.SimPlayoffDay()
	c.SimUserSeries()
	c.SimRound()
	c.SimAllPlayoffs()
	if !reflect.DeepEqual(a.Season, c.Season) {
		t.Fatal("mixed controls differ")
	}
	s := a.Season
	if len(s.Series) != 15 || s.Champion < 0 || s.Awards.FinalsMVP.Name == "" {
		t.Fatalf("series %d champ %d mvp %q", len(s.Series), s.Champion, s.Awards.FinalsMVP.Name)
	}
	if s.Awards.FinalsMVP.TeamID != s.Champion {
		t.Fatal("finals MVP not on champion")
	}
	if len(s.PlayoffBoxes) != s.PlayoffGames || s.PlayoffGames == 0 || len(s.PlayoffStats) == 0 {
		t.Fatal("playoff boxes/stats missing")
	}
}

func TestSaveLoadMidPlayoffs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "l.json")
	a := readyLeague(5, 7)
	a.StartPlayoffs()
	for i := 0; i < 5; i++ {
		a.SimPlayoffDay()
	}
	if err := a.Save(path); err != nil {
		t.Fatal(err)
	}
	b, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a.SimAllPlayoffs()
	b.SimAllPlayoffs()
	if !reflect.DeepEqual(a, b) {
		t.Fatal("loaded mid-playoff league diverged")
	}
}

func TestRecordingDoesNotChangeGame(t *testing.T) {
	lg := GenerateLeague(2)
	x := SimGameRec(NewRNG(9), lg.Teams[1], lg.Teams[2], false)
	y := SimGameRec(NewRNG(9), lg.Teams[1], lg.Teams[2], true)
	if x.Events != nil || len(y.Events) == 0 {
		t.Fatal("events only expected with recording")
	}
	y.Events = nil
	if !reflect.DeepEqual(x, y) {
		t.Fatal("recording changed the game")
	}
}

// Replaying the events must rebuild the box score exactly (mirrors the JS viewer).
func TestEventsRebuildBox(t *testing.T) {
	lg := GenerateLeague(4)
	for seed := uint64(1); seed <= 20; seed++ {
		res := SimGameRec(NewRNG(seed), lg.Teams[3], lg.Teams[8], true)
		got := map[int]*StatLine{}
		for _, tb := range []*TeamBox{res.Home, res.Away} {
			for _, l := range tb.Lines {
				got[l.Player.ID] = &StatLine{}
			}
		}
		for _, e := range res.Events {
			l := got[e.Player]
			switch e.Type {
			case EvMade2, EvMade3:
				l.FGM++
				l.FGA++
				l.Pts += 2
				if e.Type == EvMade3 {
					l.TPM++
					l.TPA++
					l.Pts++
				}
				if e.Ast >= 0 {
					got[e.Ast].Ast++
				}
			case EvMiss2, EvMiss3:
				l.FGA++
				if e.Type == EvMiss3 {
					l.TPA++
				}
			case EvFT:
				l.FTA += 2
				l.FTM += e.N
				l.Pts += e.N
			case EvTov:
				l.Tov++
			case EvOReb, EvDReb:
				l.Reb++
			}
		}
		for _, tb := range []*TeamBox{res.Home, res.Away} {
			for _, l := range tb.Lines {
				g := got[l.Player.ID]
				want := StatLine{Pts: l.Pts, Reb: l.Reb, Ast: l.Ast, Tov: l.Tov, FGM: l.FGM, FGA: l.FGA, TPM: l.TPM, TPA: l.TPA, FTM: l.FTM, FTA: l.FTA}
				if *g != want {
					t.Fatalf("seed %d player %d: events %+v box %+v", seed, l.Player.ID, *g, want)
				}
			}
		}
		last := res.Events[len(res.Events)-1]
		if last.HS != res.Home.Score || last.AS != res.Away.Score {
			t.Fatalf("seed %d final score mismatch", seed)
		}
	}
}
