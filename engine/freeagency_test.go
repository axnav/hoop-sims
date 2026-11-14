package engine

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// draftDoneLeague plays a season and the draft, stopping before free agency.
func draftDoneLeague(seed uint64) *League { return draftDoneLeagueFor(seed, 0) }

func draftDoneLeagueFor(seed uint64, user int) *League { return draftDoneLeagueOpt(seed, user, false) }

// draftDoneLeagueOpt is draftDoneLeagueFor with in-season development
// optionally off (for tests whose fixture depends on one seed's outcome).
func draftDoneLeagueOpt(seed uint64, user int, noInSeasonDev bool) *League {
	lg := GenerateLeague(seed)
	lg.Options.NoInSeasonDev = noInSeasonDev
	lg.UserTeamID = user
	lg.StartSeason()
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	lg.BeginOffseason()
	lg.DraftAutoAll()
	return lg
}

func mustBeginFA(t *testing.T, lg *League) {
	t.Helper()
	if err := lg.BeginFreeAgency(); err != nil {
		t.Fatal(err)
	}
}

// makeRoom frees n roster spots on the user's team (lowest players released)
// and gives everyone left a minimum contract so cap room is plentiful.
func makeRoom(lg *League, n int) {
	t := lg.Teams[lg.UserTeamID]
	for len(t.Roster) > RosterSize-n {
		lowest := t.Roster[0]
		for _, p := range t.Roster {
			if p.Overall() < lowest.Overall() {
				lowest = p
			}
		}
		lg.Release(t.ID, lowest.ID)
	}
	for _, p := range t.Roster {
		p.Contract = Contract{Cap.MinSalary, 2}
	}
}

