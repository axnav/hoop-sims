package engine

import "math"

// Tunable constants for the possession model.
const (
	baseTov       = 0.115
	foulChance    = 0.27
	ftBase        = 0.76
	assistChance  = 0.80
	orebBase      = 0.26
	homeEdge      = 0.018
	baseInside    = 0.595
	baseMid       = 0.43
	baseThree     = 0.35
	midShotWeight = 0.45
	threeShotWt   = 0.9
	possMean      = 98.0
	possSD        = 3.0
	overtimePoss  = 10
	skillCoef     = 0.0020 // FG% per rating point of shooter skill
	defCoef       = 0.0016 // FG% per rating point of team defense
	shootExp      = 1.3    // shot-share concentration (higher = stars shoot more)
)

var minutesByRank = [RosterSize]int{35, 34, 32, 30, 28, 22, 18, 14, 10, 8, 5, 4, 0}

type StatLine struct {
	Player                       *Player
	Min                          int
	Pts, Reb, Ast, Tov           int
	FGM, FGA, TPM, TPA, FTM, FTA int
}

type TeamBox struct {
	Team  *Team
	Score int
	Lines []*StatLine
}

type GameResult struct {
	Home, Away *TeamBox
	Overtimes  int
	Events     []Event // only filled by SimGameRec with record=true
}

// Event types in the play-by-play log.
const (
	EvMade2 = "2"    // made two
	EvMade3 = "3"    // made three
	EvMiss2 = "m2"   // missed two
	EvMiss3 = "m3"   // missed three
	EvFT    = "ft"   // two free throws, N made
	EvTov   = "tov"  // turnover
	EvOReb  = "oreb" // offensive rebound
	EvDReb  = "dreb" // defensive rebound
)

// Event is one play. HS/AS are the running home/away scores after the play,
// Per is the period (5+ = overtime), Clk the seconds left in that period.
type Event struct {
	Per    int    `json:"p"`
	Clk    int    `json:"c"`
	Team   int    `json:"t"`
	Player int    `json:"pl"`
	Ast    int    `json:"a"` // assisting player ID or -1
	Type   string `json:"y"`
	N      int    `json:"n,omitempty"`
	HS     int    `json:"hs"`
	AS     int    `json:"as"`
}

type recorder struct {
	events   []Event
	h, a     *side
	per, clk int
}

func (rc *recorder) add(s *side, pi int, typ string, n, ast int) {
	rc.events = append(rc.events, Event{Per: rc.per, Clk: rc.clk, Team: s.box.Team.ID,
		Player: s.box.Lines[pi].Player.ID, Ast: ast, Type: typ, N: n,
		HS: rc.h.box.Score, AS: rc.a.box.Score})
}

func (rc *recorder) at(idx, total, length, period int) {
	el := idx * length / total
	rc.per, rc.clk = period, length-el
}

// side holds per-team precomputed weights for one game.
type side struct {
	box              *TeamBox
	shootW, astW     []float64
	rebW             []float64
	eff              []Ratings
	defense, rebound float64
	home             bool
	rec              *recorder
	bd               *sideBadges
	// From the coach and the system (coach.go, system.go); all zero or the
	// plain constants without a league.
	teamAdd float64 // make-chance nudge: coach Offense and team fit, against the league mean
	pace    float64 // possessions added by the system
	threeWt float64 // weight of three-point shots in the shot-kind choice
	// moraleAdd is each player's make-chance nudge from his mood, against the
	// league mean (morale.go); zeros without a league.
	moraleAdd []float64
}

