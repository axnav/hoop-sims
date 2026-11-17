package engine

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

var (
	fixOnce sync.Once
	fixLg   *League
	fixY1   Awards // season one's awards, copied before the offseason
)

// awardsFixture is seed 1 after two regular seasons (the second not yet
// archived), shared by the read-only tests below.
func awardsFixture() *League {
	fixOnce.Do(func() {
		lg := GenerateLeague(1)
		lg.UserTeamID = 3
		lg.StartSeason()
		lg.SimRegularSeason()
		fixY1 = lg.Season.Awards
		lg.SimPlayoffs()
		lg.Offseason()
		lg.StartSeason()
		lg.SimRegularSeason()
		fixLg = lg
	})
	return fixLg
}

func TestAwardsDeterministic(t *testing.T) {
	run := func() Awards {
		lg := GenerateLeague(5)
		lg.StartSeason()
		lg.SimRegularSeason()
		return lg.Season.Awards
	}
	a, b := run(), run()
	if !reflect.DeepEqual(a, b) {
		t.Fatal("awards differ between two runs of the same seed")
	}
	if a.DPOY.Name == "" || a.SixthMan.Name == "" || a.Coach.Name == "" || len(a.AllLeague) != 3 {
		t.Fatalf("expected DPOY, Sixth Man, Coach and three All-League teams in season one: %+v", a)
	}
}

func TestAwardWinnersMeetRules(t *testing.T) {
	lg := awardsFixture()
	for yi, a := range []Awards{fixY1, lg.Season.Awards} {
		s := lg.Season
		if yi == 0 {
			// season one: the initial league has no rookies, so no ROY or All-Rookie.
			if a.ROY.Name != "" || len(a.AllRookie) != 0 {
				t.Fatalf("season one has rookies: %+v %+v", a.ROY, a.AllRookie)
			}
			if a.MIP.Name != "" {
				t.Fatalf("season one has a Most Improved: %+v", a.MIP)
			}
			continue
		}
		byID := map[int]*Player{}
		for _, tm := range lg.Teams {
			for _, p := range tm.Roster {
				byID[p.ID] = p
			}
		}
		for _, p := range lg.FreeAgents {
			byID[p.ID] = p
		}
		if a.ROY.Name == "" || len(a.AllRookie) == 0 {
			t.Fatalf("season two should have a rookie class: %+v", a.ROY)
		}
		for _, aw := range append([]Award{a.ROY}, a.AllRookie...) {
			p := byID[aw.PlayerID]
			if !lg.seasonRookie(p, s) || aw.GP < AwardTuning.RookieMinGames {
				t.Fatalf("%s is not a qualified rookie", aw.Name)
			}
		}
		if len(a.AllRookie) != AwardTuning.AllRookieSize || a.AllRookie[0].PlayerID != a.ROY.PlayerID {
			t.Fatalf("all-rookie team: %+v, ROY %+v", a.AllRookie, a.ROY)
		}
		// Sixth man: not in his team's top five by minutes per game.
		sm := a.SixthMan
		better := 0
		smMpg := float64(s.Stats[sm.PlayerID].Min) / float64(s.Stats[sm.PlayerID].GP)
		for _, ps := range s.Stats {
			if ps.TeamID != sm.TeamID || ps.PlayerID == sm.PlayerID {
				continue
			}
			m := float64(ps.Min) / float64(ps.GP)
			if m > smMpg || (m == smMpg && ps.PlayerID < sm.PlayerID) {
				better++
			}
		}
		if better < AwardTuning.SixthTop || sm.GP < AwardTuning.MinGames {
			t.Fatalf("sixth man %s has only %d teammates ahead on minutes", sm.Name, better)
		}
		// Most improved: not a rookie, has last season's row and gained.
		if a.MIP.Name != "" {
			rows := lg.Careers[a.MIP.PlayerID]
			if len(rows) == 0 || lg.seasonRookie(byID[a.MIP.PlayerID], s) {
				t.Fatalf("most improved %s is a rookie or has no history", a.MIP.Name)
			}
			last := rows[len(rows)-1]
			if prodScore(a.MIP.PPG, a.MIP.RPG, a.MIP.APG) <= prodScore(last.PPG, last.RPG, last.APG) {
				t.Fatalf("most improved %s did not improve", a.MIP.Name)
			}
		}
		// Coach: the team's coach, record matches the standings.
		c := a.Coach
		if lg.Teams[c.TeamID].Coach.Name != c.Name || s.Wins[c.TeamID] != c.W || s.Losses[c.TeamID] != c.L {
			t.Fatalf("coach award %+v does not match the team", c)
		}
		// All-League: three teams of 2 G, 2 F, 1 C and no player twice.
		if len(a.AllLeague) != 3 {
			t.Fatalf("want 3 all-league teams, got %d", len(a.AllLeague))
		}
		seen := map[int]bool{}
		for i, tm := range a.AllLeague {
			if len(tm) != 5 {
				t.Fatalf("all-league team %d has %d players", i, len(tm))
			}
			var g [3]int
			for _, aw := range tm {
				if seen[aw.PlayerID] {
					t.Fatalf("%s is on two all-league teams", aw.Name)
				}
				seen[aw.PlayerID] = true
				g[posGroup(byID[aw.PlayerID].Pos)]++
			}
			if g != [3]int{2, 2, 1} {
				t.Fatalf("all-league team %d positions %v, want 2 G 2 F 1 C", i, g)
			}
		}
	}
}