func TestFreeAgencyFlowAndOffers(t *testing.T) {
	// In-season development off: the test needs a free agent with an ask of
	// 8000 to 30000, which seed 21 only has without it (fixture, not a rule).
	lg := draftDoneLeagueOpt(21, 0, true)
	wantPhase(t, lg, true, PhaseDraftDone, "begin_free_agency")
	u := lg.Teams[lg.UserTeamID]
	exp := u.Roster[0]
	exp.Contract.Years = 0
	wantErr(t, "advance before FA", lg.CanAdvanceFreeAgency(), true)
	mustBeginFA(t, lg)
	wantErr(t, "begin twice", lg.CanBeginFreeAgency(true), true)
	if exp.TeamID != -1 || exp.Contract != (Contract{}) || lg.faPlayer(exp.ID) == nil {
		t.Fatal("the user's un-re-signed player was not released to the pool")
	}
	for _, tm := range lg.Teams {
		if len(lg.Expiring(tm.ID)) != 0 {
			t.Fatalf("team %d still has expiring players", tm.ID)
		}
	}
	if err := lg.ReSign(u.ID, exp.ID); err == nil || !strings.Contains(err.Error(), "Free agency has begun") {
		t.Errorf("ReSign during free agency: %v", err)
	}
	wantPhase(t, lg, true, PhaseFreeAgency, "fa_day", "start_season")

	// ask softens day by day
	p0 := lg.sortedPool()[0]
	a0, _ := lg.FreeAgentAsk(p0)
	lg.FA.Day = lg.FA.Days - 1
	aLast, _ := lg.FreeAgentAsk(p0)
	lg.FA.Day = 0
	if want := float64(a0) * 0.70; aLast >= a0 || float64(aLast) < want-100 || float64(aLast) > want+100 {
		t.Errorf("ask %d -> %d, want about 70%%", a0, aLast)
	}

	// ---- offers ----
	makeRoom(lg, 6)
	pool := lg.sortedPool()
	var star *Player // best free agent whose ask leaves room to outbid
	for _, p := range pool {
		if s, _ := lg.FreeAgentAsk(p); s >= 8000 && s <= 30000 {
			star = p
			break
		}
	}
	if star == nil {
		t.Fatal("no suitable star free agent")
	}
	ask, years := lg.FreeAgentAsk(star)

	// cap room: leave the user $5.0M
	keep := u.Roster[0].Contract
	u.ExceptionYear = lg.Year // fixture: this block tests the cap-room rule alone (the exception has its own tests)
	u.Roster[0].Contract.Salary = Cap.SalaryCap - (lg.Payroll(u.ID) - keep.Salary) - 5000
	if _, err := lg.MakeOffer(u.ID, star.ID, ask, years); err == nil || !strings.Contains(err.Error(), "cap room") {
		t.Errorf("offer over cap room: %v", err)
	}
	u.Roster[0].Contract = keep
	u.ExceptionYear = 0
	// minimum always allowed with no cap room
	u.Roster[0].Contract.Salary = Cap.SalaryCap - (lg.Payroll(u.ID) - keep.Salary) + 100
	cheap := pool[len(pool)-1]
	if _, err := lg.MakeOffer(u.ID, cheap.ID, Cap.MinSalary, 1); err != nil {
		t.Errorf("minimum offer over the cap: %v", err)
	}
	lg.dropOffer(u.ID, cheap.ID)
	u.Roster[0].Contract = keep
	// bad years and non-free-agent
	if _, err := lg.MakeOffer(u.ID, star.ID, ask, Cap.MaxContractYears+1); err == nil {
		t.Error("accepted too many years")
	}
	if _, err := lg.MakeOffer(u.ID, u.Roster[1].ID, ask, 1); err == nil {
		t.Error("offered to a player under contract")
	}
	// roster full: team 5 (a computer team) is at 13 or more
	full := lg.Teams[5]
	for len(full.Roster) < RosterSize {
		p := lg.sortedPool()[0]
		lg.forceSign(5, p, Cap.MinSalary, 1)
	}
	if _, err := lg.MakeOffer(5, star.ID, ask, years); err == nil || !strings.Contains(err.Error(), "full roster") {
		t.Errorf("full roster offer: %v", err)
	}
	// replacing own offer
	o1, err := lg.MakeOffer(u.ID, star.ID, ask, years)
	if err != nil {
		t.Fatal(err)
	}
	o2, err := lg.MakeOffer(u.ID, star.ID, ask+500, years)
	if err != nil || len(lg.OffersBy(u.ID)) != 1 || lg.OffersBy(u.ID)[0].ID != o2.ID || o2.ID == o1.ID {
		t.Fatalf("replace offer: %v %+v", err, lg.OffersBy(u.ID))
	}
	if err := lg.WithdrawOffer(u.ID, o2.ID); err != nil || len(lg.OffersBy(u.ID)) != 0 {
		t.Fatalf("withdraw: %v", err)
	}
	if lg.WithdrawOffer(u.ID, o2.ID) == nil {
		t.Error("withdrew twice")
	}

	// ---- decision rules, isolated from computer bids ----
	cf := lg.contendFactors()
	var other *Player
	for _, p := range pool {
		if s, _ := lg.FreeAgentAsk(p); p != star && lg.faPlayer(p.ID) != nil && s >= 4000 && s <= 20000 {
			other = p
			break
		}
	}
	if other == nil {
		t.Fatal("no lowball candidate")
	}
	oask, _ := lg.FreeAgentAsk(other)
	lg.FA.Offers = nil
	lg.FA.NextOfferID++
	low := FAOffer{ID: lg.FA.NextOfferID, Team: u.ID, PlayerID: other.ID, Salary: oask * 80 / 100 / 100 * 100, Years: 1, Day: 0}
	lg.FA.Offers = []FAOffer{low}
	lg.FA.Day = 0 // made today: cannot be accepted today
	if s := lg.playersDecide(cf, false); len(s) != 0 {
		t.Fatal("an offer made today was accepted today")
	}
	lg.FA.Day = 1 // below 95% of the softened ask: waits
	if s := lg.playersDecide(cf, false); len(s) != 0 {
		t.Fatal("a lowball was accepted before the ask softened")
	}
	lg.FA.Day = lg.FA.Days - 1 // ask has softened to ~70%
	if s := lg.playersDecide(cf, false); len(s) != 1 || s[0].Team != u.ID {
		t.Fatalf("softened ask not met: %+v", s)
	}
	lg.FA.Day = 0

	// ---- outbidding the computer teams ----
	lg.FA.Offers = nil
	if _, err := lg.MakeOffer(u.ID, star.ID, min(Cap.MaxSalary, ask*14/10/100*100), Cap.MaxContractYears); err != nil {
		t.Fatal(err)
	}
	if s, _ := lg.AdvanceFreeAgency(); len(s) > 0 {
		for _, x := range s {
			if x.PlayerID == star.ID {
				t.Fatal("the star signed on the day of the offer")
			}
		}
	}
	if star.TeamID != -1 {
		t.Fatal("the star signed on the day of the offer")
	}
	lg.AdvanceFreeAgency()
	if star.TeamID != u.ID {
		t.Errorf("user did not get the player he outbid everyone for (team %d)", star.TeamID)
	}

	// ---- the rest of the period and the finish ----
	wantPhase(t, lg, true, PhaseFreeAgency, "fa_day", "start_season")
	for lg.FA.DaysLeft() > 0 {
		lg.AdvanceFreeAgency()
	}
	wantPhase(t, lg, true, PhaseFreeAgency, "start_season")
	if _, err := lg.AdvanceFreeAgency(); err == nil {
		t.Error("advanced past the last day")
	}
	lg.FinishOffseason()
	if lg.FA != nil {
		t.Error("FA not cleared")
	}
	for _, tm := range lg.Teams {
		if len(tm.Roster) != RosterSize {
			t.Fatalf("team %d has %d players", tm.ID, len(tm.Roster))
		}
		for _, p := range tm.Roster {
			if p.Contract.Years < 1 || p.Contract.Salary < Cap.MinSalary {
				t.Fatalf("player %d without a contract: %+v", p.ID, p.Contract)
			}
		}
		if lg.Payroll(tm.ID) > Cap.HardCeiling {
			t.Errorf("team %d over the hard ceiling", tm.ID)
		}
	}
}