// newSide precomputes a team's weights and badge effects for one game. lg may
// be nil (no personality badges).
func newSide(lg *League, t *Team, home bool) *side {
	s := &side{box: &TeamBox{Team: t}, home: home, threeWt: threeShotWt}
	pms := t.EffectiveMinutes()
	exp := shootExp
	var sys *System
	var sets []BadgeSet
	if lg != nil {
		sys = t.systemOf()
		exp += sys.StarExp
		s.threeWt = threeShotWt * sys.ThreeMult
		sets = make([]BadgeSet, 0, len(pms))
	}
	s.bd = newSideBadges(len(pms))
	var totMin, def, reb float64
	var means *badgeMeans
	var mm float64
	if lg != nil {
		mm = lg.moraleMeansNow().mean
		means = lg.badgeMeansNow()
		for _, pm := range pms {
			totMin += float64(pm.Min)
		}
	}
	teamMin := totMin
	totMin = 0
	for i, pm := range pms {
		p, m := pm.Player, pm.Min
		if lg != nil {
			bs := lg.badgeSet(p)
			sets = append(sets, bs)
			s.bd.add(bs, means, i, float64(m), teamMin)
		}
		s.box.Lines = append(s.box.Lines, &StatLine{Player: p, Min: m})
		s.moraleAdd = append(s.moraleAdd, moraleEffect(p, mm))
		rt := p.Effective()
		s.eff = append(s.eff, rt)
		top := max(rt.Inside, rt.Mid, rt.Three)
		second := rt.Inside + rt.Mid + rt.Three - top - min(rt.Inside, rt.Mid, rt.Three)
		// Shooting ability determines how good a player is at scoring; UsageTendency
		// determines how often he looks for his own shot. Playmaking is used for
		// creation and assists, not as an automatic boost to shot volume.
		scoring := (float64(top) + float64(second)) / 2
		fm := float64(m)
		usageWeight := 0.8 + float64(p.ShotUsage())/250
		s.shootW = append(s.shootW, fm*math.Pow(scoring, exp)*usageWeight*s.bd.usage[i])
		pm := float64(rt.Playmaking)
		s.astW = append(s.astW, fm*math.Pow(pm, 2.5)*s.bd.astMult[i])
		rb := float64(rt.Rebounding)
		s.rebW = append(s.rebW, fm*rb*rb)
		totMin += fm
		def += fm * float64(rt.Defense)
		reb += fm * rb
	}
	s.defense = def/totMin + s.bd.defT
	s.rebound = reb / totMin
	if lg != nil {
		s.bd.finish(means)
		s.applyCoach(lg, t, sys, pms, sets)
	}
	return s
}

func SimGame(r *RNG, home, away *Team) *GameResult { return SimGameRec(r, home, away, false) }

// SimGameRec is SimGame with an optional play-by-play log. Recording never
// touches the random stream, so results are identical either way. It knows
// no league, so personality badges (Leader, Star) have no effect.
func SimGameRec(r *RNG, home, away *Team, record bool) *GameResult {
	return (*League)(nil).SimGameRec(r, home, away, record)
}

// SimGame plays one game with this league's badges, personality included.
func (lg *League) SimGame(r *RNG, home, away *Team) *GameResult {
	return lg.SimGameRec(r, home, away, false)
}

// SimGameRec is SimGame with an optional play-by-play log (see the plain
// SimGameRec). The league may be nil.
func (lg *League) SimGameRec(r *RNG, home, away *Team, record bool) *GameResult {
	h, a := newSide(lg, home, true), newSide(lg, away, false)
	var rc *recorder
	if record {
		rc = &recorder{h: h, a: a}
		h.rec, a.rec = rc, rc
	}
	n := int(r.Normal(possMean+h.bd.paceT+a.bd.paceT+h.pace+a.pace, possSD))
	for i := 0; i < n; i++ {
		if rc != nil {
			rc.per, rc.clk = rc.clk0(2*i, 2*n)
		}
		possession(r, h, a, lateAndClose(h, a, i, n))
		if rc != nil {
			rc.per, rc.clk = rc.clk0(2*i+1, 2*n)
		}
		possession(r, a, h, lateAndClose(h, a, i, n))
	}
	res := &GameResult{Home: h.box, Away: a.box}
	for h.box.Score == a.box.Score {
		res.Overtimes++
		for i := 0; i < overtimePoss; i++ {
			if rc != nil {
				rc.at(2*i, 2*overtimePoss, 300, 4+res.Overtimes)
			}
			possession(r, h, a, lateAndClose(h, a, n, n))
			if rc != nil {
				rc.at(2*i+1, 2*overtimePoss, 300, 4+res.Overtimes)
			}
			possession(r, a, h, lateAndClose(h, a, n, n))
		}
	}
	if rc != nil {
		res.Events = rc.events
	}
	return res
}

