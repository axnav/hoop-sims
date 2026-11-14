package engine

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// FATuning holds every free-agency number in one place.
var FATuning = struct {
	Days            int     // length of the free-agency period, in days
	SoftenTo        float64 // asking salary on the last day, as a fraction of day 0
	AcceptPct       float64 // a player takes an offer only if salary >= ask * this
	OfferLow        float64 // computer offers are the ask times a factor in OfferLow..OfferHigh
	OfferHigh       float64
	YearBonus       float64 // appeal bonus per contract year beyond the first
	TeamSwing       float64 // team factor is 1 +- TeamSwing (contending teams are better)
	RoleBonus       float64 // appeal bonus if he would be among the team's best eight
	MaxNewOffers    int     // most new offers one computer team makes per day
	SecondOfferPct  float64 // chance that a team with room makes a second offer in a day
	MinOffersAfter  int     // computer teams start offering minimum deals from this day on
	WantMinValue    float64 // a computer team bids above the minimum only for players worth at least this to it
	CandidatesPerTm int     // how many top-valued free agents a team looks at per day
	UpgradeMargin   float64 // a full computer team bids for a free agent whose keep score beats its worst player's by this much, and cuts that player on signing
}{
	Days: 10, SoftenTo: 0.70, AcceptPct: 0.95,
	OfferLow: 0.92, OfferHigh: 1.12, YearBonus: 0.02, TeamSwing: 0.08, RoleBonus: 0.03,
	MaxNewOffers: 2, SecondOfferPct: 0.4, MinOffersAfter: 5,
	WantMinValue: 2.5, CandidatesPerTm: 14, UpgradeMargin: 4,
}

// freeAgencyOffset is the RNG stream for computer offers (20000 proposals,
// 30000 injuries, 40000 contracts), so game results never change.
const freeAgencyOffset = 50000

// FAOffer is one team's standing offer to a free agent. Day is the day it was made.
type FAOffer struct {
	ID       int
	Team     int
	PlayerID int
	Salary   int
	Years    int
	Day      int
}

// FASigning is one signing during free agency.
type FASigning struct {
	Day      int
	PlayerID int
	Name     string
	Team     int
	Salary   int
	Years    int
	Pos      string
	Overall  int
}

// FreeAgency is the saved state of the free-agency period (nil outside it).
type FreeAgency struct {
	Day         int // days already resolved
	Days        int // total days
	NextOfferID int
	Offers      []FAOffer
	Log         []FASigning
}

// DaysLeft is how many days remain.
func (fa *FreeAgency) DaysLeft() int { return max(0, fa.Days-fa.Day) }

// CanBeginFreeAgency says whether free agency may begin now (the draft is done).
func (lg *League) CanBeginFreeAgency(reportSeen bool) error {
	if lg.FA != nil {
		return errors.New("free agency has already begun")
	}
	if lg.Phase(reportSeen) != PhaseDraftDone {
		return errors.New("the draft is not finished")
	}
	return nil
}

// BeginFreeAgency ends the re-sign window: computer teams keep who they want
// (and are trimmed to 13), then every player still expiring, the user's too,
// is released to the pool and the free-agency period opens.
func (lg *League) BeginFreeAgency() error {
	if lg.FA != nil {
		return errors.New("free agency has already begun")
	}
	if lg.Draft == nil || !lg.DraftDone() {
		return errors.New("the draft is not finished")
	}
	lg.reSignComputerTeams()
	lg.releaseExpiring()
	for _, t := range lg.Teams {
		if t.ID != lg.UserTeamID {
			lg.trimRoster(t)
		}
	}
	lg.FA = &FreeAgency{Days: FATuning.Days}
	return nil
}

// trimRoster releases the lowest keepScore players above 13.
func (lg *League) trimRoster(t *Team) {
	sort.SliceStable(t.Roster, func(i, j int) bool { return keepScore(t.Roster[i]) > keepScore(t.Roster[j]) })
	if len(t.Roster) > RosterSize {
		for _, p := range t.Roster[RosterSize:] {
			lg.releaseToFA(p)
		}
		t.Roster = t.Roster[:RosterSize]
	}
}

// soften is the asking-salary multiplier on a day (1 on day 0, SoftenTo on the last).
func soften(day, days int) float64 {
	if days <= 1 {
		return 1
	}
	d := max(0, min(day, days-1))
	return 1 - (1-FATuning.SoftenTo)*float64(d)/float64(days-1)
}