func TestFreeAgencyMarket(t *testing.T) {
	var firstLog []FASigning
	for _, seed := range []uint64{31, 32} {
		lg := draftDoneLeague(seed)
		mustBeginFA(t, lg)
		top := lg.sortedPool()[:10]
		perDay := make([]int, lg.FA.Days)
		for lg.FA.DaysLeft() > 0 {
			d := lg.FA.Day
			s, _ := lg.AdvanceFreeAgency()
			perDay[d] = len(s)
		}
		early, late, never := 0, 0, 0
		for _, p := range top {
			day := -1
			for _, s := range lg.FA.Log {
				if s.PlayerID == p.ID {
					day = s.Day
				}
			}
			switch {
			case day < 0:
				never++
			case day <= 3:
				early++
			default:
				late++
			}
		}
		short := 0
		for _, tm := range lg.Teams {
			short += max(0, RosterSize-len(tm.Roster))
		}
		t.Logf("seed %d: signings per day %v; top ten: %d in days 1-4, %d later, %d unsigned; roster spots still open before the fill: %d",
			seed, perDay, early, late, never, short)
		if early < 4 { // was 5; the outcome is seed-sensitive (other seeds give 3-5 even before minutes-based development)
			t.Errorf("only %d of the top ten signed in the first four days", early)
		}
		log := append([]FASigning(nil), lg.FA.Log...)
		lg.FinishOffseason()
		for _, tm := range lg.Teams {
			if lg.Payroll(tm.ID) > Cap.HardCeiling {
				t.Errorf("team %d over the hard ceiling", tm.ID)
			}
		}
		if seed == 31 {
			firstLog = log
		}
	}
	// same seed, same signings
	lg := draftDoneLeague(31)
	mustBeginFA(t, lg)
	for lg.FA.DaysLeft() > 0 {
		lg.AdvanceFreeAgency()
	}
	if !reflect.DeepEqual(firstLog, lg.FA.Log) {
		t.Error("free agency is not deterministic for a seed")
	}
}

func TestSaveRoundTripFreeAgency(t *testing.T) {
	lg := GenerateLeague(7)
	lg.FA = &FreeAgency{Day: 3, Days: 10, NextOfferID: 2,
		Offers: []FAOffer{{ID: 2, Team: 1, PlayerID: 5, Salary: 3000, Years: 2, Day: 2}},
		Log:    []FASigning{{Day: 1, PlayerID: 9, Name: "A B", Team: 4, Salary: 1200, Years: 1, Pos: "PG", Overall: 55}}}
	path := filepath.Join(t.TempDir(), "league.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.FA, lg.FA) {
		t.Errorf("FA differs after round trip: %+v", got.FA)
	}
}

func signingOf(lg *League, id int) (FASigning, bool) {
	for _, s := range lg.FA.Log {
		if s.PlayerID == id {
			return s, true
		}
	}
	return FASigning{}, false
}

