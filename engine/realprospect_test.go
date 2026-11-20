package engine

import (
	"math"
	"os"
	"testing"
)

// pedFixtureRows: three bands (picks 1-2, 3-10, 11-60). Peak falls 2 points per
// year of draft age; the first-season overall rises 1 point per year.
func pedFixtureRows() []pedRow {
	var rows []pedRow
	add := func(pick int, base float64, ages ...int) {
		for _, a := range ages {
			rows = append(rows, pedRow{pick: pick, age: a, peak: base - 2*float64(a-20), first: base - 30 + float64(a-20)})
		}
	}
	add(1, 90, 19, 20, 21)
	add(2, 92, 19, 20, 21)
	add(5, 80, 19, 20, 21)
	add(8, 78, 19, 20, 21)
	add(20, 65, 19, 20, 21)
	add(40, 55, 19, 20, 21)
	return rows
}

func TestPedigreeFitFixture(t *testing.T) {
	edges := []int{2, 10, 60}
	pg := fitPedigree(pedFixtureRows(), edges)
	if len(pg.bands) != 3 || pg.bands[0].n != 6 || pg.bands[1].n != 6 || pg.bands[2].n != 6 {
		t.Fatalf("band sizes wrong: %+v", pg.bands)
	}
	if math.Abs(pg.bands[0].peakMean-91) > 1e-9 || math.Abs(pg.bands[1].peakMean-79) > 1e-9 {
		t.Fatalf("band peak means %v %v, want 91 and 79", pg.bands[0].peakMean, pg.bands[1].peakMean)
	}
	if math.Abs(pg.peakSlope+2) > 1e-9 || math.Abs(pg.firstSlope-1) > 1e-9 {
		t.Fatalf("age slopes peak %v first %v, want -2 and +1", pg.peakSlope, pg.firstSlope)
	}
	// Younger means more room: same band, age 19 vs 21.
	b := pg.bands[0]
	if pg.peakFor(b, 19, 0) <= pg.peakFor(b, 21, 0) {
		t.Fatal("a younger player should have a higher expected peak")
	}
	if pedBandOf(edges, 1) != 0 || pedBandOf(edges, 3) != 1 || pedBandOf(edges, 61) != 2 {
		t.Fatal("pick bands wrong")
	}
	// Both residuals are exactly the age term: perfectly (anti)correlated at
	// rho clamp, and the median of a band's first-season overalls is exact.
	if pg.bands[0].firstMedian != pg.bands[0].firstMean {
		t.Fatalf("median %v mean %v", pg.bands[0].firstMedian, pg.bands[0].firstMean)
	}
}

func TestCeilingNeverBelowShownPeakOrOverall(t *testing.T) {
	for _, w := range []float64{0, 0.3, 0.9} {
		for _, shown := range []int{60, 80} {
			for _, ovr := range []int{50, 70} {
				if ovr > shown {
					continue
				}
				// A low hindsight ceiling and a low pedigree draw.
				hind, ped := float64(shown-9), float64(shown-15)
				got := blendCeiling(w, hind, ped, shown, ovr)
				if got < ovr {
					t.Fatalf("w %.1f shown %d ovr %d: ceiling %d below overall", w, shown, ovr, got)
				}
				// The pedigree part may not drop below the shown peak.
				if min := round(w*hind + (1-w)*float64(shown)); got < min {
					t.Fatalf("w %.1f shown %d: ceiling %d below %d", w, shown, got, min)
				}
				if w == 0 && got < shown {
					t.Fatalf("pure pedigree ceiling %d below shown peak %d", got, shown)
				}
			}
		}
	}
	// Fully trusted: exactly the (blurred) hindsight peak, but not below the overall.
	if got := blendCeiling(1, 70, 99, 72, 60); got != 70 {
		t.Fatalf("w=1 ceiling %d, want 70", got)
	}
	if got := blendCeiling(1, 55, 99, 72, 60); got != 60 {
		t.Fatalf("w=1 ceiling %d, want clamped to overall 60", got)
	}
	if got := blendCeiling(0, 70, 120, 60, 60); got != 99 {
		t.Fatalf("ceiling %d, want clamped to 99", got)
	}
}

func pedFixtureData() *realData {
	pg := fitPedigree(pedFixtureRows(), RealTuning.PedigreeBands[:])
	return &realData{
		ped:    pg,
		pickBy: map[string]draftPick{"a": {2020, 1, "a", "A"}, "b": {2020, 40, "b", "B"}},
		career: map[string]careerInfo{"a": {birthYear: 2001}, "b": {birthYear: 2001}},
	}
}

func TestPedigreeCeilingDeterministicAndVaries(t *testing.T) {
	d := pedFixtureData()
	if d.pedigreePeak(5, "a", 20) != d.pedigreePeak(5, "a", 20) {
		t.Fatal("same seed and player must give the same draw")
	}
	varies := false
	for seed := uint64(1); seed <= 8; seed++ {
		if d.pedigreePeak(seed, "a", 20) != d.pedigreePeak(1, "a", 20) {
			varies = true
		}
	}
	if !varies {
		t.Fatal("different league seeds should give different ceilings")
	}
	// The draw does not depend on what else was asked first.
	x := d.pedigreePeak(3, "b", 20)
	d.pedigreePeak(3, "a", 20)
	if d.pedigreePeak(3, "b", 20) != x {
		t.Fatal("draw depends on call order")
	}
	if d.pedigreePeak(1, "a", 20) > RealTuning.PedigreeMax {
		t.Fatal("draw above PedigreeMax")
	}
	// Undrafted (unknown id) falls in the last band: lower than a top pick on average.
	var top, late float64
	for seed := uint64(1); seed <= 50; seed++ {
		top += d.pedigreePeak(seed, "a", 20)
		late += d.pedigreePeak(seed, "nobody", 20)
	}
	if top <= late {
		t.Fatalf("top pick mean %.1f not above undrafted mean %.1f", top/50, late/50)
	}
}

func TestShiftToOverall(t *testing.T) {
	rt := Ratings{50, 50, 50, 50, 50, 50, 50}
	got := shiftToOverall(rt, PG, nil, 70)
	if o := overallOfRatings(got, PG, nil); o < 69 || o > 71 {
		t.Fatalf("overall %d, want about 70", o)
	}
}

func TestRealTopPicksGetHighCeilings(t *testing.T) {
	if _, err := os.Stat(realDir); err != nil {
		t.Skip("real data not present")
	}
	for seed := uint64(1); seed <= 3; seed++ {
		lg, err := NewRealLeagueSeason(realDir, seed, 2026)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, p := range lg.allPlayers() {
			if p.DraftYear == 2025 && p.DraftPick >= 1 && p.DraftPick <= 3 {
				n++
				if p.Potential < 82 {
					t.Errorf("seed %d: 2025 pick %d %s has ceiling %d, want 82 or more", seed, p.DraftPick, p.Name, p.Potential)
				}
			}
		}
		if n != 3 {
			t.Fatalf("seed %d: found %d of the 2025 top-3 picks", seed, n)
		}
	}
}