// FreeAgentAsk is what a free agent asks today: his asking contract, with the
// salary softening as the days pass (never below the minimum).
func (lg *League) FreeAgentAsk(p *Player) (salary, years int) {
	salary, years = AskingContract(p)
	if lg.FA != nil {
		s := int(math.Round(float64(salary)*soften(lg.FA.Day, lg.FA.Days)/100)) * 100
		salary = max(Cap.MinSalary, min(salary, s))
	}
	return salary, years
}

func (lg *League) faPlayer(id int) *Player {
	for _, p := range lg.FreeAgents {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// pendingAbove is the money in the team's outstanding offers above the minimum,
// leaving out any offer for the given player (which a new offer replaces).
func (lg *League) pendingAbove(teamID, exceptPlayer int) int {
	n := 0
	for _, o := range lg.FA.Offers {
		if o.Team == teamID && o.PlayerID != exceptPlayer && o.Salary > Cap.MinSalary {
			n += o.Salary
		}
	}
	return n
}

// checkOfferMoney applies the cap rules to a possible offer.
func (lg *League) checkOfferMoney(teamID, playerID, salary int) error {
	if salary == Cap.MinSalary {
		return lg.CanSign(teamID, salary, SignMinimum)
	}
	if err := lg.CanSign(teamID, salary, SignCapRoom); err != nil {
		return err
	}
	if pend := lg.pendingAbove(teamID, playerID); pend > 0 {
		if lg.Payroll(teamID)+pend+salary > Cap.SalaryCap {
			room := max(0, lg.CapRoom(teamID)-pend)
			return fmt.Errorf("You have %s in cap room after your other offers; this offer needs %s.", fmtM(room), fmtM(salary))
		}
	}
	return nil
}

// MakeOffer puts a standing offer to a free agent. It replaces the team's
// earlier offer for the same player. It can be accepted from the next day on.
func (lg *League) MakeOffer(teamID, playerID, salary, years int) (FAOffer, error) {
	return lg.makeOffer(teamID, playerID, salary, years, false)
}

// makeOffer is MakeOffer; allowFull lets a full computer team bid for an
// upgrade (it cuts its worst player if the player signs).
func (lg *League) makeOffer(teamID, playerID, salary, years int, allowFull bool) (FAOffer, error) {
	if lg.FA == nil || lg.FA.Day >= lg.FA.Days {
		return FAOffer{}, errors.New("Free agency is not open.")
	}
	if teamID < 0 || teamID >= len(lg.Teams) {
		return FAOffer{}, errors.New("That is not a valid team.")
	}
	p := lg.faPlayer(playerID)
	if p == nil {
		return FAOffer{}, errors.New("That player is not a free agent.")
	}
	if years < 1 || years > Cap.MaxContractYears {
		return FAOffer{}, fmt.Errorf("A contract must run 1 to %d years.", Cap.MaxContractYears)
	}
	t := lg.Teams[teamID]
	if len(t.Roster) >= RosterSize && !allowFull {
		return FAOffer{}, fmt.Errorf("%s already have a full roster of %d; release someone first.", t.FullName(), RosterSize)
	}
	if err := lg.checkOfferMoney(teamID, playerID, salary); err != nil {
		return FAOffer{}, err
	}
	lg.dropOffer(teamID, playerID)
	lg.FA.NextOfferID++
	o := FAOffer{ID: lg.FA.NextOfferID, Team: teamID, PlayerID: playerID, Salary: salary, Years: years, Day: lg.FA.Day}
	lg.FA.Offers = append(lg.FA.Offers, o)
	return o, nil
}

func (lg *League) dropOffer(teamID, playerID int) {
	keep := lg.FA.Offers[:0]
	for _, o := range lg.FA.Offers {
		if !(o.Team == teamID && o.PlayerID == playerID) {
			keep = append(keep, o)
		}
	}
	lg.FA.Offers = keep
}

// WithdrawOffer cancels one of the team's offers.
func (lg *League) WithdrawOffer(teamID, offerID int) error {
	if lg.FA == nil {
		return errors.New("Free agency is not open.")
	}
	for i, o := range lg.FA.Offers {
		if o.ID == offerID && o.Team == teamID {
			lg.FA.Offers = append(lg.FA.Offers[:i], lg.FA.Offers[i+1:]...)
			return nil
		}
	}
	return errors.New("That offer does not exist.")
}

// OffersBy lists the team's outstanding offers.
func (lg *League) OffersBy(teamID int) []FAOffer {
	var out []FAOffer
	if lg.FA == nil {
		return out
	}
	for _, o := range lg.FA.Offers {
		if o.Team == teamID {
			out = append(out, o)
		}
	}
	return out
}

// OffersFor counts the offers out for a player (no details: rival bids stay hidden).
func (lg *League) OffersFor(playerID int) int {
	n := 0
	if lg.FA == nil {
		return 0
	}
	for _, o := range lg.FA.Offers {
		if o.PlayerID == playerID {
			n++
		}
	}
	return n
}

// sortedPool returns the free agents best overall first (ties by ID).
func (lg *League) sortedPool() []*Player {
	ps := append([]*Player(nil), lg.FreeAgents...)
	sort.SliceStable(ps, func(i, j int) bool {
		if ps[i].Overall() != ps[j].Overall() {
			return ps[i].Overall() > ps[j].Overall()
		}
		return ps[i].ID < ps[j].ID
	})
	return ps
}

// AdvanceFreeAgency resolves one day and returns that day's signings. Computer
// teams bid first, then players choose among offers made on earlier days.
// After the last day it returns an error.
func (lg *League) AdvanceFreeAgency() ([]FASigning, error) {
	fa := lg.FA
	if fa == nil {
		return nil, errors.New("Free agency is not open.")
	}
	if fa.Day >= fa.Days {
		return nil, errors.New("Free agency is over; start the new season.")
	}
	cf := lg.contendFactors()
	lg.computerOffers(cf)
	signed := lg.playersDecide(cf)
	fa.Day++
	return signed, nil
}

// computerOffers lets each computer team with open roster spots bid.
func (lg *League) computerOffers(cf []float64) {
	fa := lg.FA
	r := lg.dayRNG(lg.Year, freeAgencyOffset+fa.Day)
	pool := lg.sortedPool()
	if len(pool) == 0 {
		return
	}
	soft := soften(fa.Day, fa.Days)
	for _, t := range lg.Teams {
		if t.ID == lg.UserTeamID {
			continue
		}
		spots := RosterSize - len(t.Roster)
		upgrade := spots <= 0 // full roster: only a clear upgrade is worth a bid
		limit := spots + 1    // one backup offer in case a player turns it down or signs elsewhere
		worst := 0.0
		if upgrade {
			limit = 1
			worst = keepScore(lg.worstKeep(t))
		}
		if len(lg.OffersBy(t.ID)) >= limit {
			continue
		}
		type cand struct {
			p *Player
			v float64
		}
		cands := make([]cand, 0, len(pool))
		for _, p := range pool {
			cands = append(cands, cand{p, lg.value(p, t.ID, cf)})
		}
		sort.SliceStable(cands, func(i, j int) bool {
			if cands[i].v != cands[j].v {
				return cands[i].v > cands[j].v
			}
			return cands[i].p.ID < cands[j].p.ID
		})
		if len(cands) > FATuning.CandidatesPerTm {
			cands = cands[:FATuning.CandidatesPerTm]
		}
		want := 1
		if FATuning.MaxNewOffers > 1 && r.Chance(FATuning.SecondOfferPct) {
			want = FATuning.MaxNewOffers
		}
		made := 0
		for _, c := range cands {
			if made >= want || len(lg.OffersBy(t.ID)) >= limit {
				break
			}
			if c.v < FATuning.WantMinValue || (upgrade && keepScore(c.p) < worst+FATuning.UpgradeMargin) {
				continue
			}
			already := false
			for _, o := range lg.OffersBy(t.ID) {
				if o.PlayerID == c.p.ID {
					already = true
				}
			}
			if already {
				continue
			}
			ask, years := AskingContract(c.p)
			sal := int(math.Round(float64(ask)*soft*(FATuning.OfferLow+r.Float64()*(FATuning.OfferHigh-FATuning.OfferLow))/100)) * 100
			sal = max(Cap.MinSalary, min(Cap.MaxSalary, sal))
			years = clamp(years+r.Between(-1, 1)*b2i(r.Chance(0.3)), 1, Cap.MaxContractYears)
			if lg.checkOfferMoney(t.ID, c.p.ID, sal) != nil {
				continue
			}
			if _, err := lg.makeOffer(t.ID, c.p.ID, sal, years, upgrade); err == nil {
				made++
			}
		}
		// Late in the period, fill open spots with minimum offers to players
		// who would take one.
		if made == 0 && !upgrade && fa.Day >= FATuning.MinOffersAfter && len(lg.OffersBy(t.ID)) < limit {
			for _, c := range cands {
				if c.v < 1 {
					continue
				}
				ask, _ := AskingContract(c.p)
				if float64(Cap.MinSalary) < FATuning.AcceptPct*math.Max(float64(Cap.MinSalary), float64(ask)*soft) {
					continue
				}
				if lg.OffersBy(t.ID) != nil && hasOffer(lg.OffersBy(t.ID), c.p.ID) {
					continue
				}
				if _, err := lg.MakeOffer(t.ID, c.p.ID, Cap.MinSalary, 1); err == nil {
					break
				}
			}
		}
	}
}

func hasOffer(os []FAOffer, pid int) bool {
	for _, o := range os {
		if o.PlayerID == pid {
			return true
		}
	}
	return false
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// playersDecide lets every free agent with an offer made before today pick
// the most appealing acceptable one.
func (lg *League) playersDecide(cf []float64) []FASigning {
	fa := lg.FA
	var out []FASigning
	for _, p := range lg.sortedPool() {
		var offs []FAOffer
		for _, o := range fa.Offers {
			if o.PlayerID == p.ID && o.Day < fa.Day {
				offs = append(offs, o)
			}
		}
		if len(offs) == 0 {
			continue
		}
		ask, _ := lg.FreeAgentAsk(p)
		need := FATuning.AcceptPct * float64(ask)
		appeal := func(o FAOffer) float64 {
			a := float64(o.Salary) * (1 + FATuning.YearBonus*float64(o.Years-1))
			a *= 1 - FATuning.TeamSwing + 2*FATuning.TeamSwing*cf[o.Team]
			better := 0
			for _, q := range lg.Teams[o.Team].Roster {
				if q.Overall() > p.Overall() {
					better++
				}
			}
			if better < 8 {
				a *= 1 + FATuning.RoleBonus
			}
			return a
		}
		sort.SliceStable(offs, func(i, j int) bool {
			ai, aj := appeal(offs[i]), appeal(offs[j])
			if ai != aj {
				return ai > aj
			}
			return offs[i].Team < offs[j].Team
		})
		for _, o := range offs {
			if float64(o.Salary) < need {
				continue
			}
			kind := SignCapRoom
			if o.Salary == Cap.MinSalary {
				kind = SignMinimum
			}
			var cut *Player
			var cutContract Contract
			if tm := lg.Teams[o.Team]; len(tm.Roster) >= RosterSize && o.Team != lg.UserTeamID {
				// full computer team: the signing is an upgrade, cut its worst player
				if w := lg.worstKeep(tm); keepScore(p) >= keepScore(w)+FATuning.UpgradeMargin {
					cut, cutContract = w, w.Contract
					for i, q := range tm.Roster {
						if q == w {
							tm.Roster = append(tm.Roster[:i], tm.Roster[i+1:]...)
							break
						}
					}
					lg.releaseToFA(w)
				}
			}
			if err := lg.Sign(o.Team, p, o.Salary, o.Years, kind); err != nil {
				if cut != nil { // undo the cut
					lg.FreeAgents = lg.FreeAgents[:len(lg.FreeAgents)-1]
					cut.Contract, cut.TeamID = cutContract, o.Team
					lg.Teams[o.Team].Roster = append(lg.Teams[o.Team].Roster, cut)
				}
				lg.dropOffer(o.Team, p.ID)
				continue
			}
			keep := fa.Offers[:0]
			for _, x := range fa.Offers {
				if x.PlayerID != p.ID {
					keep = append(keep, x)
				}
			}
			fa.Offers = keep
			s := FASigning{Day: fa.Day, PlayerID: p.ID, Name: p.Name, Team: o.Team, Salary: o.Salary, Years: o.Years, Pos: p.Pos.String(), Overall: p.Overall()}
			fa.Log = append(fa.Log, s)
			out = append(out, s)
			break
		}
	}
	return out
}

// worstKeep is the team's player with the lowest keep score (ties: higher ID).
func (lg *League) worstKeep(t *Team) *Player {
	var w *Player
	for _, p := range t.Roster {
		if w == nil || keepScore(p) < keepScore(w) || (keepScore(p) == keepScore(w) && p.ID > w.ID) {
			w = p
		}
	}
	return w
}

// fillRosters is the end-of-period fill: every team short of 13 signs the best
// remaining free agents to one-year minimum deals (users included).
func (lg *League) fillRosters(order []int) {
	if len(order) != len(lg.Teams) {
		order = order[:0]
		for i := range lg.Teams {
			order = append(order, i)
		}
	}
	for _, tid := range order {
		t := lg.Teams[tid]
		for len(t.Roster) < RosterSize && len(lg.FreeAgents) > 0 {
			p := lg.sortedPool()[0]
			if lg.Sign(tid, p, Cap.MinSalary, 1, SignMinimum) != nil {
				lg.forceSign(tid, p, Cap.MinSalary, 1)
			}
		}
	}
}
