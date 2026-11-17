package engine

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// Coaches. Every team has one (Team.Coach, saved). A coach has an Offense and
// a Defense rating (a small nudge to the team's shooting and defense), a
// Development rating (faster or slower growth for the roster) and a favourite
// system. All three effects are measured against the league average, so the
// league as a whole plays as before. Names are fictional in both modes.
//
// Coaches are hired and fired in the offseason only: BeginOffseason fills
// League.CoachPool with candidates, the user may hire one (HireCoach, the old
// coach is gone), computer teams replace weak coaches and choose their system
// in FinishOffseason, which also empties the pool. Random stream offset
// 110000 (RULES.md section 3): every draw is hashed or from its own stream.

// coachOffset is the RNG stream for coach generation and offseason decisions.
const coachOffset = 110000

// Coach is a team's head coach.
type Coach struct {
	ID          int `json:",omitempty"` // candidate number while in the pool
	Name        string
	Age         int
	Offense     int
	Defense     int
	Development int
	System      string // favourite system key
}

// CoachTuning holds every number behind coaches. Edit here.
var CoachTuning = struct {
	// Generation: ratings are Normal(RatingMean, RatingSD) clamped to RatingMin..RatingMax.
	RatingMean, RatingSD float64
	RatingMin, RatingMax int
	AgeMin, AgeMax       int
	PoolSize             int
	// Offense: make chance per rating point above the league mean, ceiling.
	OffScale, OffCap float64
	// Defense: team defense rating points per coach point above the mean, ceiling.
	DefScale, DefCap float64
	// Development: growth is multiplied by 1 + DevRange at DevHalf points above
	// the league mean (and 1 - DevRange at DevHalf below), linear between.
	DevRange, DevHalf float64
	// Fit: make chance per point of team fit above the league mean, ceiling.
	FitScale, FitCap float64
	// FavouritePenalty: team fit points lost by running a system other than the
	// coach's favourite.
	FavouritePenalty float64
	// Computer teams: a coach whose average of the three ratings is below
	// ReplaceBelow is replaced with chance ReplaceLosing after a season of
	// LosingWins wins or fewer, ReplaceWeak otherwise; the new coach must be
	// better by ReplaceMargin points of the average.
	ReplaceBelow, ReplaceLosing, ReplaceWeak, ReplaceMargin float64
	LosingWins                                              int
	// A computer team leaves the favourite system for the best-fitting one only
	// when it beats the favourite by SystemMargin team fit points (penalty included).
	SystemMargin float64
}{
	RatingMean: 66, RatingSD: 11, RatingMin: 40, RatingMax: 95,
	AgeMin: 38, AgeMax: 68, PoolSize: 12,
	OffScale: 0.00012, OffCap: 0.004,
	DefScale: 0.055, DefCap: 2.5,
	DevRange: 0.08, DevHalf: 25,
	FitScale: 0.00022, FitCap: 0.007,
	FavouritePenalty: 8,
	ReplaceBelow:     60, ReplaceLosing: 0.6, ReplaceWeak: 0.15, ReplaceMargin: 4,
	LosingWins:   35,
	SystemMargin: 5,
}

// Overall is the average of the three ratings.
func (c *Coach) Overall() float64 {
	return float64(c.Offense+c.Defense+c.Development) / 3
}

func newCoach(r *RNG) *Coach {
	t := &CoachTuning
	rate := func() int {
		return clamp(round(r.Normal(t.RatingMean, t.RatingSD)), t.RatingMin, t.RatingMax)
	}
	c := &Coach{Name: Pick(r, firstNames) + " " + Pick(r, lastNames), Age: r.Between(t.AgeMin, t.AgeMax)}
	c.Offense, c.Defense, c.Development = rate(), rate(), rate()
	c.System = Systems[r.IntN(len(Systems))].Key
	return c
}

