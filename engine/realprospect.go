package engine

import (
	"hash/fnv"
	"math"
	"sort"
)

// Scouting-style ceilings for real prospects and young real players
// (HANDOFF section 23, part C). From the real draft classes with at least
// PedigreeMinSeasons seasons after the draft, the distribution of what each
// pick band became (peak overall) and how good it was in its first season is
// measured with the part A ratings. A player's ceiling is then a draw from
// his band, blended with what the data already shows of him.

// pedRow is one drafted player of the pedigree sample.
type pedRow struct {
	pick, age   int     // overall pick, age at the draft
	peak, first float64 // best overall afterwards, overall in his first season
}

// pedBand is the sample statistics of one pick band.
type pedBand struct {
	n                  int
	age                float64 // mean age at the draft
	peakMean, peakSD   float64
	firstMean, firstSD float64
	firstMedian        float64
}

// pedigree is the fitted table: one band per RealTuning.PedigreeBands edge,
// a linear age term (points per year of age at the draft, from the band-mean
// residuals) for each of the two measures, and the correlation of the two
// residuals (a better rookie tends to peak higher).
type pedigree struct {
	bands                 []pedBand
	peakSlope, firstSlope float64
	rho                   float64
}

// pedBandOf is the index of the band holding a pick (picks above the last
// edge, and undrafted players, fall into the last band).
func pedBandOf(edges []int, pick int) int {
	for i, e := range edges {
		if pick <= e {
			return i
		}
	}
	return len(edges) - 1
}

func fitPedigree(rows []pedRow, edges []int) *pedigree {
	pg := &pedigree{bands: make([]pedBand, len(edges))}
	by := make([][]pedRow, len(edges))
	for _, r := range rows {
		b := pedBandOf(edges, r.pick)
		by[b] = append(by[b], r)
	}
	for i, rs := range by {
		if len(rs) == 0 {
			continue
		}
		b := &pg.bands[i]
		b.n = len(rs)
		var peaks, firsts []float64
		for _, r := range rs {
			b.age += float64(r.age) / float64(len(rs))
			peaks = append(peaks, r.peak)
			firsts = append(firsts, r.first)
		}
		b.peakMean, b.peakSD = meanSD(peaks)
		b.firstMean, b.firstSD = meanSD(firsts)
		sort.Float64s(firsts)
		b.firstMedian = firsts[len(firsts)/2]
		if len(firsts)%2 == 0 {
			b.firstMedian = (firsts[len(firsts)/2-1] + firsts[len(firsts)/2]) / 2
		}
	}
	// Age slopes and residual correlation, pooled over bands.
	var sxx, sxp, sxf, spp, sff, spf float64
	for i, rs := range by {
		b := pg.bands[i]
		for _, r := range rs {
			dx, dp, df := float64(r.age)-b.age, r.peak-b.peakMean, r.first-b.firstMean
			sxx += dx * dx
			sxp += dx * dp
			sxf += dx * df
		}
	}
	if sxx > 0 {
		pg.peakSlope, pg.firstSlope = sxp/sxx, sxf/sxx
	}
	for i, rs := range by {
		b := pg.bands[i]
		for _, r := range rs {
			dp := r.peak - b.peakMean - pg.peakSlope*(float64(r.age)-b.age)
			df := r.first - b.firstMean - pg.firstSlope*(float64(r.age)-b.age)
			spp += dp * dp
			sff += df * df
			spf += dp * df
		}
	}
	if spp > 0 && sff > 0 {
		pg.rho = clampF(spf/math.Sqrt(spp*sff), -0.95, 0.95)
	}
	return pg
}

// band returns the fitted band for a pick; a band with no sample borrows the
// nearest band that has one.
func (pg *pedigree) band(edges []int, pick int) pedBand {
	i := pedBandOf(edges, pick)
	for d := 0; d < len(pg.bands); d++ {
		for _, j := range []int{i - d, i + d} {
			if j >= 0 && j < len(pg.bands) && pg.bands[j].n > 0 {
				return pg.bands[j]
			}
		}
	}
	return pedBand{}
}

// peakFor and firstFor are the band's expected values at a draft age plus z
// standard deviations of the band's spread.
func (pg *pedigree) peakFor(b pedBand, age int, z float64) float64 {
	return b.peakMean + pg.peakSlope*(float64(age)-b.age) + RealTuning.PedigreeSpread*b.peakSD*z
}

func (pg *pedigree) firstFor(b pedBand, age int, z float64) float64 {
	return b.firstMean + pg.firstSlope*(float64(age)-b.age) + b.firstSD*z
}

func (pg *pedigree) firstMedianFor(b pedBand, age int) float64 {
	return b.firstMedian + pg.firstSlope*(float64(age)-b.age)
}