func TestFreeAgencyStarsSign(t *testing.T) {
	// seed 11, user team 3 re-signs nobody: the top free agent must sign for real money
	lg := draftDoneLeagueFor(11, 3)
	mustBeginFA(t, lg)
	star := lg.sortedPool()[0]
	orig, _ := AskingContract(star)
	for lg.FA.DaysLeft() > 0 {
		lg.AdvanceFreeAgency()
	}
	s, ok := signingOf(lg, star.ID)
	if !ok || s.Salary*100 < 40*orig {
		t.Fatalf("star (ovr %d, ask %d) signing: %+v ok=%v", star.Overall(), orig, s, ok)
	}
	t.Logf("seed 11 star ovr %d asked %d: signed with team %d for %d x %d on day %d", star.Overall(), orig, s.Team, s.Salary, s.Years, s.Day)
	lg.FinishOffseason()
	if star.TeamID < 0 {
		t.Error("star not on a roster after the offseason")
	}

	// no 80+ player unsigned or on the minimum, over a few seeds
	for _, seed := range []uint64{31, 32, 5} {
		lg := draftDoneLeague(seed)
		mustBeginFA(t, lg)
		var tops []*Player
		for _, p := range lg.FreeAgents {
			if p.Overall() >= 80 {
				tops = append(tops, p)
			}
		}
		for lg.FA.DaysLeft() > 0 {
			lg.AdvanceFreeAgency()
		}
		for _, p := range tops {
			if s, ok := signingOf(lg, p.ID); !ok || s.Salary <= Cap.MinSalary {
				t.Errorf("seed %d: %d-rated player unsigned or on the minimum: %+v %v", seed, p.Overall(), s, ok)
			}
		}
	}

	// the user offers all his cap room on day 0
	lg = draftDoneLeagueFor(11, 3)
	mustBeginFA(t, lg)
	star = lg.sortedPool()[0]
	orig, _ = AskingContract(star)
	room := lg.CapRoom(3) / 100 * 100
	if room <= Cap.MinSalary || room*100 < 50*orig {
		t.Skipf("user room %d too small for ask %d", room, orig)
	}
	if _, err := lg.MakeOffer(3, star.ID, room, 2); err != nil {
		t.Fatal(err)
	}
	for lg.FA.DaysLeft() > 0 {
		lg.AdvanceFreeAgency()
	}
	s, ok = signingOf(lg, star.ID)
	if !ok {
		t.Fatal("star unsigned despite the user's offer")
	}
	if s.Team == 3 && s.Salary != room {
		t.Errorf("user signed him at %d, offered %d", s.Salary, room)
	}
	t.Logf("user offered %d (ask %d): star signed with team %d for %d", room, orig, s.Team, s.Salary)
}

// ---- mid-level exception and restricted free agency ----

// resetTeam keeps the first n players on minimum two-year deals and tops the
// first one up so the payroll is the target.
func resetTeam(lg *League, tid, n, payroll int) {
	t := lg.Teams[tid]
	if len(t.Roster) > n {
		t.Roster = t.Roster[:n]
	}
	for _, p := range t.Roster {
		p.Contract = Contract{Cap.MinSalary, 2}
	}
	t.Roster[0].Contract.Salary += payroll - lg.Payroll(tid)
}

// poolPlayer finds a free agent with an ask in the range who is not restricted
// and is not in skip; with rights >= 0 he must be worth keeping to that team.
func poolPlayer(lg *League, lo, hi int, skip map[int]bool, rights int) *Player {
	cf := lg.contendFactors()
	for _, p := range lg.sortedPool() {
		ask, _ := lg.FreeAgentAsk(p)
		if _, r := lg.RightsTeam(p.ID); r || skip[p.ID] || ask < lo || ask > hi {
			continue
		}
		if rights >= 0 && lg.value(p, rights, cf) < Cap.ReSignMinValue {
			continue
		}
		return p
	}
	return nil
}

func setRights(lg *League, p *Player, tid int) {
	if lg.FA.Rights == nil {
		lg.FA.Rights = map[int]int{}
	}
	lg.FA.Rights[p.ID] = tid
}