// initCoaches gives every team without a coach a generated one: one hashed
// stream per team (league seed and team ID), so it never depends on order.
// Used when a league is created and by the version 3 -> 4 save upgrade.
func (lg *League) initCoaches() {
	for _, t := range lg.Teams {
		if t.Coach == nil {
			t.Coach = newCoach(NewRNG(mix64(lg.Seed, coachOffset, 0, uint64(t.ID))))
		}
	}
	lg.cmeans = nil
}

// coachMeans are the league averages the effects are measured against:
// coach ratings and team fit (derived from the current rosters, never saved;
// valid for as long as the badge means they were built beside).
type coachMeans struct {
	src            *badgeMeans
	off, def, fit  float64
	teams, coaches int
}

// systemOf is the system the team runs: its own choice, else the coach's
// favourite, else Balanced.
func (t *Team) systemOf() *System {
	if s := SystemByKey(t.System); s != nil {
		return s
	}
	if t.Coach != nil {
		if s := SystemByKey(t.Coach.System); s != nil {
			return s
		}
	}
	return &Systems[0]
}

// OnFavourite says whether the team runs its coach's favourite system (a team
// without a coach has none to miss).
func (t *Team) OnFavourite() bool {
	return t.Coach == nil || t.System == "" || t.System == t.Coach.System
}

// SystemKey is the key of the system the team runs.
func (t *Team) SystemKey() string { return t.systemOf().Key }

// teamFit is the team's fit under sys: minutes-weighted average of the fit
// scores of the players with minutes (sets are their badges, in pms order),
// minus the penalty when sys is not the coach's favourite.
func (t *Team) teamFit(sys *System, pms []PlayerMinutes, sets []BadgeSet) float64 {
	var sum, w float64
	for i, pm := range pms {
		if pm.Min <= 0 {
			continue
		}
		sum += float64(pm.Min) * sys.fitScore(inputsOf(pm.Player), sets[i])
		w += float64(pm.Min)
	}
	if w == 0 {
		return 0
	}
	f := sum / w
	if t.Coach != nil && sys.Key != t.Coach.System {
		f -= CoachTuning.FavouritePenalty
	}
	return f
}

// TeamFitUnder is the team's fit if it ran the system with this key (the
// penalty for leaving the coach's favourite included).
func (lg *League) TeamFitUnder(t *Team, key string) float64 {
	sys := SystemByKey(key)
	if sys == nil {
		sys = t.systemOf()
	}
	pms := t.EffectiveMinutes()
	sets := make([]BadgeSet, len(pms))
	for i, pm := range pms {
		sets[i] = lg.badgeSet(pm.Player)
	}
	return t.teamFit(sys, pms, sets)
}

// TeamFit is the team's fit with the system it runs now.
func (lg *League) TeamFit(t *Team) float64 { return lg.TeamFitUnder(t, t.SystemKey()) }

// PlayerFit is the player's fit with the team's current system.
func (lg *League) PlayerFit(t *Team, p *Player) Fit {
	return lg.PlayerFitUnder(t.systemOf(), p)
}

// PlayerFitUnder is the player's fit with the given system (no off-favourite
// penalty: that is a team matter).
func (lg *League) PlayerFitUnder(sys *System, p *Player) Fit {
	return sys.FitFor(p, lg.badgeSet(p))
}

// coachMeansNow returns the league means, building them when missing or when
// the badge means were rebuilt (a new day of games).
func (lg *League) coachMeansNow() *coachMeans {
	bm := lg.badgeMeansNow()
	if lg.cmeans != nil && lg.cmeans.src == bm {
		return lg.cmeans
	}
	m := &coachMeans{src: bm}
	for _, t := range lg.Teams {
		if c := t.Coach; c != nil {
			m.off += float64(c.Offense)
			m.def += float64(c.Defense)
			m.coaches++
		}
		pms := t.EffectiveMinutes()
		if len(pms) == 0 {
			continue
		}
		sets := make([]BadgeSet, len(pms))
		for i, pm := range pms {
			sets[i] = lg.badgeSet(pm.Player)
		}
		m.fit += t.teamFit(t.systemOf(), pms, sets)
		m.teams++
	}
	if m.coaches > 0 {
		m.off /= float64(m.coaches)
		m.def /= float64(m.coaches)
	}
	if m.teams > 0 {
		m.fit /= float64(m.teams)
	}
	lg.cmeans = m
	return m
}

