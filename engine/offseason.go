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
	Order   []int // original team IDs for one round, first pick first (each pick's owner selects)
	Made    int   // picks made so far
	Total   int
	Results []PickResult
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
	lg.FinishOffseason()
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
	before := map[int]int{}
	for _, p := range lg.allPlayers() {
		before[p.ID] = p.Overall()
		p.Progress(r)
	}

	// Retirement.
	keep := func(ps []*Player) []*Player {
		var out []*Player
		for _, p := range ps {
			if r.Chance(p.RetireChance()) {
				rep.Retirements = append(rep.Retirements, Retiree{p.ID, p.Name, p.Age, p.Overall(), p.TeamID})
				delete(before, p.ID)
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
	// Contracts: one season used up. Players now at 0 years are expiring; they
	// stay on the roster until FinishOffseason.
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			p.Contract.Years = max(0, p.Contract.Years-1)
		}
	}
	sort.SliceStable(rep.Retirements, func(i, j int) bool { return rep.Retirements[i].Overall > rep.Retirements[j].Overall })

	// Risers and fallers.
	var ch []Change
	for _, p := range lg.allPlayers() {
		ch = append(ch, Change{p.ID, p.TeamID, p.Name, p.Age, before[p.ID], p.Overall()})
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
	lg.Draft = &Draft{Pool: pool, Order: order, Total: c.DraftSize}
	lg.Report = rep
	return rep
}

func draftScore(p *Player) float64 {
	c := &Progression
	return c.DraftOvrWeight*float64(p.Overall()) + c.DraftPotWeight*float64(p.Potential)
}

// DraftDone reports whether every pick has been made (or no draft is running).
func (lg *League) DraftDone() bool { return lg.Draft == nil || lg.Draft.Made >= lg.Draft.Total }

// DraftOnClock returns the team ID about to pick.
func (lg *League) DraftOnClock() int {
	d := lg.Draft
	orig := d.Order[d.Made%len(d.Order)]
	round := d.Made/len(d.Order) + 1
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
	bi := -1
	if playerID < 0 {
		bi = 0
		for i, p := range d.Pool {
			if draftScore(p) > draftScore(d.Pool[bi]) {
				bi = i
			}
		}
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
	tid := lg.DraftOnClock()
	p := d.Pool[bi]
	d.Pool = append(d.Pool[:bi], d.Pool[bi+1:]...)
	d.Made++
	p.DraftPick = d.Made
	p.TeamID = tid
	round := (d.Made-1)/len(d.Order) + 1
	p.Contract = RookieContract(d.Made, round) // draft picks always sign (see CanSign)
	lg.Teams[tid].Roster = append(lg.Teams[tid].Roster, p)
	pr := PickResult{d.Made, (d.Made-1)/len(d.Order) + 1, tid, p.ID, p.Name, p.Age, p.Overall(), p.Potential}
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
func (lg *League) FinishOffseason() {
	c := &Progression
	if lg.FA == nil {
		// Callers that skip the free-agency screens: open it now.
		if lg.BeginFreeAgency() != nil {
			return
		}
	}
	for lg.FA.Day < lg.FA.Days {
		lg.AdvanceFreeAgency()
	}
	order := append([]int(nil), lg.Draft.Order...)
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

	lg.clearOffseasonInjuries()
	lg.Year++
	lg.SimAITrades(NewRNG(lg.Seed*1000003 + uint64(lg.Year)*10007 + tradeOffset))
}