// clk0 maps a regulation possession index to (period, seconds left).
func (rc *recorder) clk0(idx, total int) (int, int) {
	el := idx * 2880 / total
	return el/720 + 1, 720 - el%720
}

// lateAndClose says whether this possession (number i of n per team) is in the
// last ClutchShare of regulation, or overtime (i >= n), with the score close.
func lateAndClose(h, a *side, i, n int) bool {
	t := &BadgeTuning
	if i < n && float64(i) < float64(n)*(1-t.ClutchShare) {
		return false
	}
	d := h.box.Score - a.box.Score
	return d <= t.ClutchMargin && d >= -t.ClutchMargin
}

func possession(r *RNG, off, def *side, clutch bool) {
	for attempt := 0; attempt < 3; attempt++ {
		si := r.PickWeighted(off.shootW)
		sl := off.box.Lines[si]
		rt := off.eff[si]

		tov := baseTov + (def.defense-60)*0.003 - (float64(rt.Playmaking)-60)*0.0008 - off.bd.tovCut[si] + def.bd.stealT
		if r.Chance(tov) {
			sl.Tov++
			if off.rec != nil {
				off.rec.add(off, si, EvTov, 0, -1)
			}
			return
		}

		in, mid, three := float64(rt.Inside), float64(rt.Mid), float64(rt.Three)
		kind := r.PickWeighted([]float64{in * in, mid * mid * midShotWeight, three * three * off.threeWt})

		if kind == 0 && r.Chance(foulChance+off.bd.foulAdd[si]-def.bd.foulT) {
			p := ftBase + (mid-60)*0.004 + off.bd.ftAdd[si]
			ftm0 := sl.FTM
			for i := 0; i < 2; i++ {
				sl.FTA++
				if r.Chance(p) {
					sl.FTM++
					sl.Pts++
					off.box.Score++
				}
			}
			if off.rec != nil {
				off.rec.add(off, si, EvFT, sl.FTM-ftm0, -1)
			}
			return
		}

		var base, skill float64
		pts := 2
		switch kind {
		case 0:
			base, skill = baseInside, in
		case 1:
			base, skill = baseMid, mid
		default:
			base, skill, pts = baseThree, three, 3
		}
		p := base + (skill-60)*skillCoef - (def.defense-60)*defCoef
		if off.home {
			p += homeEdge
		}
		p += off.bd.kind[si][kind] + off.bd.leader + off.teamAdd + off.moraleAdd[si]
		if clutch {
			p += off.bd.clutch[si]
		}
		if kind == 0 {
			p -= def.bd.rimT
		}
		sl.FGA++
		if pts == 3 {
			sl.TPA++
		}
		if r.Chance(p) {
			sl.FGM++
			if pts == 3 {
				sl.TPM++
			}
			sl.Pts += pts
			off.box.Score += pts
			ast := -1
			if r.Chance(min(assistChance+off.bd.astT, 0.97)) {
				if ai := r.PickWeighted(off.astW); ai != si {
					off.box.Lines[ai].Ast++
					ast = off.box.Lines[ai].Player.ID
				}
			}
			if off.rec != nil {
				typ := EvMade2
				if pts == 3 {
					typ = EvMade3
				}
				off.rec.add(off, si, typ, 0, ast)
			}
			return
		}

		if off.rec != nil {
			typ := EvMiss2
			if pts == 3 {
				typ = EvMiss3
			}
			off.rec.add(off, si, typ, 0, -1)
		}
		oreb := orebBase + (off.rebound+off.bd.orebT-def.rebound-def.bd.drebT)*0.004
		if r.Chance(oreb) {
			ri := r.PickWeighted(off.rebW)
			off.box.Lines[ri].Reb++
			if off.rec != nil {
				off.rec.add(off, ri, EvOReb, 0, -1)
			}
			continue
		}
		ri := r.PickWeighted(def.rebW)
		def.box.Lines[ri].Reb++
		if off.rec != nil {
			off.rec.add(def, ri, EvDReb, 0, -1)
		}
		return
	}
}
