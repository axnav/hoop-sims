package engine

type Position int

const (
	PG Position = iota
	SG
	SF
	PF
	C
)

func (p Position) String() string {
	return [...]string{"PG", "SG", "SF", "PF", "C"}[p]
}

// Ratings are on a 25-99 scale.
type Ratings struct {
	Inside      int // finishing at the rim, post scoring
	Mid         int // mid-range and free throws
	Three       int // three-point shooting
	Playmaking  int // passing and ball handling
	Rebounding  int
	Defense     int
	Athleticism int
}

func (r Ratings) array() [7]int {
	return [7]int{r.Inside, r.Mid, r.Three, r.Playmaking, r.Rebounding, r.Defense, r.Athleticism}
}

type Player struct {
	ID        int
	Name      string
	Age       int
	Pos       Position   // primary position
	AltPos    []Position // other positions the player can also play
	Ratings   Ratings    // the visible ratings everyone sees
	Potential int        // the overall this player can grow into
	// UsageTendency is this player's appetite for taking shots, on a 25-90
	// scale. It is separate from shooting skill and playmaking. A zero value
	// means an older save or imported player should use a derived default.
	UsageTendency int `json:",omitempty"`
	TeamID        int

	Experience int // seasons played
	DraftYear  int // 0 for the initial league
	DraftPick  int // overall pick number, 0 if undrafted

	// RealID is the real-data player ID (empty in generated leagues).
	RealID string `json:",omitempty"`

	// Impact is the hidden number: how much better or worse the player
	// really performs than the visible ratings suggest (roughly -8 to +8).
	// The game sim uses it; the interface never shows it directly.
	Impact int

	// Contract is the player's deal (see contract.go); zero for free agents.
	Contract Contract

	// TeamSince is the league year he joined his current team (draft, signing
	// or trade); 0 = unknown, treated as neutral (see motivation.go).
	TeamSince int `json:",omitempty"`

	// Focus is the training focus (see training.go); zero = Balanced.
	Focus Focus `json:",omitempty"`

	// PerfBadges are the badges lifted one tier by last season's statistics
	// (key to 1, see badges.go); nil = none (or an old save).
	PerfBadges map[string]int `json:",omitempty"`

	// Injury is the current injury; nil means healthy (see injury.go).
	Injury *Injury `json:",omitempty"`

	// Dev is the change in the latest yearly progression; nil = never
	// progressed (or an old save).
	Dev *DevChange `json:",omitempty"`

	// SeasonDev is the in-season change so far this season (new minus
	// season-start values, see inseason.go); nil = none yet (or an old save).
	SeasonDev *DevChange `json:",omitempty"`
}

// Positions lists every position the player can play, primary first.
func (p *Player) Positions() []Position {
	return append([]Position{p.Pos}, p.AltPos...)
}

// PosLabel is the display form, e.g. "PG/SG".
func (p *Player) PosLabel() string {
	s := p.Pos.String()
	for _, a := range p.AltPos {
		s += "/" + a.String()
	}
	return s
}

func (p *Player) CanPlay(pos Position) bool {
	for _, q := range p.Positions() {
		if q == pos {
			return true
		}
	}
	return false
}

// overallWeights says how much each rating counts at each position.
// Order: Inside, Mid, Three, Playmaking, Rebounding, Defense, Athleticism.
// Each row sums to 1.
var overallWeights = map[Position][7]float64{
	PG: {.08, .14, .20, .30, .03, .13, .12},
	SG: {.10, .17, .25, .15, .04, .15, .14},
	SF: {.15, .14, .17, .10, .10, .18, .16},
	PF: {.22, .10, .08, .06, .22, .19, .13},
	C:  {.27, .06, .03, .05, .27, .22, .10},
}

func overallAt(r Ratings, pos Position) int {
	sum := 0.0
	for i, v := range r.array() {
		sum += float64(v) * overallWeights[pos][i]
	}
	return clamp(int(sum+0.5), 25, 99)
}

// OverallAt is the visible overall if the player lines up at pos.
func (p *Player) OverallAt(pos Position) int { return overallAt(p.Ratings, pos) }

// Overall is the visible overall: the best across the player's positions.
func (p *Player) Overall() int {
	best := 0
	for _, pos := range p.Positions() {
		best = max(best, p.OverallAt(pos))
	}
	return best
}

// Effective returns the ratings the game sim should use: the visible
// ratings shifted by the hidden Impact.
func (p *Player) Effective() Ratings {
	adj := func(v int) int { return clamp(v+p.Impact, 25, 99) }
	r := p.Ratings
	return Ratings{adj(r.Inside), adj(r.Mid), adj(r.Three), adj(r.Playmaking),
		adj(r.Rebounding), adj(r.Defense), adj(r.Athleticism)}
}

// ShotUsage returns the player's shot-taking tendency. Older saves and
// imported players without this field get a role estimate from their skills.
func (p *Player) ShotUsage() int {
	if p.UsageTendency > 0 {
		return clamp(p.UsageTendency, 25, 90)
	}
	return usageFromRatings(p.Ratings, p.Pos)
}

// ScoringRole gives the numeric shot tendency a plain-language label.
func (p *Player) ScoringRole() string {
	switch {
	case p.ShotUsage() < 42:
		return "Facilitator"
	case p.ShotUsage() >= 63:
		return "Primary scorer"
	default:
		return "Balanced scorer"
	}
}

func usageFromRatings(rt Ratings, pos Position) int {
	top := max(rt.Inside, rt.Mid, rt.Three)
	second := rt.Inside + rt.Mid + rt.Three - top - min(rt.Inside, rt.Mid, rt.Three)
	shooting := (top + second) / 2
	positionBias := [...]int{-3, 4, 2, 0, -2}
	if pos < PG || pos > C {
		pos = SF
	}
	return clamp(50+round(float64(shooting-rt.Playmaking)*0.45)+positionBias[pos], 25, 90)
}

// usageVariation provides persistent role variety without consuming extra
// seeded RNG values, which would otherwise reshuffle every generated league.
func usageVariation(id int, name string) int {
	h := uint64(id + 1)
	for i := 0; i < len(name); i++ {
		h = (h ^ uint64(name[i])) * 1099511628211
	}
	x := h
	x ^= x >> 16
	x *= 0x7feb352d
	x ^= x >> 15
	return int(x%17) - 8
}

// TrueOverall is the hidden overall: what the player is really worth.
func (p *Player) TrueOverall() int {
	best := 0
	for _, pos := range p.Positions() {
		best = max(best, overallAt(p.Effective(), pos))
	}
	return best
}
