package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func coachRowsOrSkip(t *testing.T) []coachRow {
	t.Helper()
	if _, err := os.Stat(realDir); err != nil {
		t.Skip("real data not present")
	}
	rows := loadCoachRows(realDir)
	if len(rows) == 0 {
		t.Skip("coaches.csv not present")
	}
	return rows
}

func TestRealCoachesMatchFile(t *testing.T) {
	rows := coachRowsOrSkip(t)
	ct := &CoachTuning
	for _, season := range []int{2011, 2026} {
		lg, err := NewRealLeagueSeason(realDir, 1, season)
		if err != nil {
			t.Fatal(err)
		}
		abbrs := RealTeamAbbrevsFor(season)
		found := 0
		for _, tm := range lg.Teams {
			for _, r := range rows {
				if r.season != season || r.abbr != abbrs[tm.ID] {
					continue
				}
				found++
				c := tm.Coach
				if c.Name != r.name {
					t.Errorf("%d %s: coach %q, file %q", season, r.abbr, c.Name, r.name)
				}
				if r.birth > 0 && c.Age != clamp(season-r.birth, 28, 85) {
					t.Errorf("%d %s: age %d, want %d", season, r.abbr, c.Age, season-r.birth)
				}
				for _, v := range []int{c.Offense, c.Defense} {
					if v < ct.RatingMin || v > ct.RatingMax {
						t.Errorf("%s rating %d out of range", c.Name, v)
					}
				}
				if !ValidSystemKey(c.System) {
					t.Errorf("%s system %q", c.Name, c.System)
				}
				break
			}
		}
		if found != 30 {
			t.Errorf("%d: %d teams matched a coach row, want 30", season, found)
		}
	}
}

func TestRealCoachSameInEveryStartSeason(t *testing.T) {
	coachRowsOrSkip(t)
	a, err := NewRealLeagueSeason(realDir, 1, 2015)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewRealLeagueSeason(realDir, 1, 2016)
	if err != nil {
		t.Fatal(err)
	}
	same := 0
	for _, ta := range a.Teams {
		for _, tb := range b.Teams {
			if ta.Coach.Name == tb.Coach.Name {
				same++
				x, y := ta.Coach, tb.Coach
				if x.Offense != y.Offense || x.Defense != y.Defense || x.System != y.System {
					t.Errorf("%s differs between 2015 and 2016: %+v vs %+v", x.Name, *x, *y)
				}
			}
		}
	}
	if same == 0 {
		t.Error("no coach appears in both seasons")
	}
}

func TestRealCoachRatingAndSystemRules(t *testing.T) {
	// Average (zero) coach sits at the generated mean; extremes clamp.
	if got := coachRatingFromZ(0); got != int(CoachTuning.RatingMean) {
		t.Errorf("z 0 -> %d", got)
	}
	if got := coachRatingFromZ(50); got != CoachTuning.RatingMax {
		t.Errorf("z 50 -> %d", got)
	}
	if got := coachRatingFromZ(-50); got != CoachTuning.RatingMin {
		t.Errorf("z -50 -> %d", got)
	}
	// Shrink: one season is worth n/(n+3) = 0.25 of its z.
	p := shrinkProfile([]teamStat{{o: 2, d: -1, pace: 0, three: 0}})
	if p.off != 0.5 || p.def != -0.25 || p.n != 1 {
		t.Errorf("shrink: %+v", p)
	}
	// Defence is negated in seasonZ: lower d_rtg gives a higher z.
	z := seasonZ(map[string]teamStat{"A": {o: 100, d: 100}, "B": {o: 110, d: 110}})
	if z["A"].d <= 0 || z["B"].d >= 0 || z["B"].o <= 0 {
		t.Errorf("seasonZ: %+v", z)
	}
	cases := []struct {
		p    coachProfile
		want string
	}{
		{coachProfile{off: 0, def: 0.6}, "defense_first"},
		{coachProfile{off: 0.3, def: -0.5, pace: 0.5, three: 0.6}, "run_gun"},
		{coachProfile{off: 0.3, def: 0.1, pace: 0.5, three: 0.6}, "pace_space"},
		{coachProfile{off: 0, def: 0, pace: -0.5, three: -0.5}, "inside_out"},
		{coachProfile{off: 0.6, def: 0.5}, "star_led"},
		{coachProfile{}, "balanced"},
	}
	for _, c := range cases {
		if got := coachSystemFor(c.p); got != c.want {
			t.Errorf("%+v -> %s, want %s", c.p, got, c.want)
		}
	}
}

// Without coaches.csv the league keeps the generated coaches exactly.
func TestRealCoachesAbsentFileKeepsGenerated(t *testing.T) {
	if _, err := os.Stat(realDir); err != nil {
		t.Skip("real data not present")
	}
	dir := t.TempDir()
	ents, err := os.ReadDir(realDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if e.Name() == "coaches.csv" {
			continue
		}
		src, _ := filepath.Abs(filepath.Join(realDir, e.Name()))
		if err := os.Symlink(src, filepath.Join(dir, e.Name())); err != nil {
			t.Fatal(err)
		}
	}
	lg, err := NewRealLeagueSeason(dir, 5, 2016)
	if err != nil {
		t.Fatal(err)
	}
	for _, tm := range lg.Teams {
		want := newCoach(NewRNG(mix64(lg.Seed, coachOffset, 0, uint64(tm.ID))))
		if *tm.Coach != *want {
			t.Errorf("team %d: coach %+v, want generated %+v", tm.ID, *tm.Coach, *want)
		}
	}
}
