package engine

import "math"

// ProgressionConfigT holds every tunable number for player development,
// retirement, rookies and roster management. Edit Progression below.
type ProgressionConfigT struct {
	// True ceiling = Potential + Impact * CeilingImpactMult (hidden).
	CeilingImpactMult float64
	// CeilingOffset is added to the true ceiling: growth closes only part of
	// the gap each year, so without it players finish a few points short of
	// their visible Potential.
	CeilingOffset float64

	// Growth phase (age < PrimeStartAge): delta = (ceiling - Overall)*rate + Normal(0, GrowthNoiseSD).
	GrowthNoiseSD float64
	RateAge21     float64 // rate at age <= 21
	RateAge24     float64 // rate at ages 22-24
	RateAge26     float64 // rate at ages 25-26
	PrimeStartAge int     // first prime age (growth below this)

	// Prime phase: delta = Normal(0, PrimeNoiseSD) + Impact*PrimeImpactMult.
	PrimeNoiseSD    float64
	PrimeImpactMult float64

	// Decline phase (age >= DeclineStartAge):
	// delta = -(age-DeclineBaseAge)*DeclinePerYear + Normal(0, DeclineNoiseSD) + Impact*DeclineImpactMult.
	DeclineStartAge   int
	DeclineBaseAge    int
	DeclinePerYear    float64
	DeclineNoiseSD    float64
	DeclineImpactMult float64

	// Per-rating application of the delta.
	RatingNoiseSD          float64 // extra per-rating noise
	AthleticismDeclineMult float64 // athleticism falls this much faster in decline
	ShootingDeclineMult    float64 // Mid/Three fall this fraction as fast in decline
	AthleticismGrowthStop  int     // no athleticism growth at or after this age

	// Leap / stall events (players under LeapMaxAge, one roll per offseason).
	LeapMaxAge  int
	LeapChance  float64
	LeapMin     float64
	LeapMax     float64
	StallChance float64
	StallMin    float64 // magnitude of the drop
	StallMax    float64

	// Minutes-based development (players aged MinutesDevMaxAge or under, from
	// their regular-season minutes per team game, see MinutesAdjust). At or
	// above MinutesDevFull minutes the yearly change gets +MinutesDevBonus
	// rating points, at or below MinutesDevNone it gets -MinutesDevPenalty,
	// linear in between with zero at MinutesDevNeutral. Both 0 = switched off.
	MinutesDevMaxAge  int
	MinutesDevFull    float64
	MinutesDevNone    float64
	MinutesDevNeutral float64
	MinutesDevBonus   float64
	MinutesDevPenalty float64

	// In-season development: an advance on the yearly change. InSeasonShare of
	// the expected yearly trend arrives in InSeasonSteps equal steps, one after
	// every InSeasonEvery-th day played; the offseason then only adds the rest.
	// Each step also gets Normal(0, InSeasonNoiseSD) overall points of noise.
	InSeasonShare   float64
	InSeasonEvery   int
	InSeasonSteps   int
	InSeasonNoiseSD float64

	// Visible Potential estimate.
	PotentialLearnRate float64 // share of the gap to true ceiling closed per year
	PotentialEqualAge  int     // from this age Potential == Overall

	// Retirement.
	RetireStartAge   int
	RetireAgeStep    float64
	RetireLowOverall int
	RetireLowBonus   float64
	RetireForceAge   int

	// Draft and rookies.
	DraftSize          int
	DraftOvrWeight     float64
	DraftPotWeight     float64
	RookieMinAge       int
	RookieMaxAge       int
	RookieTalentMean   float64 // current talent at age 19
	RookieTalentPerYr  float64 // extra current talent per year of age above 19
	RookiePotMean      float64 // mean of the potential target
	RookiePotSD        float64
	RookieEliteChance  float64 // chance of an elite prospect
	RookieEliteBonus   float64 // mean extra potential for elites
	RookieEliteSD      float64
	RookieStarEvery    int // every Nth rookie ID is a franchise-level prospect (guaranteed supply, DraftSize/N per class)
	RookieStarMean     float64
	RookieStarSD       float64
	RookiePotAgeDrop   float64 // potential target lost per year of age above 19
	RookieCurrentShare float64 // share of (potential target - mean) that shows up as current talent

	// ImpactSD is the spread of the hidden Impact given to new players.
	ImpactSD float64

	// Initial league: mean peak talent and chance of a star-level peak.
	InitialPeakMean    float64
	InitialEliteChance float64
	// YoungOverallCap limits the overall of players under 21 in a new league.
	YoungOverallCap int

	// Rosters.
	YouthBonusAge     int     // roster-keep score adds max(0, YouthBonusAge-age) * YouthBonusPerYear
	YouthBonusPerYear float64 // 1.0 = plain spec formula; higher protects young prospects
	FreeAgentPool     int
}