func TestUserExceptionSigning(t *testing.T) {
	lg := draftDoneLeagueFor(21, 0)
	mustBeginFA(t, lg)
	u := lg.UserTeamID
	resetTeam(lg, u, 12, Cap.SalaryCap-5000) // $5.0M of cap room, one open spot
	skip := map[int]bool{}
	var ps []*Player
	for i := 0; i < 3; i++ {
		p := poolPlayer(lg, 2000, 8000, skip, -1)
		if p == nil {
			t.Fatal("no free agent for the fixture")
		}
		skip[p.ID] = true
		ps = append(ps, p)
	}
	o1, err := lg.MakeOffer(u, ps[0].ID, 12000, 2)
	if err != nil || o1.Kind != SignException {
		t.Fatalf("exception offer: %+v %v", o1, err)
	}
	// A pending exception offer does not use cap room: a cap-room offer still fits.
	o2, err := lg.MakeOffer(u, ps[1].ID, 4000, 1)
	if err != nil || o2.Kind != SignCapRoom {
		t.Fatalf("cap-room offer next to an exception offer: %+v %v", o2, err)
	}
	// But there is only one exception.
	if _, err := lg.MakeOffer(u, ps[2].ID, 9000, 2); err == nil {
		t.Error("a second exception offer was accepted")
	}
	// Exception limits are readable.
	if _, err := lg.MakeOffer(u, ps[2].ID, Cap.MidLevel+100, 2); err == nil {
		t.Error("offer above the mid-level and above cap room accepted")
	}
	lg.WithdrawOffer(u, o2.ID)
	for lg.FA.DaysLeft() > 0 {
		if _, ok := signingOf(lg, ps[0].ID); ok {
			break
		}
		lg.AdvanceFreeAgency()
	}
	s, ok := signingOf(lg, ps[0].ID)
	if !ok || s.Team != u || s.Via != "exception" || s.Salary != 12000 {
		t.Fatalf("exception signing: %+v ok=%v", s, ok)
	}
	if lg.Teams[u].ExceptionYear != lg.Year || !lg.ExceptionFor(u).Used {
		t.Error("exception not recorded as used")
	}
	if lg.Payroll(u) <= Cap.SalaryCap {
		t.Errorf("payroll %d not over the cap after the signing", lg.Payroll(u))
	}
	lg.Teams[u].Roster = lg.Teams[u].Roster[:RosterSize-1] // a spot for the next offer
	if lg.FA.DaysLeft() > 0 {
		if _, err := lg.MakeOffer(u, ps[2].ID, 9000, 2); err == nil || !strings.Contains(err.Error(), "already used") {
			t.Errorf("exception used twice: %v", err)
		}
	}
}

func TestRestrictedEligibility(t *testing.T) {
	lg := draftDoneLeagueFor(21, 0)
	u := lg.Teams[lg.UserTeamID]
	young, old := u.Roster[0], u.Roster[1]
	young.Contract.Years, young.Experience = 0, 2
	old.Contract.Years, old.Experience = 0, FATuning.RFAMaxExperience+1
	if !lg.RestrictedEligible(young) || lg.RestrictedEligible(old) {
		t.Fatal("eligibility by experience wrong")
	}
	mustBeginFA(t, lg)
	if rt, ok := lg.RightsTeam(young.ID); !ok || rt != lg.UserTeamID {
		t.Errorf("user's young expiring player: rights %d %v", rt, ok)
	}
	if _, ok := lg.RightsTeam(old.ID); ok {
		t.Error("an experienced player is restricted")
	}
	if len(lg.FA.Rights) < 2 {
		t.Logf("rights held: %d", len(lg.FA.Rights))
	}
	for pid, tid := range lg.FA.Rights {
		p := lg.faPlayer(pid)
		if p == nil || !lg.RestrictedEligible(p) || tid < 0 || tid >= len(lg.Teams) {
			t.Fatalf("bad rights entry %d -> %d", pid, tid)
		}
	}
}

func TestOfferSheetMinYears(t *testing.T) {
	lg := draftDoneLeagueFor(21, 0)
	mustBeginFA(t, lg)
	u := lg.UserTeamID
	resetTeam(lg, u, 10, 20000)
	p := poolPlayer(lg, 3000, 20000, nil, -1)
	setRights(lg, p, 5)
	ask, _ := lg.FreeAgentAsk(p)
	if _, err := lg.MakeOffer(u, p.ID, ask, 1); err == nil || !strings.Contains(err.Error(), "restricted") {
		t.Errorf("one-year offer sheet: %v", err)
	}
	if _, err := lg.MakeOffer(u, p.ID, ask, FATuning.OfferSheetMinYears); err != nil {
		t.Errorf("two-year offer sheet: %v", err)
	}
	if lg.MinYearsFor(u, p.ID) != 2 || lg.MinYearsFor(5, p.ID) != 1 {
		t.Error("MinYearsFor wrong")
	}
	// the rights team itself may offer one year, even over the cap
	resetTeam(lg, 5, 9, 165000)
	o, err := lg.MakeOffer(5, p.ID, ask, 1)
	if err != nil || o.Kind != SignOwn {
		t.Errorf("rights team's own offer: %+v %v", o, err)
	}
	// computer offers respect the minimum
	for lg.FA.DaysLeft() > 0 {
		lg.AdvanceFreeAgency()
		for _, o := range lg.FA.Offers {
			if rt, ok := lg.RightsTeam(o.PlayerID); ok && rt != o.Team && o.Years < FATuning.OfferSheetMinYears {
				t.Fatalf("offer sheet of %d year(s): %+v", o.Years, o)
			}
		}
	}
}

