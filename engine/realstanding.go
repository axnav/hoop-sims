package engine

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// standing.csv (private, optional, data/real/): season, player_id, player,
// band, target. Band "top" marks that season's reference top players (used
// only by the measuring tool and its test). A numeric target sets that
// player-season's visible overall in rate (see realdata.go).

// StandingRow is one line of standing.csv.
type StandingRow struct {
	Season   int
	PlayerID string
	Name     string
	Band     string
	Target   float64
	HasTgt   bool
}

// LoadStanding reads standing.csv from dir; a missing file gives no rows.
func LoadStanding(dir string) []StandingRow {
	rows, err := readCSV(filepath.Join(dir, "standing.csv"))
	if err != nil {
		return nil
	}
	var out []StandingRow
	for _, m := range rows {
		s := StandingRow{Season: int(num(m["season"])), PlayerID: m["player_id"], Name: m["player"], Band: m["band"]}
		if t := strings.TrimSpace(m["target"]); t != "" {
			if v, err := strconv.ParseFloat(t, 64); err == nil {
				s.Target, s.HasTgt = v, true
			}
		}
		out = append(out, s)
	}
	return out
}

func loadStandingTargets(dir string) map[int]map[string]float64 {
	out := map[int]map[string]float64{}
	for _, s := range LoadStanding(dir) {
		if !s.HasTgt {
			continue
		}
		if out[s.Season] == nil {
			out[s.Season] = map[string]float64{}
		}
		out[s.Season][s.PlayerID] = s.Target
	}
	return out
}

// RealRow is one player of a real league in the measuring report.
type RealRow struct {
	ID        string
	Name      string
	Team      string
	Age       int
	Pos       string
	MPG, VORP float64
	Ovr, Pot  int
	Rat       [7]int // inside, mid, three, playmaking, rebounding, defence, athleticism
	Top       bool   // band "top" in standing.csv
}

// RealReport is the league of one real season, best overall first.
type RealReport struct {
	Season                  int
	Rows                    []RealRow
	RefTop15, RefTop30, Ref int // reference players in the game's top 15 / top 30, and how many there are
	Under20In40             int // players under 20 minutes a game inside the top 40
	CorrVORP                float64
	Mean                    float64 // mean overall of the listed players
	N90                     int     // players rated 90 or more
}

// RealRatingsReport builds the real league of a season and measures it
// against the reference list. Never carries Impact.
func RealRatingsReport(dir string, seed uint64, season int) (*RealReport, error) {
	lg, err := NewRealLeagueSeason(dir, seed, season)
	if err != nil {
		return nil, err
	}
	d, err := loadRealDataFrom(dir, season)
	if err != nil {
		return nil, err
	}
	top := map[string]bool{}
	for _, s := range LoadStanding(dir) {
		if s.Season == season && s.Band == "top" {
			top[s.PlayerID] = true
		}
	}
	rep := &RealReport{Season: season, Ref: len(top)}
	add := func(p *Player, team string) {
		row := d.all[season][p.RealID]
		if row == nil {
			return
		}
		rt := p.Ratings
		rep.Rows = append(rep.Rows, RealRow{
			ID: p.RealID, Name: p.Name, Team: team, Age: p.Age, Pos: p.PosLabel(),
			MPG: mpgOf(row), VORP: row.vorp, Ovr: p.Overall(), Pot: p.Potential,
			Rat: [7]int{rt.Inside, rt.Mid, rt.Three, rt.Playmaking, rt.Rebounding, rt.Defense, rt.Athleticism},
			Top: top[p.RealID],
		})
	}
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			add(p, t.Name)
		}
	}
	for _, p := range lg.FreeAgents {
		add(p, "FA")
	}
	sort.SliceStable(rep.Rows, func(i, j int) bool {
		a, b := rep.Rows[i], rep.Rows[j]
		if a.Ovr != b.Ovr {
			return a.Ovr > b.Ovr
		}
		if a.VORP != b.VORP {
			return a.VORP > b.VORP
		}
		return a.ID < b.ID
	})
	for i, r := range rep.Rows {
		if r.Top && i < 15 {
			rep.RefTop15++
		}
		if r.Top && i < 30 {
			rep.RefTop30++
		}
		if i < 40 && r.MPG < 20 {
			rep.Under20In40++
		}
	}
	var xs, ys []float64
	for _, r := range rep.Rows {
		if d.all[season][r.ID].mp >= RealTuning.MinQual {
			xs = append(xs, float64(r.Ovr))
			ys = append(ys, r.VORP)
		}
	}
	rep.CorrVORP = correlation(xs, ys)
	for _, r := range rep.Rows {
		rep.Mean += float64(r.Ovr) / float64(len(rep.Rows))
		if r.Ovr >= 90 {
			rep.N90++
		}
	}
	return rep, nil
}

func correlation(x, y []float64) float64 {
	if len(x) < 2 {
		return 0
	}
	mx, sx := meanSD(x)
	my, sy := meanSD(y)
	c := 0.0
	for i := range x {
		c += (x[i] - mx) * (y[i] - my)
	}
	return c / float64(len(x)) / (sx * sy)
}

// Summary is the one-line summary the measuring tool prints per season.
func (r *RealReport) Summary() string {
	return fmt.Sprintf("season %d  ref-in-top15 %d/%d  ref-in-top30 %d/%d  under20mpg-in-top40 %d  corr(ovr,vorp) %.3f",
		r.Season, r.RefTop15, r.Ref, r.RefTop30, r.Ref, r.Under20In40, math.Round(r.CorrVORP*1000)/1000)
}
