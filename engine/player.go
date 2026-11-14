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
	TeamID    int

	Experience int // seasons played
	DraftYear  int // 0 for the initial league
	DraftPick  int // overall pick number, 0 if undrafted

	// RealID is the real-data player ID (empty in generated leagues).
	RealID string `json:",omitempty"`

	// Impact is the hidden number: how much better or worse the player
	// really performs than the visible ratings suggest (roughly -8 to +8).
	// The game sim uses it; the interface never shows it directly.
	Impact int
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

// TrueOverall is the hidden overall: what the player is really worth.
func (p *Player) TrueOverall() int {
	best := 0
	for _, pos := range p.Positions() {
		best = max(best, overallAt(p.Effective(), pos))
	}
	return best
}