var Progression = ProgressionConfigT{
	CeilingImpactMult: 2.0,
	CeilingOffset:     0,

	GrowthNoiseSD: 2.0,
	RateAge21:     0.30,
	RateAge24:     0.25,
	RateAge26:     0.20,
	PrimeStartAge: 27,

	PrimeNoiseSD:    1.0,
	PrimeImpactMult: 0.1,

	DeclineStartAge:   31,
	DeclineBaseAge:    30,
	DeclinePerYear:    0.6,
	DeclineNoiseSD:    1.2,
	DeclineImpactMult: 0.1,

	RatingNoiseSD:          1.2,
	AthleticismDeclineMult: 1.5,
	ShootingDeclineMult:    0.5,
	AthleticismGrowthStop:  25,

	LeapMaxAge:  25,
	LeapChance:  0.06,
	LeapMin:     3,
	LeapMax:     5,
	StallChance: 0.06,
	StallMin:    2,
	StallMax:    4,

	MinutesDevMaxAge:  24,
	MinutesDevFull:    26.0,
	MinutesDevNone:    6.0,
	MinutesDevNeutral: 15.5,
	MinutesDevBonus:   0.8,
	MinutesDevPenalty: 0.6,

	InSeasonShare:   0.5,
	InSeasonEvery:   16,
	InSeasonSteps:   4,
	InSeasonNoiseSD: 0.25,

	PotentialLearnRate: 0.20,
	PotentialEqualAge:  27,

	RetireStartAge:   34,
	RetireAgeStep:    0.15,
	RetireLowOverall: 60,
	RetireLowBonus:   0.30,
	RetireForceAge:   40,

	DraftSize:          60,
	DraftOvrWeight:     0.4,
	DraftPotWeight:     0.6,
	RookieMinAge:       19,
	RookieMaxAge:       22,
	RookieTalentMean:   43.5,
	RookieTalentPerYr:  2,
	RookiePotMean:      43,
	RookiePotSD:        8,
	RookieEliteChance:  0.255,
	RookieEliteBonus:   24,
	RookieEliteSD:      7,
	RookieStarEvery:    30,
	RookieStarMean:     89,
	RookieStarSD:       2,
	RookiePotAgeDrop:   1,
	RookieCurrentShare: 0.4,

	ImpactSD: 1.8,

	InitialPeakMean:    63,
	InitialEliteChance: 0.07,
	YoungOverallCap:    80,

	YouthBonusAge:     25,
	YouthBonusPerYear: 2.5,
	FreeAgentPool:     60,
}

// TrueCeiling is the hidden ceiling: Potential shifted by Impact.
func (p *Player) TrueCeiling() float64 {
	return float64(p.Potential) + float64(p.Impact)*Progression.CeilingImpactMult + Progression.CeilingOffset
}

func round(v float64) int { return int(math.Floor(v + 0.5)) }

// MinutesAdjust is the extra yearly rating change for a young player who
// played mpg minutes per team game (negative mpg = no statistics, no change).
func MinutesAdjust(age int, mpg float64) float64 {
	c := &Progression
	if mpg < 0 || age > c.MinutesDevMaxAge {
		return 0
	}
	if mpg >= c.MinutesDevNeutral {
		f := math.Min(1, (mpg-c.MinutesDevNeutral)/(c.MinutesDevFull-c.MinutesDevNeutral))
		return c.MinutesDevBonus * f
	}
	f := math.Min(1, (c.MinutesDevNeutral-mpg)/(c.MinutesDevNeutral-c.MinutesDevNone))
	return -c.MinutesDevPenalty * f
}

// DevChange is how a player's visible numbers moved in his latest yearly progression (new minus old).
type DevChange struct {
	Overall, Potential int
	Ratings            Ratings
}

// Progress ages the player one year with no minutes adjustment.
func (p *Player) Progress(r *RNG) { p.ProgressMinutes(r, -1) }

