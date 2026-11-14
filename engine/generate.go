package engine

import "sort"

const RosterSize = 13

// posBias shifts each rating by position so guards and bigs feel different.
// Order: Inside, Mid, Three, Playmaking, Rebounding, Defense, Athleticism.
var posBias = map[Position][7]int{
	PG: {-6, 3, 5, 12, -12, -2, 3},
	SG: {-3, 5, 7, 4, -8, 0, 3},
	SF: {0, 2, 2, 0, -2, 2, 3},
	PF: {6, -1, -5, -6, 8, 3, 0},
	C:  {10, -5, -12, -9, 13, 5, -3},
}

// buildPlayer creates a player with the given current talent and age.
func buildPlayer(r *RNG, id int, pos Position, talent float64, age int) *Player {
	b := posBias[pos]
	rate := func(i int) int {
		return clamp(int(talent+float64(b[i])+r.Normal(0, 6)), 25, 99)
	}
	p := &Player{
		ID:   id,
		Name: Pick(r, firstNames) + " " + Pick(r, lastNames),
		Age:  age,
		Pos:  pos,
		Ratings: Ratings{
			Inside: rate(0), Mid: rate(1), Three: rate(2), Playmaking: rate(3),
			Rebounding: rate(4), Defense: rate(5), Athleticism: rate(6),
		},
		TeamID: -1,
		Impact: clamp(int(r.Normal(0, Progression.ImpactSD)), -8, 8),
	}
	// A player's offensive role varies independently from raw skill. The
	// ratings provide a role-shaped average; noise creates pass-first and
	// score-first players with otherwise similar abilities.
	p.UsageTendency = clamp(usageFromRatings(p.Ratings, pos)+usageVariation(id, p.Name), 25, 90)
	// About half of players can also play a neighbouring position.
	if r.Chance(0.45) {
		alt := pos + Position(r.Between(0, 1)*2-1)
		if alt >= PG && alt <= C {
			p.AltPos = []Position{alt}
		}
	}
	return p
}

// GeneratePlayer creates a random player. talentMean is the mean of the
// player's peak talent (roughly 60 for an established league player). Young
// players start below their peak (and have potential to reach it); players
// past 30 start below it because of decline, so a fresh league already looks
// like a steady-state one.
func GeneratePlayer(r *RNG, id int, pos Position, talentMean float64) *Player {
	peak := r.Normal(talentMean, 8)
	if r.Chance(Progression.InitialEliteChance) {
		peak += r.Normal(Progression.RookieEliteBonus, Progression.RookieEliteSD)
	}
	age := r.Between(19, 35)
	room := 0
	if yearsLeft := 28 - age; yearsLeft > 0 {
		room = r.Between(0, yearsLeft*3)
	}
	decline := 0.0
	if age > 30 {
		decline = 0.3 * float64((age-30)*(age-29))
	}
	p := buildPlayer(r, id, pos, peak-float64(room)-decline, age)
	if ex := p.Overall() - Progression.YoungOverallCap; age < 21 && ex > 0 {
		r := p.Ratings.array()
		for i := range r {
			r[i] = clamp(r[i]-ex, 25, 99)
		}
		p.Ratings = Ratings{r[0], r[1], r[2], r[3], r[4], r[5], r[6]}
	}
	p.Potential = clamp(p.Overall()+room, 25, 99)
	p.Experience = max(0, age-19)
	return p
}

type Team struct {
	ID     int
	City   string
	Name   string
	Roster []*Player
	// TaxPaid is the luxury tax the team paid, by season year (contract.go).
	TaxPaid map[int]int `json:",omitempty"`
}

func (t *Team) FullName() string { return t.City + " " + t.Name }

// Rotation returns the roster in playing order: five starters, one per
// position slot (each filled by the best unused player who can play it),
// then the bench sorted best-first by overall.
func (t *Team) Rotation() []*Player {
	// Injured players sit out; with fewer than five healthy players
	// everyone is included so a game can always be played.
	pool := make([]*Player, 0, len(t.Roster))
	for _, p := range t.Roster {
		if !p.Injured() {
			pool = append(pool, p)
		}
	}
	if len(pool) < 5 {
		pool = t.Roster
	}
	used := map[*Player]bool{}
	var starters []*Player
	for pos := PG; pos <= C; pos++ {
		var best *Player
		for _, p := range pool {
			if !used[p] && p.CanPlay(pos) && (best == nil || p.OverallAt(pos) > best.OverallAt(pos)) {
				best = p
			}
		}
		if best != nil {
			used[best] = true
			starters = append(starters, best)
		}
	}
	sort.SliceStable(starters, func(i, j int) bool { return starters[i].Overall() > starters[j].Overall() })
	var bench []*Player
	for _, p := range pool {
		if !used[p] {
			bench = append(bench, p)
		}
	}
	sort.SliceStable(bench, func(i, j int) bool { return bench[i].Overall() > bench[j].Overall() })
	return append(starters, bench...)
}

