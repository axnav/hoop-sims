package engine

import (
	"encoding/csv"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// RealTuning holds every tunable number of the stats -> ratings formula.
type RealTuningT struct {
	MinQual     float64 // minutes needed to count in the percentile pools
	ShrinkPower float64 // low-minute players regress to Replacement by (mp/MinQual)^power
	Replacement float64

	// Target overall curve: Mean + Sd*z, compressed above CompressZ.
	Mean, Sd     float64
	CompressZ    float64
	CompressSlop float64

	// Weight of the box-score profile in the visible overall (rest is the
	// BPM/WS value estimate); ImpactK scales the gap into hidden Impact.
	BoxWeight float64
	ImpactK   float64
	ImpactMax int

	// Composite value estimate weights (z-scores).
	ValBPM, ValWS48, ValPER float64

	// Volume part of the value score (realstanding.go work): VolWeight is its
	// share of the blend with the per-minute part; inside it VORP, total win
	// shares and minutes a game are z-scores weighted VolVORP, VolWS, VolMPG.
	VolWeight, VolVORP, VolWS, VolMPG float64
	// The low-minute shrink also uses minutes a game and games played: full
	// weight from ShrinkMPG minutes a game and ShrinkGames games, below that
	// (x/full)^power on top of the season-minutes shrink.
	ShrinkMPG, ShrinkMPGPower, ShrinkGames, ShrinkGamesPower float64
	// BoxGroups: the box-score percentile is taken within guards (PG, SG),
	// wings (SF) and bigs (PF, C) instead of the whole league, so rebounds and
	// blocks do not lift big men over guards.
	BoxGroups bool

	UsageInside, UsageMid, UsageThree, UsagePlay float64
	RookieDrop                                   float64

	// Pedigree (realprospect.go): draft classes PedigreeFirstDraft to
	// PedigreeLastDraft (each has six or more real seasons after it) give, per
	// pick band (upper pick of each band), the distribution of the peak and of
	// the first-season overall. PedigreeSpread scales the band's spread of
	// the peak draw; PedigreeBoost is the scouting optimism added to the
	// draw, in standard deviations of the band (a ceiling is above the typical outcome). RookieBlendMP: a first
	// real season with fewer minutes is blended toward the band's median
	// first-season overall by minutes / RookieBlendMP.
	PedigreeBands                         [7]int
	PedigreeFirstDraft, PedigreeLastDraft int
	PedigreeSpread, PedigreeBoost         float64
	PedigreeMax                           float64 // a pedigree draw never exceeds this
	RookieBlendMP                         float64

	// PotentialBlurSD is the standard deviation (rating points) of a normal
	// draw added to a young real player's or prospect's hindsight ceiling, so
	// the same real players do not become stars in every league. The draw is
	// per player and per league seed (own stream, independent of everything
	// else). 0 gives the exact hindsight peaks.
	PotentialBlurSD float64

	// Real head coaches (realcoaches.go). A coach's Offense and Defense come
	// from his teams' o_rtg and d_rtg in league standard deviations, averaged
	// over every season he coached in coaches.csv and shrunk by
	// n/(n+RealCoachShrink) (players explain most of a team's rating).
	// RealCoachSpread scales the result onto CoachTuning.RatingSD.
	RealCoachShrink, RealCoachSpread float64
	// Growth of real players (progression.go, TrueCeiling and
	// ProgressMentored; fictional players are untouched): the hidden Impact
	// moves the growth target by RealImpactMult x Impact (fictional: 2.0),
	// RealCeilingOffset is added to the target, and a young player's
	// few-minutes penalty is multiplied by RealMinutesPenaltyShare.
	RealImpactMult, RealCeilingOffset, RealMinutesPenaltyShare float64
	// Favourite system rule (z values after the same shrink): defence ahead of
	// offence by CoachDefLead and above CoachDefMin gives defense_first; three
	// rate at or above CoachThreeHigh is pace_space, or run_gun when pace is
	// also at or above CoachPaceHigh and defence is below average; pace at or
	// below CoachPaceLow with three rate at or below CoachThreeLow is
	// inside_out; offence at or above CoachStarOff is star_led; else balanced.
	CoachDefLead, CoachDefMin, CoachThreeHigh, CoachPaceHigh float64
	CoachPaceLow, CoachThreeLow, CoachStarOff                float64
}

var RealTuning = RealTuningT{
	MinQual: 500, ShrinkPower: 0.6, Replacement: 50,
	Mean: 63.8, Sd: 12, CompressZ: 1.5, CompressSlop: 1.35,
	BoxWeight: 0.3, ImpactK: 1.0, ImpactMax: 5,
	ValBPM: 0.6, ValWS48: 0.3, ValPER: 0.1,
	VolWeight: 0.4, VolVORP: 0.4, VolWS: 0.3, VolMPG: 0.3,
	ShrinkMPG: 28, ShrinkMPGPower: 1.0, ShrinkGames: 50, ShrinkGamesPower: 0.5,
	BoxGroups:   true,
	UsageInside: 30, UsageMid: 22, UsageThree: 22, UsagePlay: 8,
	RookieDrop: 4, PotentialBlurSD: 4.0,
	PedigreeBands:      [7]int{1, 3, 5, 10, 14, 30, 60},
	PedigreeFirstDraft: 2011, PedigreeLastDraft: 2019,
	PedigreeSpread: 0.5, PedigreeBoost: 1.0, PedigreeMax: 97, RookieBlendMP: 1200,
	RealCoachShrink: 3, RealCoachSpread: 1.0,
	RealImpactMult: 0.25, RealCeilingOffset: 1, RealMinutesPenaltyShare: 0.25,
	CoachDefLead: 0.2, CoachDefMin: 0, CoachThreeHigh: 0.2, CoachPaceHigh: 0.1,
	CoachPaceLow: -0.1, CoachThreeLow: -0.1, CoachStarOff: 0.25,
}

// ---------------------------------------------------------------- loading

type statRow struct {
	season                                     int
	pid, name, team, pos                       string
	age                                        float64
	g, gs, mp                                  float64
	fg, fga, x3p, x3pa, x2p, x2pa, ft, fta     float64
	orb, drb, trb, ast, stl, blk, tov, pf, pts float64
	per, ts, ftr, orbp, drbp, trbp, astp, stlp float64
	blkp, tovp, usg, ows, dws, ws, ws48        float64
	obpm, dbpm, bpm, vorp                      float64
	combined                                   bool
}

type careerInfo struct {
	pos       string
	ht        float64
	birthYear int
	from      int
}

type draftPick struct {
	year, pick int
	pid, name  string
}

type realData struct {
	start  int                           // first season of the league being built (draft classes before it are not stored)
	all    map[int]map[string]*statRow   // season -> pid -> combined (or only) row
	teams  map[int]map[string][]*statRow // season -> pid -> per-team rows
	career map[string]careerInfo
	picks  []draftPick
	pickBy map[string]draftPick
	models map[int]*seasonModel
	rated  map[int]map[string]*ratedPlayer
	target map[int]map[string]float64 // standing.csv overall targets (optional)
	ped    *pedigree                  // pick-band table, measured on first use (realprospect.go)
}

func readCSV(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rd := csv.NewReader(f)
	rd.FieldsPerRecord = -1
	hdr, err := rd.Read()
	if err != nil {
		return nil, err
	}
	var out []map[string]string
	for {
		rec, err := rd.Read()
		if err != nil {
			break
		}
		m := make(map[string]string, len(hdr))
		for i, h := range hdr {
			if i < len(rec) {
				m[h] = rec[i]
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func num(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func isCombinedTeam(t string) bool {
	return t == "TOT" || (len(t) == 3 && t[1:] == "TM")
}

// Real seasons are named by their ending year (2011 = 2010-11). A league can
// start in any season from firstRealSeason to lastRealSeason; lastDataSeason is
// the last season that has any rows in the CSVs (a few games of 2026-27, used
// only as extra evidence for ceilings).
const (
	firstRealSeason = 2011
	lastRealSeason  = 2026
	lastDataSeason  = 2027
)

func loadRealData(dir string) (*realData, error) { return loadRealDataFrom(dir, firstRealSeason) }

// loadRealDataFrom loads the CSVs; stat rows from season `start` on, draft
// classes of year `start` on (older picks stay reachable through pickBy).
func loadRealDataFrom(dir string, start int) (*realData, error) {
	tot, err := readCSV(filepath.Join(dir, "Player Totals.csv"))
	if err != nil {
		return nil, err
	}
	adv, err := readCSV(filepath.Join(dir, "Advanced.csv"))
	if err != nil {
		return nil, err
	}
	type key struct {
		season    int
		pid, team string
	}
	advBy := map[key]map[string]string{}
	for _, m := range adv {
		if m["lg"] != "NBA" {
			continue
		}
		advBy[key{int(num(m["season"])), m["player_id"], m["team"]}] = m
	}
	d := &realData{
		start: start, target: loadStandingTargets(dir),
		all: map[int]map[string]*statRow{}, teams: map[int]map[string][]*statRow{},
		career: map[string]careerInfo{}, pickBy: map[string]draftPick{},
		models: map[int]*seasonModel{}, rated: map[int]map[string]*ratedPlayer{},
	}
	for _, m := range tot {
		if m["lg"] != "NBA" {
			continue
		}
		season := int(num(m["season"]))
		if season < firstRealSeason {
			continue
		}
		a := advBy[key{season, m["player_id"], m["team"]}]
		if a == nil {
			a = map[string]string{}
		}
		r := &statRow{
			season: season, pid: m["player_id"], name: m["player"], team: m["team"], pos: m["pos"],
			age: num(m["age"]), g: num(m["g"]), gs: num(m["gs"]), mp: num(m["mp"]),
			fg: num(m["fg"]), fga: num(m["fga"]), x3p: num(m["x3p"]), x3pa: num(m["x3pa"]),
			x2p: num(m["x2p"]), x2pa: num(m["x2pa"]), ft: num(m["ft"]), fta: num(m["fta"]),
			orb: num(m["orb"]), drb: num(m["drb"]), trb: num(m["trb"]), ast: num(m["ast"]),
			stl: num(m["stl"]), blk: num(m["blk"]), tov: num(m["tov"]), pf: num(m["pf"]), pts: num(m["pts"]),
			per: num(a["per"]), ts: num(a["ts_percent"]), ftr: num(a["f_tr"]),
			orbp: num(a["orb_percent"]), drbp: num(a["drb_percent"]), trbp: num(a["trb_percent"]),
			astp: num(a["ast_percent"]), stlp: num(a["stl_percent"]), blkp: num(a["blk_percent"]),
			tovp: num(a["tov_percent"]), usg: num(a["usg_percent"]),
			ows: num(a["ows"]), dws: num(a["dws"]), ws: num(a["ws"]), ws48: num(a["ws_48"]),
			obpm: num(a["obpm"]), dbpm: num(a["dbpm"]), bpm: num(a["bpm"]), vorp: num(a["vorp"]),
			combined: isCombinedTeam(m["team"]),
		}
		if d.all[season] == nil {
			d.all[season] = map[string]*statRow{}
			d.teams[season] = map[string][]*statRow{}
		}
		if r.combined {
			d.all[season][r.pid] = r
		} else {
			d.teams[season][r.pid] = append(d.teams[season][r.pid], r)
			if _, ok := d.all[season][r.pid]; !ok {
				d.all[season][r.pid] = r
			}
		}
	}
	ci, err := readCSV(filepath.Join(dir, "Player Career Info.csv"))
	if err != nil {
		return nil, err
	}
	for _, m := range ci {
		by := 0
		if len(m["birth_date"]) >= 4 {
			by = int(num(m["birth_date"][:4]))
		}
		d.career[m["player_id"]] = careerInfo{m["pos"], num(m["ht_in_in"]), by, int(num(m["from"]))}
	}
	dr, err := readCSV(filepath.Join(dir, "Draft Pick History.csv"))
	if err != nil {
		return nil, err
	}
	for _, m := range dr {
		if m["lg"] != "NBA" {
			continue
		}
		y := int(num(m["season"]))
		if y < d.start || m["player_id"] == "" || m["player_id"] == "NA" {
			continue
		}
		pk := draftPick{y, int(num(m["overall_pick"])), m["player_id"], m["player"]}
		d.picks = append(d.picks, pk)
		d.pickBy[pk.pid] = pk
	}
	// A player's draft year may predate firstRealSeason; those are not needed
	// for pickBy except by id, so also read older picks for current players.
	for _, m := range dr {
		if m["lg"] != "NBA" || m["player_id"] == "" || m["player_id"] == "NA" {
			continue
		}
		if _, ok := d.pickBy[m["player_id"]]; !ok {
			d.pickBy[m["player_id"]] = draftPick{int(num(m["season"])), int(num(m["overall_pick"])), m["player_id"], m["player"]}
		}
	}
	return d, nil
}

// ---------------------------------------------------------------- model

type dist []float64

func newDist(v []float64) dist {
	sort.Float64s(v)
	return dist(v)
}

// pct is the midpoint percentile of v in the pool, in (0,1).
func (d dist) pct(v float64) float64 {
	if len(d) == 0 {
		return 0.5
	}
	lo := sort.SearchFloat64s(d, v)
	hi := lo
	for hi < len(d) && d[hi] == v {
		hi++
	}
	return (float64(lo+hi) / 2) / float64(len(d))
}

type seasonModel struct {
	season                                          int
	lg3, lgFT, lg2                                  float64
	reg3, ft, reg2, a3, a2, ftr, orb, ast, tov, trb dist
	dbpm, stl, blk, dws48, usg                      dist
	zBox, zVal                                      dist
	zBoxG                                           [3]dist // by position group, see boxGroup
	pmM, pmS, vlM, vlS                              float64 // per-minute and volume parts of the value score
	vorpM, vorpS, wsTM, wsTS, mpgM, mpgS            float64
	meanBox, sdBox                                  float64
	bpmM, bpmS, wsM, wsS, perM, perS                float64
}

func meanSD(v []float64) (float64, float64) {
	if len(v) == 0 {
		return 0, 1
	}
	m := 0.0
	for _, x := range v {
		m += x
	}
	m /= float64(len(v))
	s := 0.0
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	s = math.Sqrt(s / float64(len(v)))
	if s == 0 {
		s = 1
	}
	return m, s
}

func (r *statRow) per36(x float64) float64 {
	if r.mp <= 0 {
		return 0
	}
	return x / r.mp * 36
}

func (r *statRow) ftRate() float64 {
	if r.fga <= 0 {
		return 0
	}
	return r.fta / r.fga
}

func (r *statRow) dws48() float64 {
	if r.mp <= 0 {
		return 0
	}
	return r.dws / r.mp * 48
}

const (
	k3 = 100.0
	kF = 80.0
	k2 = 200.0
)

func (m *seasonModel) in3(r *statRow) float64  { return (r.x3p + k3*m.lg3) / (r.x3pa + k3) }
func (m *seasonModel) inFT(r *statRow) float64 { return (r.ft + kF*m.lgFT) / (r.fta + kF) }
func (m *seasonModel) in2(r *statRow) float64  { return (r.x2p + k2*m.lg2) / (r.x2pa + k2) }

func (d *realData) model(season int) *seasonModel {
	if m := d.models[season]; m != nil {
		return m
	}
	c := &RealTuning
	m := &seasonModel{season: season}
	var qual []*statRow
	for _, r := range d.all[season] {
		if r.mp >= c.MinQual {
			qual = append(qual, r)
		}
	}
	sort.Slice(qual, func(i, j int) bool { return qual[i].pid < qual[j].pid })
	var t3, t3a, tf, tfa, t2, t2a float64
	for _, r := range qual {
		t3 += r.x3p
		t3a += r.x3pa
		tf += r.ft
		tfa += r.fta
		t2 += r.x2p
		t2a += r.x2pa
	}
	m.lg3, m.lgFT, m.lg2 = t3/math.Max(t3a, 1), tf/math.Max(tfa, 1), t2/math.Max(t2a, 1)
	col := func(f func(*statRow) float64) dist {
		v := make([]float64, len(qual))
		for i, r := range qual {
			v[i] = f(r)
		}
		return newDist(v)
	}
	m.reg3 = col(m.in3)
	m.ft = col(m.inFT)
	m.reg2 = col(m.in2)
	m.a3 = col(func(r *statRow) float64 { return r.per36(r.x3pa) })
	m.a2 = col(func(r *statRow) float64 { return r.per36(r.x2pa) })
	m.ftr = col((*statRow).ftRate)
	m.orb = col(func(r *statRow) float64 { return r.orbp })
	m.ast = col(func(r *statRow) float64 { return r.astp })
	m.tov = col(func(r *statRow) float64 { return r.tovp })
	m.trb = col(func(r *statRow) float64 { return r.trbp })
	m.dbpm = col(func(r *statRow) float64 { return r.dbpm })
	m.stl = col(func(r *statRow) float64 { return r.stlp })
	m.blk = col(func(r *statRow) float64 { return r.blkp })
	m.dws48 = col((*statRow).dws48)
	m.usg = col(func(r *statRow) float64 { return r.usg })

	getv := func(f func(*statRow) float64) (float64, float64) {
		v := make([]float64, len(qual))
		for i, r := range qual {
			v[i] = f(r)
		}
		return meanSD(v)
	}
	m.bpmM, m.bpmS = getv(func(r *statRow) float64 { return r.bpm })
	m.wsM, m.wsS = getv(func(r *statRow) float64 { return r.ws48 })
	m.perM, m.perS = getv(func(r *statRow) float64 { return r.per })
	d.models[season] = m // set early: boxRaw below needs the distributions

	box := make([]float64, len(qual))
	for i, r := range qual {
		box[i] = d.boxOverall(m, r)
	}
	m.meanBox, m.sdBox = meanSD(box)
	m.zBox = newDist(append([]float64(nil), box...))
	var grp [3][]float64
	for i, r := range qual {
		pos, _ := d.position(m, r)
		g := boxGroup(pos)
		grp[g] = append(grp[g], box[i])
	}
	for g := range grp {
		m.zBoxG[g] = newDist(grp[g])
	}
	// Value score: moments of its two parts over the pool, then the blend.
	m.vorpM, m.vorpS = getv(func(r *statRow) float64 { return r.vorp })
	m.wsTM, m.wsTS = getv(func(r *statRow) float64 { return r.ws })
	m.mpgM, m.mpgS = getv(mpgOf)
	m.pmM, m.pmS = getv(m.perMinRaw)
	m.vlM, m.vlS = getv(m.volumeRaw)
	vals := make([]float64, len(qual))
	for i, r := range qual {
		vals[i] = m.valueZ(r)
	}
	m.zVal = newDist(vals)
	return m
}

func mpgOf(r *statRow) float64 {
	if r.g <= 0 {
		return 0
	}
	return r.mp / r.g
}

func (m *seasonModel) perMinRaw(r *statRow) float64 {
	c := &RealTuning
	return c.ValBPM*(r.bpm-m.bpmM)/m.bpmS + c.ValWS48*(r.ws48-m.wsM)/m.wsS + c.ValPER*(r.per-m.perM)/m.perS
}

func (m *seasonModel) volumeRaw(r *statRow) float64 {
	c := &RealTuning
	return c.VolVORP*(r.vorp-m.vorpM)/m.vorpS + c.VolWS*(r.ws-m.wsTM)/m.wsTS + c.VolMPG*(mpgOf(r)-m.mpgM)/m.mpgS
}

// valueZ blends the per-minute part with the volume part (each in its own
// pool standard deviations).
func (m *seasonModel) valueZ(r *statRow) float64 {
	c := &RealTuning
	return (1-c.VolWeight)*(m.perMinRaw(r)-m.pmM)/m.pmS + c.VolWeight*(m.volumeRaw(r)-m.vlM)/m.vlS
}

// boxGroup: 0 guards (PG, SG), 1 wings (SF), 2 bigs (PF, C).
func boxGroup(p Position) int {
	switch p {
	case PG, SG:
		return 0
	case SF:
		return 1
	}
	return 2
}

// ---------------------------------------------------------------- ratings

func pctRating(p float64) float64 { return 30 + 62*p }

func clampF(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func parsePos(s string) (Position, []Position) {
	one := func(t string) (Position, bool) {
		switch t {
		case "PG":
			return PG, true
		case "SG", "G":
			return SG, true
		case "SF", "F":
			return SF, true
		case "PF":
			return PF, true
		case "C":
			return C, true
		}
		return PG, false
	}
	var ps []Position
	for _, t := range strings.Split(s, "-") {
		if p, ok := one(strings.TrimSpace(t)); ok {
			ps = append(ps, p)
		}
	}
	if len(ps) == 0 {
		return SF, nil
	}
	return ps[0], ps[1:]
}

func containsPos(ps []Position, p Position) bool {
	for _, q := range ps {
		if q == p {
			return true
		}
	}
	return false
}

// position resolves Pos and AltPos from the season's listed position, the
// player's height and his assist/usage profile.
func (d *realData) position(m *seasonModel, r *statRow) (Position, []Position) {
	pos, alt := parsePos(r.pos)
	if len(alt) == 0 {
		ht := d.career[r.pid].ht
		if ht == 0 {
			ht = 78
		}
		astP := m.ast.pct(r.astp)
		var a Position = -1
		switch pos {
		case PG:
			if ht >= 76 || m.usg.pct(r.usg) > 0.7 {
				a = SG
			}
		case SG:
			if astP > 0.7 {
				a = PG
			} else if ht >= 79 {
				a = SF
			}
		case SF:
			if ht <= 78 {
				a = SG
			} else if ht >= 81 {
				a = PF
			}
		case PF:
			if ht >= 82 && m.trb.pct(r.trbp) > 0.6 {
				a = C
			} else if ht <= 80 {
				a = SF
			}
		case C:
			if ht <= 82 || m.reg3.pct(m.in3(r)) > 0.6 {
				a = PF
			}
		}
		if a >= PG && a != pos {
			alt = []Position{a}
		}
	}
	return pos, alt
}

// baseRatings is the raw stat-profile ratings (float, unshifted).
func (d *realData) baseRatings(m *seasonModel, r *statRow, pos Position) [7]float64 {
	c := &RealTuning
	big := pos == PF || pos == C
	u := m.usg.pct(r.usg) - 0.5

	three := 0.5*m.reg3.pct(m.in3(r)) + 0.5*m.a3.pct(r.per36(r.x3pa))
	mid := 0.75*m.ft.pct(m.inFT(r)) + 0.25*m.reg2.pct(m.in2(r))
	if big {
		mid = m.ft.pct(m.inFT(r))
	}
	inside := 0.3*m.reg2.pct(m.in2(r)) + 0.25*m.ftr.pct(r.ftRate()) + 0.25*m.a2.pct(r.per36(r.x2pa)) + 0.2*m.orb.pct(r.orbp)
	play := 0.85*m.ast.pct(r.astp) + 0.15*(1-m.tov.pct(r.tovp))
	reb := m.trb.pct(r.trbp)
	def := 0.35*m.dbpm.pct(r.dbpm) + 0.2*m.stl.pct(r.stlp) + 0.2*m.blk.pct(r.blkp) + 0.25*m.dws48.pct(r.dws48())
	ath := 0.25*m.stl.pct(r.stlp) + 0.2*m.blk.pct(r.blkp) + 0.2*m.orb.pct(r.orbp) + 0.2*m.ftr.pct(r.ftRate()) + 0.15*m.a2.pct(r.per36(r.x2pa))

	var out [7]float64
	out[0] = pctRating(inside) + c.UsageInside*u
	out[1] = pctRating(mid) + c.UsageMid*u
	out[2] = pctRating(three) + c.UsageThree*u
	out[3] = pctRating(play) + c.UsagePlay*u
	out[4] = pctRating(reb)
	out[5] = pctRating(def)
	out[6] = pctRating(ath) + clampF((27-r.age)*0.9, -7, 6)
	return out
}

func ratingsOf(a [7]float64) Ratings {
	v := func(i int) int { return clamp(int(math.Floor(a[i]+0.5)), 25, 99) }
	return Ratings{v(0), v(1), v(2), v(3), v(4), v(5), v(6)}
}

func overallOfRatings(rt Ratings, pos Position, alt []Position) int {
	best := overallAt(rt, pos)
	for _, a := range alt {
		best = max(best, overallAt(rt, a))
	}
	return best
}

func (d *realData) boxOverall(m *seasonModel, r *statRow) float64 {
	pos, alt := d.position(m, r)
	b := d.baseRatings(m, r, pos)
	rt := ratingsOf(b)
	return float64(overallOfRatings(rt, pos, alt))
}

// zq turns a percentile into a compressed normal score.
func zq(p float64) float64 {
	c := &RealTuning
	p = clampF(p, 0.003, 0.9985)
	z := math.Sqrt2 * math.Erfinv(2*p-1)
	if z > c.CompressZ {
		z = c.CompressZ + c.CompressSlop*(z-c.CompressZ)
	}
	return z
}

type ratedPlayer struct {
	Ratings Ratings
	Pos     Position
	Alt     []Position
	Impact  int
	Overall int
	Qual    bool // had enough minutes for the numbers to be trusted
}

// rate converts one player-season to ratings. drop lowers the target
// overall (used for draft prospects who enter before their first season).
func (d *realData) rate(r *statRow, drop float64) *ratedPlayer {
	c := &RealTuning
	m := d.model(r.season)
	pos, alt := d.position(m, r)
	b := d.baseRatings(m, r, pos)
	boxOvr := float64(overallOfRatings(ratingsOf(b), pos, alt))

	zb := m.zBox
	if c.BoxGroups {
		zb = m.zBoxG[boxGroup(pos)]
	}
	tBox := c.Mean + c.Sd*zq(zb.pct(boxOvr))
	tVal := c.Mean + c.Sd*zq(m.zVal.pct(m.valueZ(r)))
	w := math.Pow(math.Min(1, r.mp/c.MinQual), c.ShrinkPower) *
		math.Pow(math.Min(1, mpgOf(r)/c.ShrinkMPG), c.ShrinkMPGPower) *
		math.Pow(math.Min(1, r.g/c.ShrinkGames), c.ShrinkGamesPower)
	shrink := func(t float64) float64 { return c.Replacement + (t-c.Replacement)*w }
	tBox, tVal = shrink(tBox), shrink(tVal)
	tVis := c.BoxWeight*tBox + (1-c.BoxWeight)*tVal - drop

	// A player-season named in standing.csv with a target keeps its shape but
	// shows that overall. Its Impact is 0: the target says the formula's value
	// was wrong, so the gap to it must not lower his play in games.
	t, hasTarget := d.target[r.season][r.pid]
	if hasTarget {
		tVis = t - drop
	}
	delta := tVis - boxOvr
	var rt Ratings
	ovr := 0
	for it := 0; it < 8; it++ {
		var a [7]float64
		for i := range a {
			a[i] = b[i] + delta
		}
		rt = ratingsOf(a)
		ovr = overallOfRatings(rt, pos, alt)
		diff := tVis - float64(ovr)
		if math.Abs(diff) <= 0.5 {
			break
		}
		delta += diff
	}
	imp := clamp(int(math.Floor(c.ImpactK*(tVal-drop-float64(ovr))+0.5)), -c.ImpactMax, c.ImpactMax)
	if hasTarget {
		imp = 0
	}
	return &ratedPlayer{Ratings: rt, Pos: pos, Alt: alt, Impact: imp, Overall: ovr, Qual: r.mp >= c.MinQual}
}

func (d *realData) ratedFor(season int, pid string) *ratedPlayer {
	if d.rated[season] == nil {
		d.rated[season] = map[string]*ratedPlayer{}
	}
	if rp, ok := d.rated[season][pid]; ok {
		return rp
	}
	r := d.all[season][pid]
	if r == nil {
		return nil
	}
	rp := d.rate(r, 0)
	d.rated[season][pid] = rp
	return rp
}

// RatingsFromStats rates one player-season (the combined row for a traded
// player is chosen by the data loader) and returns ratings, positions and
// hidden impact.
func RatingsFromStats(dir string, season int, playerID string) (Ratings, Position, []Position, int, error) {
	d, err := loadRealData(dir)
	if err != nil {
		return Ratings{}, 0, nil, 0, err
	}
	rp := d.ratedFor(season, playerID)
	if rp == nil {
		return Ratings{}, 0, nil, 0, fmt.Errorf("no %d row for %s", season, playerID)
	}
	return rp.Ratings, rp.Pos, rp.Alt, rp.Impact, nil
}

// hindsightPeak is the best visible overall from `from` on (qualified
// seasons only), at least cur.
func (d *realData) hindsightPeak(pid string, from int, cur int) int {
	best := cur
	for s := from; s <= lastDataSeason; s++ {
		r := d.all[s][pid]
		if r == nil || r.mp < RealTuning.MinQual {
			continue
		}
		if rp := d.ratedFor(s, pid); rp != nil {
			best = max(best, rp.Overall)
		}
	}
	return best
}

// ceilingWeight is the share of the hindsight ceiling that can be trusted:
// full seasons of data after `after` (up to lastRealSeason) divided by the
// seasons until the player turns Progression.PotentialEqualAge, capped at 1.
func ceilingWeight(after, age int) float64 {
	need := PotentialEqual() - age
	if need <= 0 {
		return 1
	}
	have := lastRealSeason - after
	if have < 0 {
		have = 0
	}
	return math.Min(1, float64(have)/float64(need))
}

// ---------------------------------------------------------------- league

// NewRealLeague builds the league as it stood at the start of 2010-11, with
// the real draft classes of 2011 onward stored for the offseasons.
func NewRealLeague(dataDir string, seed uint64) (*League, error) {
	return NewRealLeagueSeason(dataDir, seed, firstRealSeason)
}

// NewRealLeagueSeason builds the league as it stood at the start of a real
// season (named by its ending year, 2011 to 2026), with the real draft
// classes of that year onward stored for the offseasons.
func NewRealLeagueSeason(dataDir string, seed uint64, season int) (*League, error) {
	if season < firstRealSeason || season > lastRealSeason {
		return nil, fmt.Errorf("real seasons run from %d to %d, not %d", firstRealSeason, lastRealSeason, season)
	}
	d, err := loadRealDataFrom(dataDir, season)
	if err != nil {
		return nil, err
	}
	if len(d.all[season]) == 0 {
		return nil, fmt.Errorf("no %d NBA data in %s", season, dataDir)
	}
	r := NewRNG(seed)
	lg := &League{Seed: seed, Year: season, RealSeason: season, Careers: map[int][]CareerRow{}, DraftClasses: map[int][]*Player{}}
	table := realTeamsFor(season)
	teamIdx := map[string]int{}
	for i, t := range table {
		teamIdx[t.Abbr] = i
		lg.Teams = append(lg.Teams, &Team{ID: i, City: t.City, Name: t.Name})
	}

	// Home team: where the player logged the most minutes.
	home := map[string]string{}
	var pids []string
	for pid := range d.all[season] {
		pids = append(pids, pid)
	}
	sort.Strings(pids)
	for _, pid := range pids {
		best := -1.0
		for _, tr := range d.teams[season][pid] {
			if _, ok := teamIdx[tr.team]; ok && tr.mp > best {
				best, home[pid] = tr.mp, tr.team
			}
		}
		if _, ok := home[pid]; !ok {
			if rr := d.all[season][pid]; !rr.combined {
				if _, ok := teamIdx[rr.team]; ok {
					home[pid] = rr.team
				}
			}
		}
	}
	// minutes for the player on his home team.
	minHome := func(pid string) float64 {
		for _, tr := range d.teams[season][pid] {
			if tr.team == home[pid] {
				return tr.mp
			}
		}
		return d.all[season][pid].mp
	}
	byTeam := map[string][]string{}
	for _, pid := range pids {
		if h, ok := home[pid]; ok {
			byTeam[h] = append(byTeam[h], pid)
		}
	}
	used := map[string]bool{}
	nextID := 0
	mk := func(pid string, season int) *Player {
		row := d.all[season][pid]
		rp := d.ratedFor(season, pid)
		ci := d.career[pid]
		p := &Player{
			ID: nextID, Name: row.name, Age: int(row.age), Pos: rp.Pos, AltPos: rp.Alt,
			Ratings: rp.Ratings, TeamID: -1, Impact: rp.Impact, RealID: pid,
			UsageTendency: usageFromPercentile(d.model(season).usg.pct(row.usg)),
		}
		nextID++
		if ci.from > 0 {
			p.Experience = max(1, season-ci.from+1)
		} else {
			p.Experience = max(0, p.Age-19)
		}
		if pk, ok := d.pickBy[pid]; ok {
			p.DraftYear, p.DraftPick = pk.year, pk.pick
		}
		p.Potential = rp.Overall
		if p.Age < PotentialEqual() {
			p.Potential = d.ceilingFor(seed, pid, season, p.Age, rp.Overall)
		}
		return p
	}
	for _, t := range table {
		ids := byTeam[t.Abbr]
		sort.SliceStable(ids, func(i, j int) bool {
			if a, b := minHome(ids[i]), minHome(ids[j]); a != b {
				return a > b
			}
			return ids[i] < ids[j]
		})
		tm := lg.Teams[teamIdx[t.Abbr]]
		for _, pid := range ids {
			if len(tm.Roster) >= RosterSize {
				break
			}
			p := mk(pid, season)
			p.TeamID = tm.ID
			tm.Roster = append(tm.Roster, p)
			used[pid] = true
		}
	}
	// Everyone else: free agents, best 60 by overall.
	var fas []*Player
	for _, pid := range pids {
		if !used[pid] {
			fas = append(fas, mk(pid, season))
		}
	}
	sort.SliceStable(fas, func(i, j int) bool {
		if a, b := fas[i].Overall(), fas[j].Overall(); a != b {
			return a > b
		}
		return fas[i].ID < fas[j].ID
	})
	if len(fas) > Progression.FreeAgentPool {
		fas = fas[:Progression.FreeAgentPool]
	}
	lg.FreeAgents = fas
	for _, t := range lg.Teams {
		for len(t.Roster) < RosterSize && len(lg.FreeAgents) > 0 {
			p := lg.FreeAgents[0]
			lg.FreeAgents = lg.FreeAgents[1:]
			p.TeamID = t.ID
			t.Roster = append(t.Roster, p)
		}
	}

	// Draft classes.
	years := map[int][]draftPick{}
	for _, pk := range d.picks {
		years[pk.year] = append(years[pk.year], pk)
	}
	var ys []int
	for y := range years {
		ys = append(ys, y)
	}
	sort.Ints(ys)
	for _, y := range ys {
		pks := years[y]
		sort.SliceStable(pks, func(i, j int) bool { return pks[i].pick < pks[j].pick })
		for _, pk := range pks {
			p := d.prospect(r, seed, pk, y)
			p.ID = nextID
			nextID++
			lg.DraftClasses[y] = append(lg.DraftClasses[y], p)
		}
	}
	lg.NextID = nextID
	lg.initPicks()
	lg.assignInitialContracts()
	lg.initCoaches()
	applyRealCoaches(lg, dataDir, season)
	return lg, nil
}

// blurPotential adds the league's uncertainty to a hindsight ceiling. The draw
// depends only on (league seed, player), so it is the same whatever order
// players are built in.
func blurPotential(seed uint64, pid string, peak int) int {
	sd := RealTuning.PotentialBlurSD
	if sd == 0 {
		return peak
	}
	h := fnv.New64a()
	h.Write([]byte(pid))
	r := NewRNG(mix64(seed, 0xB1A5, h.Sum64()))
	return peak + round(r.Normal(0, sd))
}

// PotentialEqual is the age from which Potential equals Overall.
func PotentialEqual() int { return Progression.PotentialEqualAge }

// usageFromPercentile maps a player's historical usage percentile onto the
// shot-tendency scale used by the fictional generator.
func usageFromPercentile(p float64) int {
	return clamp(30+round(p*60), 25, 90)
}

func (d *realData) prospect(r *RNG, seed uint64, pk draftPick, year int) *Player {
	ci := d.career[pk.pid]
	age := 20
	if ci.birthYear > 0 {
		age = clamp(year-ci.birthYear, 18, 25)
	}
	var row *statRow
	first := 0
	for s := year + 1; s <= lastDataSeason; s++ {
		if x := d.all[s][pk.pid]; x != nil && x.mp > 0 {
			row, first = x, s
			break
		}
	}
	p := &Player{Name: pk.name, Age: age, TeamID: -1, DraftYear: year, DraftPick: pk.pick, RealID: pk.pid}
	if row == nil {
		// No real season yet: his rookie overall is drawn from his pick
		// band's first-season distribution (the shape comes from the
		// fictional generator), his ceiling from the band's peak distribution.
		pos := Position(r.IntN(5))
		bp := buildPlayer(r, 0, pos, 36+r.Normal(0, 3), age)
		want := clamp(round(d.pedigreeFirst(seed, pk, year)), 30, 80)
		p.Pos, p.AltPos, p.Impact, p.UsageTendency = pos, bp.AltPos, 0, bp.UsageTendency
		p.Ratings = shiftToOverall(bp.Ratings, pos, bp.AltPos, want)
		ovr := p.Overall()
		p.Potential = clamp(max(round(d.pedigreeCeiling(seed, pk.pid, age)), ovr), ovr, 99)
		return p
	}
	rp := d.rate(row, RealTuning.RookieDrop)
	// A first season with few minutes says little: blend toward the band's
	// median first-season overall by minutes played.
	if w := math.Min(1, row.mp/RealTuning.RookieBlendMP); w < 1 {
		med := d.pedigreeFirstMedian(pk, year)
		want := w*float64(rp.Overall) + (1-w)*med
		rp = d.rate(row, RealTuning.RookieDrop+float64(rp.Overall)-want)
	}
	p.Pos, p.AltPos, p.Ratings, p.Impact = rp.Pos, rp.Alt, rp.Ratings, rp.Impact
	p.UsageTendency = usageFromPercentile(d.model(first).usg.pct(row.usg))
	p.Potential = d.ceilingFor(seed, pk.pid, first, p.Age, p.Overall())
	return p
}

// takeDraftClass removes and returns the stored class for a draft year,
// padded or trimmed to the draft size, or nil if there is none.
func (lg *League) takeDraftClass(year int) []*Player {
	cls := lg.DraftClasses[year]
	if len(cls) == 0 {
		return nil
	}
	delete(lg.DraftClasses, year)
	size := Progression.DraftSize
	if len(cls) > size {
		cls = cls[:size]
	}
	r := lg.dayRNG(year, offseasonOffset+7)
	for len(cls) < size {
		cls = append(cls, GenerateRookie(r, lg.NextID, year))
		lg.NextID++
	}
	for _, p := range cls {
		p.DraftYear = year
	}
	return cls
}

// ---------------------------------------------------------------- reference

// RealPerGame is one player's real per-game line.
type RealPerGame struct {
	Name          string
	G             int
	PPG, RPG, APG float64
}

// RealPerGameStats returns real per-game lines for a season by player ID
// (combined row for traded players).
func RealPerGameStats(dir string, season int) (map[string]RealPerGame, error) {
	d, err := loadRealData(dir)
	if err != nil {
		return nil, err
	}
	out := map[string]RealPerGame{}
	for pid, r := range d.all[season] {
		if r.g > 0 {
			out[pid] = RealPerGame{r.name, int(r.g), r.pts / r.g, r.trb / r.g, r.ast / r.g}
		}
	}
	return out, nil
}

// RealTeamWins returns real wins by team abbreviation.
func RealTeamWins(dir string, season int) (map[string]int, error) {
	rows, err := readCSV(filepath.Join(dir, "Team Summaries.csv"))
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, m := range rows {
		if m["lg"] == "NBA" && int(num(m["season"])) == season && m["abbreviation"] != "NA" {
			out[m["abbreviation"]] = int(num(m["w"]))
		}
	}
	return out, nil
}

// RealLeaguePPG is the real league average points per team per game.
func RealLeaguePPG(dir string, season int) (float64, error) {
	d, err := loadRealData(dir)
	if err != nil {
		return 0, err
	}
	pts := 0.0
	for _, rows := range d.teams[season] {
		for _, r := range rows {
			pts += r.pts
		}
	}
	return pts / float64(NumTeams*SeasonGames), nil
}