func TestPlayerAwardsAndCurrent(t *testing.T) {
	lg := awardsFixture()
	y, a, ok := lg.CurrentAwards()
	if !ok || y != lg.Year || a.MVP.Name == "" {
		t.Fatalf("current awards: %v %v %+v", y, ok, a.MVP)
	}
	list := lg.PlayerAwards(a.MVP.PlayerID)
	found := false
	for _, pa := range list {
		found = found || (pa.Year == y && pa.Title == "MVP")
	}
	if !found {
		t.Fatalf("MVP missing from his award list: %v", list)
	}
	// Season one's MVP comes from the records, newest first.
	rec := lg.Records[0]
	if got := lg.PlayerAwards(rec.MVP.PlayerID); len(got) == 0 {
		t.Fatal("no awards from the records")
	}
	if got := lg.PlayerAwards(-5); got == nil || len(got) != 0 {
		t.Fatalf("unknown player must give an empty, non-nil list: %v", got)
	}
}

func TestOldSaveWithoutNewFieldsPlaysOn(t *testing.T) {
	lg := GenerateLeague(4)
	lg.StartSeason()
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	lg.Offseason()
	lg.StartSeason()
	lg.SimDays(10)
	path := filepath.Join(t.TempDir(), "league.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	raw, _ := readFile(path)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	delete(m, "News")
	strip := func(o map[string]any, keys ...string) {
		for _, k := range keys {
			delete(o, k)
		}
	}
	keys := []string{"ROY", "DPOY", "SixthMan", "MIP", "Coach", "AllLeague", "AllRookie"}
	strip(m["Season"].(map[string]any)["Awards"].(map[string]any), keys...)
	for _, r := range m["Records"].([]any) {
		strip(r.(map[string]any), append(keys, "Wins")...)
	}
	out, _ := json.Marshal(m)
	if err := writeFile(path, out); err != nil {
		t.Fatal(err)
	}
	old, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if old.News != nil || old.Season.Awards.ROY.Name != "" {
		t.Fatal("expected an old-style league")
	}
	old.SimRegularSeason()
	if old.Season.Awards.MVP.Name == "" || old.Season.Awards.ROY.Name == "" || old.Season.Awards.Coach.Name == "" {
		t.Fatalf("awards missing after playing on: %+v", old.Season.Awards)
	}
	old.SimPlayoffs()
	old.Offseason()
	old.StartSeason()
	if len(old.News) == 0 {
		t.Fatal("no news after playing on")
	}
}