// meanDevelopment is the league average Development rating.
func (lg *League) meanDevelopment() float64 {
	s, n := 0.0, 0
	for _, t := range lg.Teams {
		if t.Coach != nil {
			s += float64(t.Coach.Development)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return s / float64(n)
}

// devMult is the growth multiplier from the team's coach: 1 + DevRange at
// DevHalf points above the league mean Development, 1 - DevRange at DevHalf
// below. Players without a team (or a team without a coach) get 1.
func (lg *League) devMult(teamID int) float64 {
	if teamID < 0 || teamID >= len(lg.Teams) || lg.Teams[teamID].Coach == nil {
		return 1
	}
	t := &CoachTuning
	x := (float64(lg.Teams[teamID].Coach.Development) - lg.meanDevelopment()) / t.DevHalf
	return 1 + t.DevRange*math.Max(-1, math.Min(1, x))
}

// ---- offseason ----

// Errors for the user's coach changes.
var (
	ErrCoachPoolClosed = errors.New("coaches can only be hired in the offseason, before the next season starts")
	ErrNoSuchCoach     = errors.New("no such coach candidate")
	ErrUnknownSystem   = errors.New("unknown system")
)

// CoachPoolOpen says whether the offseason candidate pool is open.
func (lg *League) CoachPoolOpen() bool { return len(lg.CoachPool) > 0 }

// openCoachPool fills the candidate pool (called when the offseason begins).
func (lg *League) openCoachPool() {
	lg.CoachPool = nil
	for i := 0; i < CoachTuning.PoolSize; i++ {
		c := newCoach(lg.dayRNG(lg.Year, coachOffset+1+i))
		c.ID = i
		lg.CoachPool = append(lg.CoachPool, c)
	}
}

// HireCoach fires the user's coach and hires the candidate with this ID. The
// old coach is gone, the hired one leaves the pool, and the team runs the new
// coach's favourite system.
func (lg *League) HireCoach(candidateID int) error {
	if !lg.CoachPoolOpen() {
		return ErrCoachPoolClosed
	}
	for i, c := range lg.CoachPool {
		if c.ID == candidateID {
			t := lg.Teams[lg.UserTeamID]
			hired := *c
			hired.ID = 0
			lg.newsCoach(t.ID, t.Coach, &hired) // news.go
			t.Coach = &hired
			t.System = ""
			lg.CoachPool = append(lg.CoachPool[:i:i], lg.CoachPool[i+1:]...)
			lg.cmeans = nil
			return nil
		}
	}
	return ErrNoSuchCoach
}

// SetSystem makes the team run the system with this key.
func (lg *League) SetSystem(teamID int, key string) error {
	if !ValidSystemKey(key) {
		return fmt.Errorf("%w: %q", ErrUnknownSystem, key)
	}
	lg.Teams[teamID].System = key
	lg.cmeans = nil
	return nil
}

// BestSystem is the system with the best team fit for the team's roster, and
// the fit under it. Ties go to the earlier system in the list.
func (lg *League) BestSystem(t *Team) (*System, float64) {
	pms := t.EffectiveMinutes()
	sets := make([]BadgeSet, len(pms))
	for i, pm := range pms {
		sets[i] = lg.badgeSet(pm.Player)
	}
	var best *System
	bf := math.Inf(-1)
	for i := range Systems {
		if f := t.teamFit(&Systems[i], pms, sets); f > bf+1e-9 {
			best, bf = &Systems[i], f
		}
	}
	return best, bf
}

// finishCoaches runs when the offseason ends, after rosters are final: weak
// coaches of computer teams are replaced from the pool (worst record first),
// every computer team picks its system, and the pool is emptied. The user's
// team is never touched.
func (lg *League) finishCoaches() {
	t := &CoachTuning
	r := lg.dayRNG(lg.Year, coachOffset+50)
	wins := func(id int) int {
		if lg.Season != nil && id < len(lg.Season.Wins) {
			return lg.Season.Wins[id]
		}
		return 41
	}
	order := make([]int, 0, len(lg.Teams))
	for _, tm := range lg.Teams {
		if tm.ID != lg.UserTeamID {
			order = append(order, tm.ID)
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		if wi, wj := wins(order[i]), wins(order[j]); wi != wj {
			return wi < wj
		}
		return order[i] < order[j]
	})
	for _, id := range order {
		tm := lg.Teams[id]
		c := tm.Coach
		if c == nil || c.Overall() >= t.ReplaceBelow {
			continue
		}
		chance := t.ReplaceWeak
		if wins(id) <= t.LosingWins {
			chance = t.ReplaceLosing
		}
		x := r.Float64() // always drawn, so the stream does not depend on the pool
		if x >= chance || len(lg.CoachPool) == 0 {
			continue
		}
		bi := 0
		for i, cand := range lg.CoachPool {
			if cand.Overall() > lg.CoachPool[bi].Overall() {
				bi = i
			}
		}
		if lg.CoachPool[bi].Overall() < c.Overall()+t.ReplaceMargin {
			continue
		}
		hired := *lg.CoachPool[bi]
		hired.ID = 0
		lg.newsCoach(tm.ID, tm.Coach, &hired) // news.go
		tm.Coach = &hired
		tm.System = ""
		lg.CoachPool = append(lg.CoachPool[:bi:bi], lg.CoachPool[bi+1:]...)
	}
	lg.CoachPool = nil
	lg.cmeans = nil
	for _, id := range order {
		lg.chooseSystem(lg.Teams[id])
	}
}

// chooseSystem is a computer team's choice: the coach's favourite, unless the
// best-fitting system for the roster beats it by SystemMargin (the penalty for
// leaving the favourite included). The team keeps "" for the favourite.
func (lg *League) chooseSystem(tm *Team) {
	tm.System = ""
	if tm.Coach == nil {
		return
	}
	pms := tm.EffectiveMinutes()
	sets := make([]BadgeSet, len(pms))
	for i, pm := range pms {
		sets[i] = lg.badgeSet(pm.Player)
	}
	fav := SystemByKey(tm.Coach.System)
	if fav == nil {
		return
	}
	favFit := tm.teamFit(fav, pms, sets)
	var best *System
	bf := favFit + CoachTuning.SystemMargin
	for i := range Systems {
		if Systems[i].Key == fav.Key {
			continue
		}
		if f := tm.teamFit(&Systems[i], pms, sets); f > bf {
			best, bf = &Systems[i], f
		}
	}
	if best != nil {
		tm.System = best.Key
	}
}

// applyCoach fills the side's coach and system numbers once per game: the
// system's pace and defense, the coach's Offense (make chance) and Defense,
// and the team fit, each measured against the league mean so the league as a
// whole is unchanged.
func (s *side) applyCoach(lg *League, t *Team, sys *System, pms []PlayerMinutes, sets []BadgeSet) {
	ct := &CoachTuning
	m := lg.coachMeansNow()
	s.pace = sys.Pace
	s.defense += sys.Def
	s.teamAdd = clampf(ct.FitScale*(t.teamFit(sys, pms, sets)-m.fit), ct.FitCap)
	if c := t.Coach; c != nil {
		s.teamAdd += clampf(ct.OffScale*(float64(c.Offense)-m.off), ct.OffCap)
		s.defense += clampf(ct.DefScale*(float64(c.Defense)-m.def), ct.DefCap)
	}
}

// CurrentSystem is the system the team runs now.
func (t *Team) CurrentSystem() *System { return t.systemOf() }
