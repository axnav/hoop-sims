package engine

import (
	"fmt"
	"sort"
)

type Retiree struct {
	ID      int
	Name    string
	Age     int
	Overall int
	TeamID  int
}

type PickResult struct {
	Pick, Round, TeamID, PlayerID int
	Name                          string
	Age, Overall, Potential       int
}

type Change struct {
	PlayerID, TeamID    int
	Name                string
	Age, OldOvr, NewOvr int
}

type OffseasonReport struct {
	Year        int
	Retirements []Retiree // sorted by overall, best first
	Draft       []PickResult
	Risers      []Change
	Fallers     []Change
}

// Draft is a draft in progress. Picks are made one at a time.
type Draft struct {
	Pool    []*Player
	Order   []int // round 1: original team IDs, first pick first (lottery applied; each pick's owner selects)
	Order2  []int // round 2: original team IDs, worst record first (no lottery)
	Made    int   // picks made so far
	Total   int
	Results []PickResult

	// Lottery is the first-round lottery outcome, for the screens.
	Lottery []LotteryRow

	// Scouting (see scouting.go): fixed per-prospect errors in [-1, 1], the
	// user's points left and each prospect's scouting level (0..3).
	OvrErr      map[int]float64
	PotErr      map[int]float64
	ScoutPoints int
	ScoutLevel  map[int]int
}

func (lg *League) allPlayers() []*Player {
	var out []*Player
	for _, t := range lg.Teams {
		out = append(out, t.Roster...)
	}
	return append(out, lg.FreeAgents...)
}

func keepScore(p *Player) float64 {
	return float64(p.Overall()) + float64(max(0, Progression.YouthBonusAge-p.Age))*Progression.YouthBonusPerYear
}

// Offseason runs the whole offseason with the computer making every pick.
// The season must be finished (regular season). It is an autopilot: the
// user's team re-signs its expiring players like a computer team would, so a
// hands-off team is not stripped.
func (lg *League) Offseason() *OffseasonReport {
	rep := lg.BeginOffseason()
	lg.DraftAutoAll()
	lg.reSignTeam(lg.Teams[lg.UserTeamID], lg.contendFactors())
	lg.finishOffseason(true)
	return rep
}

// BeginOffseason records career history, progresses and retires players and
// sets up the draft (lg.Draft). Make picks with DraftPick / DraftAuto*, then
// call FinishOffseason.
func (lg *League) BeginOffseason() *OffseasonReport {
	c := &Progression
	s := lg.Season
	rep := &OffseasonReport{Year: lg.Year}
	r := lg.dayRNG(lg.Year, offseasonOffset)
	lg.recordSeason()
	lg.recordTax()

	// Progression.
	mentors := lg.mentorTeams()
	for _, p := range lg.allPlayers() {
		mpg := -1.0
		if ps := lg.Season.statsFor(p.ID); ps != nil {
			mpg = float64(ps.Min) / SeasonGames
		}
		p.ProgressMentored(r, mpg, lg.growthWeights(p), lg.growthMult(p, mentors)*lg.devMult(p.TeamID))
	}

	// Retirement.
	keep := func(ps []*Player) []*Player {
		var out []*Player
		for _, p := range ps {
			if r.Chance(p.RetireChance()) {
				rep.Retirements = append(rep.Retirements, Retiree{p.ID, p.Name, p.Age, p.Overall(), p.TeamID})
				lg.Retired = append(lg.Retired, p)
				continue
			}
			out = append(out, p)
		}
		return out
	}
	for _, t := range lg.Teams {
		t.Roster = keep(t.Roster)
	}
	lg.FreeAgents = keep(lg.FreeAgents)
	lg.ageDeadMoney()
	// Contracts: one season used up. Players now at 0 years are expiring; they
	// stay on the roster until FinishOffseason.
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			if p.Contract.Years <= 1 && p.Contract.NextYears > 0 {
				// an extension takes over instead of the player expiring
				p.Contract = Contract{Salary: p.Contract.NextSalary, Years: p.Contract.NextYears}
				continue
			}
			p.Contract.Years = max(0, p.Contract.Years-1)
		}
	}
	sort.SliceStable(rep.Retirements, func(i, j int) bool { return rep.Retirements[i].Overall > rep.Retirements[j].Overall })

	// Risers and fallers.
	var ch []Change
	for _, p := range lg.allPlayers() {
		// Whole-year change: in-season steps plus the offseason step.
		ch = append(ch, Change{p.ID, p.TeamID, p.Name, p.Age, p.Overall() - p.Dev.Overall, p.Overall()})
	}
	sort.SliceStable(ch, func(i, j int) bool {
		di, dj := ch[i].NewOvr-ch[i].OldOvr, ch[j].NewOvr-ch[j].OldOvr
		if di != dj {
			return di > dj
		}
		return ch[i].PlayerID < ch[j].PlayerID
	})
	rep.Risers = append(rep.Risers, ch[:5]...)
	for i := 0; i < 5; i++ {
		rep.Fallers = append(rep.Fallers, ch[len(ch)-1-i])
	}

	// Draft: worst record picks first.
	pool := make([]*Player, c.DraftSize)
	for i := range pool {
		pool[i] = GenerateRookie(r, lg.NextID, lg.Year)
		lg.NextID++
	}
	if cls := lg.takeDraftClass(lg.Year); cls != nil {
		pool = cls
	}
	order := s.Standings()
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}
	o1, lot := lg.lotteryOrder(lg.dayRNG(lg.Year, lotteryOffset), order, s.MadePlayoffs)
	lg.Draft = &Draft{Pool: pool, Order: o1, Order2: order, Lottery: lot, Total: c.DraftSize}
	lg.initScouting(lg.Draft)
	lg.Report = rep
	lg.openCoachPool()
	return rep
}