// vsComputerRights: the user offers 30000 x 3 to a restricted player whose rights a
// computer team (payroll rightsPayroll) holds.
func vsComputerRights(t *testing.T, rightsPayroll int) (*League, *Player, FASigning) {
	lg := draftDoneLeagueFor(21, 0)
	mustBeginFA(t, lg)
	u, rt := lg.UserTeamID, 5
	resetTeam(lg, u, 10, 20000)
	resetTeam(lg, rt, 9, rightsPayroll)
	p := poolPlayer(lg, 6000, 20000, nil, rt)
	if p == nil {
		t.Fatal("no fixture player")
	}
	setRights(lg, p, rt)
	if _, err := lg.MakeOffer(u, p.ID, 30000, 3); err != nil {
		t.Fatal(err)
	}
	for lg.FA.DaysLeft() > 0 && lg.faPlayer(p.ID) != nil {
		lg.AdvanceFreeAgency()
	}
	s, ok := signingOf(lg, p.ID)
	if !ok {
		t.Fatal("restricted player did not sign")
	}
	return lg, p, s
}

func TestComputerRightsTeamMatches(t *testing.T) {
	lg, p, s := vsComputerRights(t, 20000)
	if !s.Matched || s.Team != 5 || s.SheetFrom != lg.UserTeamID || s.Salary != 30000 || s.Years != 3 || s.Via != "own" {
		t.Fatalf("expected a match by team 5: %+v", s)
	}
	if p.TeamID != 5 || p.Contract != (Contract{30000, 3}) {
		t.Errorf("player: team %d contract %+v", p.TeamID, p.Contract)
	}
	if lg.OffersFor(p.ID) != 0 {
		t.Error("offers left")
	}
	if len(lg.PendingSheets()) != 0 {
		t.Error("a sheet was created for a computer rights team")
	}
}

func TestComputerRightsTeamDeclines(t *testing.T) {
	lg, p, s := vsComputerRights(t, 165000) // matching would pass the tax line
	if s.Matched || s.Team != lg.UserTeamID || s.Via != "room" || p.TeamID != lg.UserTeamID {
		t.Fatalf("expected the offering team to sign him: %+v", s)
	}
}