// ProgressMinutes ages the player one year: ratings, Potential, Age,
// Experience. mpg is his regular-season minutes per team game (negative if
// he has no statistics); young players develop a little faster with many
// minutes and slower with few (MinutesAdjust).
func (p *Player) ProgressMinutes(r *RNG, mpg float64) {
	c := &Progression
	oldOvr, oldPot, oldRt := p.Overall(), p.Potential, p.Ratings
	// In-season changes already made this season came ahead of the yearly
	// change: the formula starts from the season-start overall and the
	// offseason delta is reduced by what has already arrived.
	gain := 0.0
	var seasonDev DevChange
	if p.SeasonDev != nil {
		seasonDev = *p.SeasonDev
		gain = float64(seasonDev.Overall)
	}
	ovr := float64(p.Overall()) - gain
	delta := expectedTrend(p, ovr)
	switch {
	case p.Age < c.PrimeStartAge:
		delta += r.Normal(0, c.GrowthNoiseSD)
	case p.Age < c.DeclineStartAge:
		delta += r.Normal(0, c.PrimeNoiseSD)
	default:
		delta += r.Normal(0, c.DeclineNoiseSD)
	}
	if p.Age < c.LeapMaxAge {
		x := r.Float64()
		if x < c.LeapChance {
			delta += c.LeapMin + r.Float64()*(c.LeapMax-c.LeapMin)
		} else if x < c.LeapChance+c.StallChance {
			delta -= c.StallMin + r.Float64()*(c.StallMax-c.StallMin)
		}
	}
	if adj := MinutesAdjust(p.Age, mpg); adj > 0 {
		// Minutes alone never push a player past his true ceiling.
		adj = math.Min(adj, math.Max(0, p.TrueCeiling()-(ovr+delta)))
		delta += adj
	} else {
		delta += adj
	}
	delta -= gain
	apply := func(v int, idx int) int {
		d := ratingStep(p.Age, idx, delta+r.Normal(0, c.RatingNoiseSD))
		return clamp(v+round(d), 25, 99)
	}
	a := p.Ratings.array()
	for i := range a {
		a[i] = apply(a[i], i)
	}
	p.Ratings = Ratings{a[0], a[1], a[2], a[3], a[4], a[5], a[6]}

	p.Age++
	p.Experience++
	newOvr := p.Overall()
	if p.Age >= c.PotentialEqualAge {
		p.Potential = newOvr
	} else {
		pot := float64(p.Potential) + (p.TrueCeiling()-float64(p.Potential))*c.PotentialLearnRate
		p.Potential = clamp(max(round(pot), newOvr), 25, 99)
	}
	n, o := p.Ratings.array(), oldRt.array()
	var d [7]int
	for i := range d {
		d[i] = n[i] - o[i]
	}
	// The change shown is the whole year: season start to now.
	sd := seasonDev.Ratings.array()
	for i := range d {
		d[i] += sd[i]
	}
	p.Dev = &DevChange{Overall: newOvr - oldOvr + seasonDev.Overall, Potential: p.Potential - oldPot + seasonDev.Potential,
		Ratings: Ratings{d[0], d[1], d[2], d[3], d[4], d[5], d[6]}}
	p.SeasonDev = nil
}

// RetireChance is the probability the player retires this offseason.
func (p *Player) RetireChance() float64 {
	c := &Progression
	if p.Age >= c.RetireForceAge {
		return 1
	}
	if p.Age < c.RetireStartAge {
		return 0
	}
	pr := float64(p.Age-(c.RetireStartAge-1)) * c.RetireAgeStep
	if p.Overall() < c.RetireLowOverall {
		pr += c.RetireLowBonus
	}
	return pr
}

// GenerateRookie creates a draft prospect: low current talent, wide potential.
func GenerateRookie(r *RNG, id, year int) *Player {
	c := &Progression
	age := r.Between(c.RookieMinAge, c.RookieMaxAge)
	pos := Position(r.IntN(5))
	target := r.Normal(c.RookiePotMean, c.RookiePotSD) - float64(age-c.RookieMinAge)*c.RookiePotAgeDrop
	if r.Chance(c.RookieEliteChance) {
		target += r.Normal(c.RookieEliteBonus, c.RookieEliteSD)
	}
	if c.RookieStarEvery > 0 && id%c.RookieStarEvery == 0 {
		target = max(target, r.Normal(c.RookieStarMean, c.RookieStarSD))
	}
	// Better prospects are also somewhat better today.
	talent := c.RookieTalentMean + float64(age-c.RookieMinAge)*c.RookieTalentPerYr +
		(target-c.RookiePotMean)*c.RookieCurrentShare
	p := buildPlayer(r, id, pos, r.Normal(talent, 5), age)
	p.Potential = clamp(max(round(target), p.Overall()), 25, 99)
	p.DraftYear = year
	return p
}