type League struct {
	// Version is the save format version, stamped by Save (see SaveVersion).
	Version int `json:"version"`

	Seed       uint64
	Year       int
	NextID     int // next unused player ID
	Teams      []*Team
	FreeAgents []*Player
	Season     *Season

	UserTeamID int
	Retired    []*Player // retired players, kept so their history stays viewable
	// Careers holds one row per player per finished season, by player ID.
	Careers map[int][]CareerRow
	// Records has one entry per finished season (champion, awards, bracket).
	Records []*SeasonRecord
	// LastBox is the user team's most recent game.
	LastBox *Box
	// Draft is non-nil while a draft is in progress; Report is the current
	// offseason's report.
	Draft  *Draft
	Report *OffseasonReport
	// FA is non-nil during the free-agency period (after the draft).
	FA *FreeAgency

	// DraftClasses holds stored future rookies by draft year (real-data
	// mode); a year with a stored class uses it instead of generating one.
	DraftClasses map[int][]*Player `json:",omitempty"`

	// Picks holds every draft pick in the PickYearsAhead window, with its
	// current owner (see DraftPick).
	Picks []DraftPick

	// Transactions is the log of executed trades, oldest first.
	Transactions []TradeResult `json:",omitempty"`

	// Proposals are trade offers from computer teams waiting for the user
	// (negotiate.go). NextProposalID numbers them. DeadlineStopYear is the
	// season in which the deadline pause already fired. All zero in old saves.
	Proposals        []Proposal `json:",omitempty"`
	NextProposalID   int        `json:",omitempty"`
	DeadlineStopYear int        `json:",omitempty"`
	Options          Options

	// newProposals counts proposals created on the most recent SimDay (not saved).
	newProposals int
	// newInjuries lists injuries from the most recent day (not saved).
	newInjuries []InjuryLine
}

// GenerateLeague builds 30 teams of 13 players. Each roster gets a balanced
// mix of positions: two of each, plus three wildcards.
func GenerateLeague(seed uint64) *League {
	r := NewRNG(seed)
	lg := &League{Seed: seed, Year: 2025, Careers: map[int][]CareerRow{}}
	id := 0
	for i, tn := range teamNames {
		t := &Team{ID: i, City: tn[0], Name: tn[1]}
		for slot := 0; slot < RosterSize; slot++ {
			pos := Position(slot % 5)
			if slot >= 10 {
				pos = Position(r.IntN(5))
			}
			p := GeneratePlayer(r, id, pos, Progression.InitialPeakMean)
			p.TeamID = t.ID
			t.Roster = append(t.Roster, p)
			id++
		}
		lg.Teams = append(lg.Teams, t)
	}
	lg.NextID = id
	lg.dealEvenly()
	lg.initPicks()
	lg.assignInitialContracts()
	return lg
}

// dealEvenly redistributes the generated players so no team starts with all
// the stars. Slot by slot, the 30 players are ranked by true overall and the
// best goes to the team with the least talent so far.
func (lg *League) dealEvenly() {
	total := make([]float64, NumTeams)
	for slot := 0; slot < RosterSize; slot++ {
		ps := make([]*Player, NumTeams)
		for i, t := range lg.Teams {
			ps[i] = t.Roster[slot]
		}
		sort.SliceStable(ps, func(a, b int) bool {
			if x, y := ps[a].TrueOverall(), ps[b].TrueOverall(); x != y {
				return x > y
			}
			return ps[a].ID < ps[b].ID
		})
		teams := make([]int, NumTeams)
		for i := range teams {
			teams[i] = i
		}
		sort.SliceStable(teams, func(a, b int) bool { return total[teams[a]] < total[teams[b]] })
		for rank, p := range ps {
			tid := teams[rank]
			total[tid] += float64(p.TrueOverall())
			p.TeamID = tid
			lg.Teams[tid].Roster[slot] = p
		}
	}
}