func draftScore(p *Player) float64 {
	c := &Progression
	return c.DraftOvrWeight*float64(p.Overall()) + c.DraftPotWeight*float64(p.Potential)
}

// DraftDone reports whether every pick has been made (or no draft is running).
func (lg *League) DraftDone() bool { return lg.Draft == nil || lg.Draft.Made >= lg.Draft.Total }

// slot returns the original team and round of the i-th pick (0-based).
func (d *Draft) slot(i int) (orig, round int) {
	n := len(d.Order)
	if i < n {
		return d.Order[i], 1
	}
	o := d.Order2
	if len(o) != n {
		o = d.Order
	}
	return o[(i-n)%n], 2
}

// DraftOnClock returns the team ID about to pick.
func (lg *League) DraftOnClock() int {
	d := lg.Draft
	lg.ensureDraft()
	orig, round := d.slot(d.Made)
	// Order gives the slot (by original team); the pick's owner selects.
	if owner, ok := lg.PickOwner(lg.Year, round, orig); ok {
		return owner
	}
	return orig
}

// DraftPick makes the on-the-clock team's pick: the given player, or the
// computer's best choice if playerID < 0.
func (lg *League) DraftPick(playerID int) error {
	d := lg.Draft
	if lg.DraftDone() {
		return fmt.Errorf("draft is finished")
	}
	lg.ensureDraft()
	tid := lg.DraftOnClock()
	bi := -1
	if playerID < 0 {
		bi = lg.autoChoice(tid)
	} else {
		for i, p := range d.Pool {
			if p.ID == playerID {
				bi = i
			}
		}
		if bi < 0 {
			return fmt.Errorf("player %d is not available", playerID)
		}
	}
	p := d.Pool[bi]
	d.Pool = append(d.Pool[:bi], d.Pool[bi+1:]...)
	d.Made++
	p.DraftPick = d.Made
	p.TeamID = tid
	p.TeamSince = lg.Year
	_, round := d.slot(d.Made - 1)
	p.Contract = RookieContract(d.Made, round) // draft picks always sign (see CanSign)
	lg.Teams[tid].Roster = append(lg.Teams[tid].Roster, p)
	pr := PickResult{d.Made, round, tid, p.ID, p.Name, p.Age, p.Overall(), p.Potential}
	d.Results = append(d.Results, pr)
	lg.Report.Draft = append(lg.Report.Draft, pr)
	return nil
}

// DraftAutoUntilUser lets the computer pick until the user's team is on the
// clock (or the draft ends).
func (lg *League) DraftAutoUntilUser() {
	for !lg.DraftDone() && lg.DraftOnClock() != lg.UserTeamID {
		lg.DraftPick(-1)
	}
}

// DraftAutoAll lets the computer make all remaining picks, including the user's.
func (lg *League) DraftAutoAll() {
	for !lg.DraftDone() {
		lg.DraftPick(-1)
	}
}

// FinishOffseason trims rosters, fills them from free agency and advances the
// year. The draft must be finished.
//
// Offer sheets against the user that are still unanswered are declined (a
// user who never answers has not matched).
func (lg *League) FinishOffseason() { lg.finishOffseason(false) }

// finishOffseason is FinishOffseason; matchUser answers the user's pending
// offer sheets with the computer-team match rule instead (the fully
// automatic Offseason).
func (lg *League) finishOffseason(matchUser bool) {
	c := &Progression
	if lg.FA == nil {
		// Callers that skip the free-agency screens: open it now.
		if lg.BeginFreeAgency() != nil {
			return
		}
	}
	// No screens here: offer sheets against the user are answered at once so
	// this can never wait on the user.
	lg.autoAnswerSheets(lg.contendFactors(), matchUser)
	for lg.FA.Day < lg.FA.Days {
		lg.advanceDay(true, matchUser)
	}
	order := append([]int(nil), lg.Draft.Order2...)
	if len(order) == 0 {
		order = append(order, lg.Draft.Order...)
	}
	lg.Draft = nil
	lg.FA = nil
	lg.rollPicks(lg.Year)

	// Rosters: trim to 13, then fill from free agents with minimum deals.
	for _, t := range lg.Teams {
		lg.trimRoster(t)
	}
	lg.fillRosters(order)
	lg.FreeAgents = lg.sortedPool()
	if len(lg.FreeAgents) > c.FreeAgentPool {
		lg.FreeAgents = lg.FreeAgents[:c.FreeAgentPool]
	}

	lg.autoFocusTeams()
	lg.finishCoaches()
	lg.moraleOffseason()
	lg.clearOffseasonInjuries()
	lg.Year++
	lg.SimAITrades(NewRNG(lg.Seed*1000003 + uint64(lg.Year)*10007 + tradeOffset))
	lg.moveRequestersOffseason()
}
