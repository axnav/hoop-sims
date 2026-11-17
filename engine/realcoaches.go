package engine

import (
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Real head coaches. The optional private file coaches.csv (season,
// abbreviation, coach, birth_year) names each team's head coach per season.
// When it is present, a real-data league keeps the generated Coach object
// (Development stays the generated draw) and overwrites its name, age,
// Offense, Defense and favourite system. Ratings come from the results of
// every team the coach led, over all seasons in the file (hindsight, like the
// players), so a coach is the same in every start season. No randomness here.
// Without the file, or for a team with no row, nothing changes.

type coachRow struct {
	season int
	abbr   string
	name   string
	birth  int
}

type teamStat struct{ o, d, pace, three float64 }

// coachProfile is a coach's shrunk average z-scores (higher is better for
// off and def; pace and three are plain style z-scores).
type coachProfile struct {
	off, def, pace, three float64
	n                     int
}

// coachRatingFromZ maps a z-score onto the generated coaches' rating scale.
func coachRatingFromZ(z float64) int {
	t := &CoachTuning
	v := round(t.RatingMean + t.RatingSD*RealTuning.RealCoachSpread*z)
	return clamp(v, t.RatingMin, t.RatingMax)
}

// coachSystemFor is the favourite-system rule (thresholds in RealTuning).
func coachSystemFor(p coachProfile) string {
	t := &RealTuning
	switch {
	case p.def-p.off >= t.CoachDefLead && p.def > t.CoachDefMin:
		return "defense_first"
	case p.three >= t.CoachThreeHigh && p.pace >= t.CoachPaceHigh && p.def < 0:
		return "run_gun"
	case p.three >= t.CoachThreeHigh:
		return "pace_space"
	case p.pace <= t.CoachPaceLow && p.three <= t.CoachThreeLow:
		return "inside_out"
	case p.off >= t.CoachStarOff:
		return "star_led"
	}
	return balancedKey
}

// shrinkProfile averages the z rows and shrinks by n/(n+RealCoachShrink).
func shrinkProfile(zs []teamStat) coachProfile {
	n := len(zs)
	if n == 0 {
		return coachProfile{}
	}
	var p coachProfile
	for _, z := range zs {
		p.off += z.o
		p.def += z.d
		p.pace += z.pace
		p.three += z.three
	}
	f := float64(n) / (float64(n) + RealTuning.RealCoachShrink)
	p.off *= f / float64(n)
	p.def *= f / float64(n)
	p.pace *= f / float64(n)
	p.three *= f / float64(n)
	p.n = n
	return p
}

// seasonZ turns each team's stats into z-scores against the league of that
// season (population standard deviation over the team rows). Defence is
// negated so higher is better.
func seasonZ(stats map[string]teamStat) map[string]teamStat {
	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var mean, sq teamStat
	for _, k := range keys {
		s := stats[k]
		mean.o += s.o
		mean.d += s.d
		mean.pace += s.pace
		mean.three += s.three
	}
	n := float64(len(keys))
	if n == 0 {
		return nil
	}
	mean = teamStat{mean.o / n, mean.d / n, mean.pace / n, mean.three / n}
	for _, k := range keys {
		s := stats[k]
		sq.o += (s.o - mean.o) * (s.o - mean.o)
		sq.d += (s.d - mean.d) * (s.d - mean.d)
		sq.pace += (s.pace - mean.pace) * (s.pace - mean.pace)
		sq.three += (s.three - mean.three) * (s.three - mean.three)
	}
	sd := func(v float64) float64 {
		v = math.Sqrt(v / n)
		if v < 1e-9 {
			return 1
		}
		return v
	}
	sdv := teamStat{sd(sq.o), sd(sq.d), sd(sq.pace), sd(sq.three)}
	out := make(map[string]teamStat, len(keys))
	for _, k := range keys {
		s := stats[k]
		out[k] = teamStat{
			o:     (s.o - mean.o) / sdv.o,
			d:     -(s.d - mean.d) / sdv.d,
			pace:  (s.pace - mean.pace) / sdv.pace,
			three: (s.three - mean.three) / sdv.three,
		}
	}
	return out
}

// loadCoachRows reads coaches.csv (nil when missing or unreadable).
func loadCoachRows(dataDir string) []coachRow {
	recs, err := readCSV(filepath.Join(dataDir, "coaches.csv"))
	if err != nil {
		return nil
	}
	var rows []coachRow
	for _, m := range recs {
		low := make(map[string]string, len(m))
		for k, v := range m {
			low[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		}
		season, err := strconv.Atoi(low["season"])
		if err != nil || low["abbreviation"] == "" || low["coach"] == "" {
			continue
		}
		birth, _ := strconv.Atoi(low["birth_year"])
		rows = append(rows, coachRow{season, low["abbreviation"], low["coach"], birth})
	}
	return rows
}

// loadSeasonZ reads Team Summaries.csv into z-scores per season and team
// abbreviation (the league-average row has abbreviation NA and is skipped).
func loadSeasonZ(dataDir string) map[int]map[string]teamStat {
	recs, err := readCSV(filepath.Join(dataDir, "Team Summaries.csv"))
	if err != nil {
		return nil
	}
	raw := map[int]map[string]teamStat{}
	for _, m := range recs {
		ab := m["abbreviation"]
		if ab == "" || ab == "NA" {
			continue
		}
		season, err := strconv.Atoi(m["season"])
		if err != nil || m["o_rtg"] == "" || m["d_rtg"] == "" {
			continue
		}
		if raw[season] == nil {
			raw[season] = map[string]teamStat{}
		}
		raw[season][ab] = teamStat{num(m["o_rtg"]), num(m["d_rtg"]), num(m["pace"]), num(m["x3p_ar"])}
	}
	out := map[int]map[string]teamStat{}
	for s, st := range raw {
		out[s] = seasonZ(st)
	}
	return out
}

// realCoachProfiles builds each coach's profile (by name) over his whole record.
func realCoachProfiles(rows []coachRow, z map[int]map[string]teamStat) map[string]coachProfile {
	by := map[string][]teamStat{}
	for _, r := range rows {
		if s, ok := z[r.season][r.abbr]; ok {
			by[r.name] = append(by[r.name], s)
		}
	}
	out := make(map[string]coachProfile, len(by))
	for name, zs := range by {
		out[name] = shrinkProfile(zs)
	}
	return out
}

// applyRealCoaches overwrites the generated coaches of a real league with the
// real head coaches of the season, when coaches.csv is present.
func applyRealCoaches(lg *League, dataDir string, season int) {
	rows := loadCoachRows(dataDir)
	if len(rows) == 0 {
		return
	}
	profiles := realCoachProfiles(rows, loadSeasonZ(dataDir))
	abbrs := RealTeamAbbrevsFor(season)
	for _, t := range lg.Teams {
		if t.Coach == nil || t.ID < 0 || t.ID >= len(abbrs) {
			continue
		}
		for _, r := range rows {
			if r.season != season || r.abbr != abbrs[t.ID] {
				continue
			}
			c := t.Coach
			c.Name = r.name
			if r.birth > 0 {
				c.Age = clamp(season-r.birth, 28, 85)
			}
			if p, ok := profiles[r.name]; ok && p.n > 0 {
				c.Offense = coachRatingFromZ(p.off)
				c.Defense = coachRatingFromZ(p.def)
				c.System = coachSystemFor(p)
			}
			break
		}
	}
	lg.cmeans = nil
}