// pedigreeOf measures the table from the data once per loaded dataset.
func (d *realData) pedigreeOf() *pedigree {
	if d.ped != nil {
		return d.ped
	}
	c := &RealTuning
	var picks []draftPick
	for _, pk := range d.pickBy {
		if pk.year >= c.PedigreeFirstDraft && pk.year <= c.PedigreeLastDraft && pk.pick >= 1 {
			picks = append(picks, pk)
		}
	}
	sort.Slice(picks, func(i, j int) bool {
		if picks[i].year != picks[j].year {
			return picks[i].year < picks[j].year
		}
		return picks[i].pick < picks[j].pick
	})
	var rows []pedRow
	for _, pk := range picks {
		first := 0
		for s := pk.year + 1; s <= lastDataSeason; s++ {
			if x := d.all[s][pk.pid]; x != nil && x.mp > 0 {
				first = s
				break
			}
		}
		if first == 0 {
			continue // never played: no rating to measure
		}
		fr := d.rate(d.all[first][pk.pid], c.RookieDrop)
		peak, anyPeak := -1, 0
		for s := first; s <= lastDataSeason; s++ {
			x := d.all[s][pk.pid]
			if x == nil || x.mp <= 0 {
				continue
			}
			rp := d.ratedFor(s, pk.pid)
			anyPeak = max(anyPeak, rp.Overall)
			if x.mp >= c.MinQual {
				peak = max(peak, rp.Overall)
			}
		}
		if peak < 0 {
			peak = anyPeak
		}
		rows = append(rows, pedRow{pick: pk.pick, age: d.draftAge(pk.pid, pk.year), peak: float64(peak), first: float64(fr.Overall)})
	}
	d.ped = fitPedigree(rows, c.PedigreeBands[:])
	return d.ped
}

// draftAge is a drafted player's age in his draft year (20 if the birth year
// is unknown), limited to the ages the prospects use.
func (d *realData) draftAge(pid string, year int) int {
	if by := d.career[pid].birthYear; by > 0 {
		return clamp(year-by, 18, 25)
	}
	return 20
}

// pedigreeBandFor is the pick and draft age used for a player: his real draft
// slot, or the last band at the age of a typical undrafted rookie.
func (d *realData) pedigreeSlot(pid string, age int) (pick, draftAge int) {
	if pk, ok := d.pickBy[pid]; ok && pk.pick >= 1 {
		return pk.pick, d.draftAge(pid, pk.year)
	}
	return 61, clamp(age, 20, 24)
}

// pedStream gives the player's two standard normal draws (first-season and
// peak), correlated by the fitted rho. They depend only on league seed and
// player, never on build order. Own stream: salt 150000 (RULES.md section 3).
func pedStream(seed uint64, pid string, rho float64) (zFirst, zPeak float64) {
	h := fnv.New64a()
	h.Write([]byte(pid))
	r := NewRNG(mix64(seed, 150000, h.Sum64()))
	a, b := r.NormFloat64(), r.NormFloat64()
	return a, rho*a + math.Sqrt(1-rho*rho)*b
}

// pedigreePeak is the pedigree draw of a player's peak overall plus the
// scouting optimism (RealTuning.PedigreeBoost standard deviations of his band:
// a ceiling is what he could reach, above what a typical player of the band
// becomes).
func (d *realData) pedigreePeak(seed uint64, pid string, age int) float64 {
	pg := d.pedigreeOf()
	pick, da := d.pedigreeSlot(pid, age)
	b := pg.band(RealTuning.PedigreeBands[:], pick)
	_, zp := pedStream(seed, pid, pg.rho)
	return math.Min(RealTuning.PedigreeMax, pg.peakFor(b, da, zp)+RealTuning.PedigreeBoost*b.peakSD)
}

// pedigreeFirst is the pedigree draw of a prospect's first-season overall
// (used for a prospect with no real season).
func (d *realData) pedigreeFirst(seed uint64, pk draftPick, year int) float64 {
	pg := d.pedigreeOf()
	b := pg.band(RealTuning.PedigreeBands[:], pk.pick)
	zf, _ := pedStream(seed, pk.pid, pg.rho)
	return pg.firstFor(b, d.draftAge(pk.pid, year), zf)
}

// pedigreeFirstMedian is the band's median first-season overall at the age.
func (d *realData) pedigreeFirstMedian(pk draftPick, year int) float64 {
	pg := d.pedigreeOf()
	return pg.firstMedianFor(pg.band(RealTuning.PedigreeBands[:], pk.pick), d.draftAge(pk.pid, year))
}

// ceilingFor is the stored ceiling of a young player (age below
// PotentialEqualAge). It blends the trusted part of what the data shows (the
// blurred hindsight peak from season `from` on, weight = seasons seen /
// seasons until PotentialEqualAge) with a pedigree draw from his draft band
// (the rest, never below a peak the data already shows). The result is never
// below his overall.
func (d *realData) ceilingFor(seed uint64, pid string, from, age, ovr int) int {
	shown := d.hindsightPeak(pid, from, ovr)
	hind := blurPotential(seed, pid, shown)
	w := ceilingWeight(max(d.start, from), age)
	ped := d.pedigreeCeiling(seed, pid, age)
	return blendCeiling(w, float64(hind), ped, shown, ovr)
}

func (d *realData) pedigreeCeiling(seed uint64, pid string, age int) float64 {
	return d.pedigreePeak(seed, pid, age)
}

// blendCeiling mixes the trusted hindsight ceiling (weight w) with the
// pedigree ceiling. The pedigree part never goes below the peak the data
// already shows, and the result never below the player's overall. (A fully
// trusted ceiling, w = 1, is the blurred hindsight peak itself.)
func blendCeiling(w, hind, ped float64, shown, ovr int) int {
	v := round(w*hind + (1-w)*math.Max(ped, float64(shown)))
	return clamp(v, ovr, 99)
}

// shiftToOverall moves every rating by the same amount until the overall
// matches want (within one point), keeping the player's shape.
func shiftToOverall(rt Ratings, pos Position, alt []Position, want int) Ratings {
	for it := 0; it < 10; it++ {
		diff := want - overallOfRatings(rt, pos, alt)
		if diff == 0 {
			break
		}
		a := rt.array()
		for i := range a {
			a[i] = clamp(a[i]+diff, 25, 99)
		}
		rt = Ratings{a[0], a[1], a[2], a[3], a[4], a[5], a[6]}
	}
	return rt
}
