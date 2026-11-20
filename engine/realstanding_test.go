package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func fixtureData() *realData {
	d := &realData{
		all:    map[int]map[string]*statRow{2015: {}},
		career: map[string]careerInfo{}, models: map[int]*seasonModel{},
		rated: map[int]map[string]*ratedPlayer{}, target: map[int]map[string]float64{},
	}
	pos := []string{"PG", "SG", "SF", "PF", "C"}
	for i := 0; i < 40; i++ {
		f := float64(i)
		id := "p" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		d.all[2015][id] = &statRow{
			season: 2015, pid: id, name: id, pos: pos[i%5], age: 25,
			g: 70, gs: 60, mp: 1500 + 30*f,
			fg: 300 + f, fga: 700, x3p: 60 + f, x3pa: 180, x2p: 240, x2pa: 520, ft: 150, fta: 190,
			orb: 60, drb: 200, trb: 260, ast: 150 + 2*f, stl: 50, blk: 30, tov: 100, pts: 900 + 10*f,
			per: 10 + f/3, ts: 0.5, usg: 18 + f/4, orbp: 4, drbp: 12, trbp: 8, astp: 12, stlp: 1.5, blkp: 1,
			tovp: 12, ows: 2, dws: 2, ws: 2 + f/5, ws48: 0.05 + f/400, obpm: 0, dbpm: 0, bpm: -2 + f/8, vorp: f / 10,
		}
	}
	return d
}

// A target sets the visible overall; the ratings keep their shape and the
// hidden Impact keeps the formula's value-minus-overall gap.
func TestStandingTargetOverride(t *testing.T) {
	d := fixtureData()
	row := d.all[2015]["paa"]
	free := d.rate(row, 0)
	d.target[2015] = map[string]float64{"paa": 90}
	set := d.rate(row, 0)
	if set.Overall < 89 || set.Overall > 91 {
		t.Fatalf("target 90 gave overall %d (formula %d)", set.Overall, free.Overall)
	}
	if free.Overall > 80 {
		t.Fatalf("fixture player should start well below 90, got %d", free.Overall)
	}
	if set.Pos != free.Pos {
		t.Fatalf("position changed")
	}
	// Same player through ratedFor sees the override too.
	if rp := d.ratedFor(2015, "paa"); rp.Overall != set.Overall {
		t.Fatalf("ratedFor %d vs rate %d", rp.Overall, set.Overall)
	}
	// Another player is untouched.
	other := d.all[2015]["pba"]
	a := d.rate(other, 0)
	delete(d.target[2015], "paa")
	b := d.rate(other, 0)
	if a.Overall != b.Overall || a.Ratings != b.Ratings {
		t.Fatalf("target for one player changed another")
	}
}

func TestLoadStanding(t *testing.T) {
	dir := t.TempDir()
	if rows := LoadStanding(dir); rows != nil {
		t.Fatalf("missing file should give no rows")
	}
	csv := "season,player_id,player,band,target\n2015,a01,A,top,\n2015,b01,B,top,88\n"
	if err := os.WriteFile(filepath.Join(dir, "standing.csv"), []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	rows := LoadStanding(dir)
	if len(rows) != 2 || rows[0].HasTgt || !rows[1].HasTgt || rows[1].Target != 88 {
		t.Fatalf("rows = %+v", rows)
	}
	tg := loadStandingTargets(dir)
	if len(tg[2015]) != 1 || tg[2015]["b01"] != 88 {
		t.Fatalf("targets = %v", tg)
	}
}

// Real-data standing: skipped without the private reference file.
func TestRealStandingMatchesReference(t *testing.T) {
	if _, err := os.Stat(filepath.Join(realDir, "standing.csv")); err != nil {
		t.Skip("standing.csv not present")
	}
	for y := 2011; y <= 2025; y++ {
		rep, err := RealRatingsReport(realDir, 1, y)
		if err != nil {
			t.Fatal(err)
		}
		if rep.RefTop30 < 11 {
			t.Errorf("%d: only %d of %d reference players in the top 30", y, rep.RefTop30, rep.Ref)
		}
		for _, r := range rep.Rows {
			if r.MPG < 15 && r.Ovr >= 75 {
				t.Errorf("%d: %s plays %.1f mpg and is rated %d", y, r.Name, r.MPG, r.Ovr)
			}
		}
		if rep.Under20In40 != 0 {
			t.Errorf("%d: %d players under 20 mpg in the top 40", y, rep.Under20In40)
		}
	}
}