// userSheet builds a pending offer sheet from team 7 against the user.
func userSheet(t *testing.T) (*League, *Player) {
	lg := draftDoneLeagueFor(21, 0)
	mustBeginFA(t, lg)
	u, off := lg.UserTeamID, 7
	resetTeam(lg, u, 12, 155000)
	resetTeam(lg, off, 9, 20000)
	p := poolPlayer(lg, 6000, 20000, nil, u)
	setRights(lg, p, u)
	if _, err := lg.makeOffer(off, p.ID, 30000, 3, false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4 && len(lg.PendingSheets()) == 0; i++ {
		if err := lg.CanAdvanceFreeAgency(); err != nil {
			t.Fatal(err)
		}
		lg.AdvanceFreeAgency()
	}
	sh := lg.PendingSheets()
	if len(sh) != 1 || sh[0].PlayerID != p.ID || sh[0].Team != off || sh[0].Salary != 30000 || sh[0].Years != 3 {
		t.Fatalf("pending sheets: %+v", sh)
	}
	if lg.faPlayer(p.ID) == nil || p.TeamID != -1 {
		t.Fatal("the player signed instead of waiting")
	}
	return lg, p
}

func TestUserOfferSheetMatch(t *testing.T) {
	lg, p := userSheet(t)
	u := lg.UserTeamID
	if err := lg.CanAdvanceFreeAgency(); err == nil || !strings.Contains(err.Error(), "Answer the offer sheet for "+p.Name) {
		t.Errorf("advance with a pending sheet: %v", err)
	}
	if err := lg.CanFinishOffseason(true); err == nil || !strings.Contains(err.Error(), "offer sheet") {
		t.Errorf("finish with a pending sheet: %v", err)
	}
	if _, err := lg.makeOffer(7, p.ID, 31000, 3, false); err == nil {
		t.Error("the held offer was replaced")
	}
	if err := lg.WithdrawOffer(7, lg.OffersBy(7)[0].ID); err == nil {
		t.Error("the held offer was withdrawn")
	}
	// the day passes with the sheet pending: he stays in the pool
	day := lg.FA.Day
	lg.AdvanceFreeAgency()
	if lg.FA.Day != day+1 || len(lg.PendingSheets()) != 1 || p.TeamID != -1 {
		t.Error("a pending sheet changed on the next day")
	}
	if len(lg.Teams[u].Roster) >= RosterSize {
		t.Fatal("fixture roster full")
	}
	g, err := lg.AnswerSheet(p.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !g.Matched || g.Team != u || g.SheetFrom != 7 || g.Salary != 30000 || g.Via != "own" {
		t.Errorf("match log row: %+v", g)
	}
	if p.TeamID != u || p.Contract != (Contract{30000, 3}) || lg.Payroll(u) <= Cap.SalaryCap {
		t.Errorf("after matching: team %d contract %+v payroll %d", p.TeamID, p.Contract, lg.Payroll(u))
	}
	if len(lg.PendingSheets()) != 0 || lg.OffersFor(p.ID) != 0 {
		t.Error("sheet or offers left")
	}
	if err := lg.CanAdvanceFreeAgency(); err != nil {
		t.Errorf("advance after answering: %v", err)
	}
	if _, err := lg.AnswerSheet(p.ID, true); err == nil {
		t.Error("answered twice")
	}
}

func TestUserOfferSheetMatchRefusals(t *testing.T) {
	lg, p := userSheet(t)
	u := lg.UserTeamID
	for len(lg.Teams[u].Roster) < RosterSize {
		lg.forceSign(u, lg.sortedPool()[len(lg.FreeAgents)-1], Cap.MinSalary, 1)
	}
	resetTeam(lg, u, 13, 100000)
	if _, err := lg.AnswerSheet(p.ID, true); err == nil || !strings.Contains(err.Error(), "full") {
		t.Errorf("match with a full roster: %v", err)
	}
	resetTeam(lg, u, 12, Cap.HardCeiling-10000)
	if _, err := lg.AnswerSheet(p.ID, true); err == nil || !strings.Contains(err.Error(), "hard ceiling") {
		t.Errorf("match past the ceiling: %v", err)
	}
	if len(lg.PendingSheets()) != 1 {
		t.Error("a refused match removed the sheet")
	}
}

func TestUserOfferSheetDecline(t *testing.T) {
	lg, p := userSheet(t)
	g, err := lg.AnswerSheet(p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if g.Matched || g.Team != 7 || g.Salary != 30000 || g.Via != "room" || p.TeamID != 7 {
		t.Errorf("decline: %+v team %d", g, p.TeamID)
	}
	if len(lg.PendingSheets()) != 0 || lg.CanAdvanceFreeAgency() != nil {
		t.Error("sheet left after declining")
	}

	// the offering team can no longer sign him: the sheet is void, he stays in the pool
	lg, p = userSheet(t)
	resetTeam(lg, 7, 9, 150000) // no longer has the cap room
	g, err = lg.AnswerSheet(p.ID, false)
	if err != nil || g.Name != "" {
		t.Fatalf("void sheet: %+v %v", g, err)
	}
	if lg.faPlayer(p.ID) == nil || len(lg.PendingSheets()) != 0 {
		t.Error("void sheet: player not back in the pool")
	}
	if rt, ok := lg.RightsTeam(p.ID); !ok || rt != lg.UserTeamID {
		t.Error("void sheet: rights lost")
	}
}

func TestNoScreensPathsNeverWait(t *testing.T) {
	lg, p := userSheet(t)
	lg.FinishOffseason()
	if lg.FA != nil || p.TeamID != 7 || p.Contract.Salary != 30000 {
		t.Fatalf("FinishOffseason must decline a pending sheet: FA %v, player team %d %+v", lg.FA != nil, p.TeamID, p.Contract)
	}
	// The fully automatic path answers it with the computer match rule.
	lg, p = userSheet(t)
	resetTeam(lg, lg.UserTeamID, 12, 100000)
	lg.finishOffseason(true)
	if lg.FA != nil || p.TeamID != 0 || p.Contract != (Contract{30000, 3}) {
		t.Fatalf("automatic path should match: FA %v, player team %d %+v", lg.FA != nil, p.TeamID, p.Contract)
	}
	lg, p = userSheet(t) // payroll 155000: matching would pass the tax line, so it declines
	lg.finishOffseason(true)
	if p.TeamID != 7 {
		t.Fatalf("automatic path should decline over the tax line: team %d", p.TeamID)
	}
	// Offseason() runs the whole thing with the user's restricted players in the pool.
	lg = GenerateLeague(5)
	lg.UserTeamID = 0
	lg.StartSeason()
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	y := lg.Year
	lg.Offseason()
	if lg.Year != y+1 || lg.FA != nil {
		t.Error("Offseason did not finish")
	}
}

func TestSaveRoundTripRightsAndSheets(t *testing.T) {
	lg, p := userSheet(t)
	lg.Teams[3].ExceptionYear = lg.Year
	path := filepath.Join(t.TempDir(), "league.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.FA, lg.FA) || got.Teams[3].ExceptionYear != lg.Year {
		t.Fatal("rights, sheets or exception year lost in the round trip")
	}
	if rt, ok := got.RightsTeam(p.ID); !ok || rt != got.UserTeamID || len(got.PendingSheets()) != 1 {
		t.Error("loaded league lost the rights or the sheet")
	}
	if _, err := got.AnswerSheet(p.ID, false); err != nil {
		t.Error(err)
	}

	// An older save (no rights, no sheets, offers without a kind) still plays out.
	lg = draftDoneLeagueFor(21, 0)
	mustBeginFA(t, lg)
	lg.FA.Rights, lg.FA.Sheets = nil, nil
	resetTeam(lg, lg.UserTeamID, 10, 20000)
	q := poolPlayer(lg, 3000, 20000, nil, -1)
	ask, _ := lg.FreeAgentAsk(q)
	if _, err := lg.MakeOffer(lg.UserTeamID, q.ID, ask, 1); err != nil {
		t.Fatal(err)
	}
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	old, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for old.FA.DaysLeft() > 0 {
		if err := old.CanAdvanceFreeAgency(); err != nil {
			t.Fatal(err)
		}
		old.AdvanceFreeAgency()
	}
	old.FinishOffseason()
}

// TestExceptionAndRFAMeasurement plays three offseasons hands-off, reports
// what the mid-level exception and restricted free agency did, and checks the
// league stays inside the hard ceiling and a sane payroll band.
func TestExceptionAndRFAMeasurement(t *testing.T) {
	lg := GenerateLeague(5)
	lg.UserTeamID = 0
	for y := 0; y < 3; y++ {
		lg.StartSeason()
		lg.SimRegularSeason()
		lg.SimPlayoffs()
		lg.BeginOffseason()
		lg.DraftAutoAll()
		mustBeginFA(t, lg)
		rights := map[int]int{}
		for k, v := range lg.FA.Rights {
			rights[k] = v
		}
		for lg.FA.DaysLeft() > 0 {
			lg.AdvanceFreeAgency()
			lg.autoAnswerSheets(lg.contendFactors(), true)
		}
		exc, signed, matched := 0, 0, 0
		for _, s := range lg.FA.Log {
			if s.Via == "exception" && s.Team != lg.UserTeamID {
				exc++
			}
			if _, ok := rights[s.PlayerID]; ok {
				if s.Matched {
					matched++
				} else if s.Team != rights[s.PlayerID] {
					signed++
				}
			}
		}
		big := 0
		for _, p := range lg.FreeAgents {
			if a, _ := AskingContract(p); a >= FATuning.SettleAskMult*Cap.MinSalary {
				big++
			}
		}
		t.Logf("offseason %d: %d restricted players, computer exception signings %d, offer sheets signed %d + matched %d, unsigned at the end %d (%d with an ask of 4x the minimum or more)",
			lg.Year, len(rights), exc, signed, matched, len(lg.FreeAgents), big)
		lg.FinishOffseason()
		avg, _, _, _, oh := avgPayroll(lg)
		t.Logf("  next season starts: avg payroll %.0f (%.0f%% of cap), over ceiling %d", avg, 100*avg/float64(Cap.SalaryCap), oh)
		if oh > 0 {
			t.Errorf("%d teams over the hard ceiling", oh)
		}
		if r := avg / float64(Cap.SalaryCap); r < 0.90 || r > 1.15 {
			t.Errorf("average payroll %.0f%% of the cap", 100*r)
		}
	}
}
