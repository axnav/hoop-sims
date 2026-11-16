package engine

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func ownerLeague(seed uint64, team int) *League {
	lg := GenerateLeague(seed)
	lg.UserTeamID = team
	lg.StartSeason()
	return lg
}

// setTeamRatings sets every rating of every player on a team (a test lever).
func setTeamRatings(tm *Team, v int) {
	for _, p := range tm.Roster {
		p.Ratings = Ratings{v, v, v, v, v, v, v}
	}
}

// ownerSeasonSteps plays a season and begins the offseason.
func ownerSeasonSteps(lg *League) {
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	lg.BeginOffseason()
}

// ownerNextSeason finishes the offseason by hand (the hands-off stepped flow).
func ownerNextSeason(lg *League) {
	lg.DraftAutoAll()
	lg.BeginFreeAgency()
	lg.FinishOffseason()
	lg.StartSeason()
}

func TestOwnerDerivedDeterministic(t *testing.T) {
	a, b := GenerateLeague(7), GenerateLeague(7)
	count := map[string]int{}
	pers := map[string]int{}
	for id := 0; id < NumTeams; id++ {
		x, y := a.OwnerOf(id), b.OwnerOf(id)
		if x != y {
			t.Fatalf("team %d: owner differs between identical leagues: %v %v", id, x, y)
		}
		if x.Name == "" || x.Personality == "" {
			t.Fatalf("team %d: empty owner %v", id, x)
		}
		count[x.Market]++
		pers[x.Personality]++
	}
	if count[MarketSmall] != 10 || count[MarketMedium] != 12 || count[MarketLarge] != 8 {
		t.Fatalf("market spread %v, want 10/12/8", count)
	}
	if len(pers) != 3 {
		t.Fatalf("personalities %v: want all three present", pers)
	}
	c := GenerateLeague(8)
	diff := 0
	for id := 0; id < NumTeams; id++ {
		if a.OwnerOf(id) != c.OwnerOf(id) {
			diff++
		}
	}
	if diff == 0 {
		t.Fatal("a different seed gave the same owners")
	}
}

func TestOwnerFinancesArithmetic(t *testing.T) {
	lg := ownerLeague(3, 0)
	m := lg.OwnerFinances()
	if m.Revenue != m.Market+m.Wins+m.Playoffs || m.Costs != m.Salaries+m.DeadMoney+m.Tax || m.Profit != m.Revenue-m.Costs {
		t.Fatalf("pieces do not add up: %+v", m)
	}
	if m.Salaries+m.DeadMoney != lg.Payroll(0) {
		t.Fatalf("salaries %d + dead %d != payroll %d", m.Salaries, m.DeadMoney, lg.Payroll(0))
	}
	// A taxed team with dead money clearly loses money.
	tm := lg.Teams[0]
	tm.DeadMoney = append(tm.DeadMoney, DeadContract{Salary: 8000, Years: 2})
	for _, p := range tm.Roster {
		p.Contract = Contract{Salary: 15000, Years: 2}
	}
	bad := lg.OwnerFinances()
	if bad.Tax <= 0 || bad.DeadMoney != 8000 || bad.Profit > -30000 {
		t.Fatalf("taxed team with dead money should clearly lose money: %+v", bad)
	}
	// A big market earns clearly more than a small one (same record).
	var small, large = -1, -1
	for id := 0; id < NumTeams; id++ {
		switch lg.OwnerOf(id).Market {
		case MarketSmall:
			small = id
		case MarketLarge:
			large = id
		}
	}
	ms, ml := lg.ownerMoneyOf(small, nil), lg.ownerMoneyOf(large, nil)
	if ml.Revenue-ms.Revenue < 20000 {
		t.Fatalf("large %d vs small %d: gap too small", ml.Revenue, ms.Revenue)
	}
	// A medium team at the cap with 41 wins roughly breaks even.
	neutral := OwnerTuning.MarketBase[1] + OwnerTuning.PerHomeWin*OwnerTuning.HomeGames/2
	if d := neutral - Cap.SalaryCap; d < -6000 || d > 6000 {
		t.Fatalf("medium market with 41 wins earns %d against a cap of %d", neutral, Cap.SalaryCap)
	}
}

func TestOwnerFinancesPlayedSeason(t *testing.T) {
	lg := ownerLeague(5, 4)
	lg.SimDays(20)
	m := lg.OwnerFinances()
	if !m.Projected {
		t.Fatal("a season in progress should be projected")
	}
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	f := lg.OwnerFinances()
	if f.Projected {
		t.Fatal("a finished season is not projected")
	}
	hw := 0
	for _, g := range lg.Season.Games {
		if g.Home == 4 && g.HomeScore > g.AwayScore {
			hw++
		}
	}
	po := 0
	for _, sr := range lg.Season.Series {
		for _, g := range sr.Games {
			if g.Home == 4 {
				po++
			}
		}
	}
	if f.Wins != hw*OwnerTuning.PerHomeWin || f.Playoffs != po*OwnerTuning.PerPlayoffHomeGame {
		t.Fatalf("revenue pieces wrong: %+v home wins %d playoff home games %d", f, hw, po)
	}
}

func TestOwnerGoalsFitRoster(t *testing.T) {
	keys := map[string]int{}
	for _, seed := range []uint64{1, 2, 3} {
		lg := GenerateLeague(seed)
		for team := 0; team < NumTeams; team++ {
			lg.UserTeamID = team
			lg.StartSeason()
			o := lg.Owner
			if len(o.Goals) != 3 {
				t.Fatalf("seed %d team %d: %d goals", seed, team, len(o.Goals))
			}
			rank := lg.ownerRank(team)
			info := lg.OwnerOf(team)
			w := ownerWeights[info.Personality]
			for i, kind := range []string{"results", "money", "building"} {
				g := o.Goals[i]
				if g.Kind != kind || g.Title == "" || g.Detail == "" || g.Weight < 1 || g.Weight > 3 || g.Weight != w[i] {
					t.Fatalf("seed %d team %d goal %d bad: %+v", seed, team, i, g)
				}
				keys[g.Key]++
			}
			res := o.Goals[0]
			if rank > 20 && (res.Key == "round" || res.Key == "conf_finals") {
				t.Fatalf("a rank %d roster was asked for %s", rank, res.Key)
			}
			if rank > 3 && res.Key == "conf_finals" {
				t.Fatalf("a rank %d roster was asked for the conference finals", rank)
			}
			if res.Key == "wins" && (res.Target < OwnerTuning.MinWinsGoal || res.Target > 60) {
				t.Fatalf("wins goal %d", res.Target)
			}
			if o.Budget < OwnerTuning.BudgetFloor || o.Budget > Cap.HardCeiling {
				t.Fatalf("budget %d", o.Budget)
			}
			if info.Personality != OwnerWinNow && o.Budget > Cap.TaxLine {
				t.Fatalf("%s owner budget %d above the tax line", info.Personality, o.Budget)
			}
		}
	}
	for _, k := range []string{"wins", "playoffs", "round", "budget", "tax", "profit", "improve", "keep_star", "rebuild"} {
		if keys[k] == 0 {
			t.Errorf("goal %q never chosen over 90 rosters: %v", k, keys)
		}
	}
	if keys["title"] != 0 {
		t.Fatal("somebody was asked for a title")
	}
	t.Logf("goal mix over 90 rosters: %v", keys)
}

// teamWith returns a team whose owner has the personality.
func teamWith(lg *League, pers string) int {
	for id := 0; id < NumTeams; id++ {
		if lg.OwnerOf(id).Personality == pers {
			return id
		}
	}
	return -1
}

func TestOwnerTrustByPersonality(t *testing.T) {
	delta := map[string][2]int{}
	for _, pers := range []string{OwnerWinNow, OwnerPatient, OwnerCheap} {
		for i, budget := range []int{1 << 30, 0} { // goal "budget": met, then missed
			lg := GenerateLeague(11)
			lg.UserTeamID = teamWith(lg, pers)
			lg.StartSeason()
			lg.Season.Day = SeasonDays // the regular season "is over"; results goals are judged on what they are
			o := lg.Owner
			o.Budget = budget
			o.Goals = []OwnerGoal{{Key: "budget", Kind: "money", Title: "Budget", Weight: 2}}
			lg.Season.Champion = 0
			lg.Season.Wins[lg.UserTeamID] = OwnerTuning.WinsPivot // the wins term is zero; only the goal moves trust
			before := o.Trust
			lg.ownerSeasonEnd()
			d := o.Trust - before
			x := delta[pers]
			x[i] = d
			delta[pers] = x
			if !o.Goals[0].Done || o.Goals[0].Met != (i == 0) {
				t.Fatalf("%s: goal judged wrong: %+v", pers, o.Goals[0])
			}
		}
	}
	for pers, d := range delta {
		if d[0] <= 0 || d[1] >= 0 {
			t.Fatalf("%s: met moved %d, missed moved %d", pers, d[0], d[1])
		}
	}
	if delta[OwnerWinNow][0] <= delta[OwnerPatient][0] || delta[OwnerWinNow][1] >= delta[OwnerPatient][1] {
		t.Fatalf("win-now trust should move faster than patient: %v", delta)
	}
}

func TestOwnerOverBudgetWeek(t *testing.T) {
	lg := ownerLeague(2, 0)
	o := lg.Owner
	o.Budget = 1000
	before := o.Trust
	lg.SimDays(7)
	if o.OverWeeks != 1 || o.Trust != before-OwnerTuning.OverWeekPenalty {
		t.Fatalf("after one week over budget: weeks %d trust %d -> %d", o.OverWeeks, before, o.Trust)
	}
	lg.SimDays(7)
	if o.OverWeeks != 2 {
		t.Fatalf("weeks %d, want 2", o.OverWeeks)
	}
	over := 0
	for _, m := range o.Messages {
		if m.Tone == "warn" {
			over++
		}
	}
	if over == 0 {
		t.Fatal("no warning message")
	}
	// Inside the budget nothing is lost.
	lg2 := ownerLeague(2, 0)
	lg2.Owner.Budget = 1 << 30
	b2 := lg2.Owner.Trust
	lg2.SimDays(14)
	if lg2.Owner.OverWeeks != 0 || lg2.Owner.Trust != b2 {
		t.Fatalf("inside the budget: weeks %d trust %d -> %d", lg2.Owner.OverWeeks, b2, lg2.Owner.Trust)
	}
}

func TestOwnerPaceWarningOnce(t *testing.T) {
	lg := ownerLeague(2, 0)
	o := lg.Owner
	o.Budget = 1 << 30
	o.Goals = []OwnerGoal{{Key: "wins", Kind: "results", Title: "Win at least 82 games", Weight: 2, Target: 82}}
	setTeamRatings(lg.Teams[0], 30)
	before := o.Trust
	lg.SimDays(60)
	if !o.PaceWarned {
		t.Fatal("no pace check")
	}
	if o.Trust != before-OwnerTuning.PacePenalty {
		t.Fatalf("trust %d -> %d, want one pace penalty", before, o.Trust)
	}
}

func TestOwnerOldSaveGetsOwnerAtNextSeason(t *testing.T) {
	ownerOff = true
	lg := GenerateLeague(4)
	lg.UserTeamID = 0
	lg.StartSeason()
	ownerOff = false
	if lg.Owner != nil {
		t.Fatal("test setup: owner should not exist")
	}
	path := filepath.Join(t.TempDir(), "old.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	old, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if old.Owner != nil {
		t.Fatal("an old save must load without an owner")
	}
	old.SimRegularSeason()
	old.SimPlayoffs()
	old.BeginOffseason() // nothing is judged without an owner
	if old.Owner != nil || old.OwnerFired() {
		t.Fatal("no owner yet, so nothing can fire the user")
	}
	if err := old.CanGoToDraft(); err != nil {
		t.Fatal(err)
	}
	ownerNextSeason(old)
	o := old.Owner
	if o == nil || o.Trust != OwnerTuning.StartTrust || len(o.Goals) != 3 || o.Budget == 0 {
		t.Fatalf("owner after the next season start: %+v", o)
	}
	if len(o.Ledger) != 0 {
		t.Fatal("the unobserved season must not be in the ledger")
	}
}

func TestOwnerSaveRoundTripAndLedger(t *testing.T) {
	lg := ownerLeague(6, 3)
	ownerSeasonSteps(lg)
	o := lg.Owner
	if len(o.Ledger) != 1 {
		t.Fatalf("ledger rows %d", len(o.Ledger))
	}
	r := o.Ledger[0]
	if r.Year != lg.Year || r.Wins != lg.Season.Wins[3] || r.Losses != lg.Season.Losses[3] || r.Playoffs == "" || len(r.Goals) != 3 ||
		r.Profit != r.Revenue-r.Costs || r.TrustEnd != o.Trust || r.TrustStart != OwnerTuning.StartTrust {
		t.Fatalf("ledger row %+v (trust %d)", r, o.Trust)
	}
	if len(o.Messages) < 3 || o.Messages[0].Text == "" {
		t.Fatalf("messages %+v", o.Messages)
	}
	path := filepath.Join(t.TempDir(), "s.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(lg.Owner)
	b, _ := json.Marshal(back.Owner)
	if string(a) != string(b) {
		t.Fatalf("owner changed in a save round trip:\n%s\n%s", a, b)
	}
}

// ruinTeam makes the user's team hopeless and expensive.
func ruinTeam(lg *League) {
	tm := lg.Teams[lg.UserTeamID]
	setTeamRatings(tm, 30)
	for _, p := range tm.Roster {
		p.Contract = Contract{Salary: 14000, Years: 3}
	}
}

func TestOwnerFiringAndJobs(t *testing.T) {
	lg := ownerLeague(9, 0)
	fired := 0
	for y := 1; y <= 8 && !lg.OwnerFired(); y++ {
		ruinTeam(lg)
		ownerSeasonSteps(lg)
		if lg.OwnerFired() {
			fired = y
			break
		}
		lg.Offseason2()
		lg.StartSeason()
	}
	if fired == 0 {
		t.Fatalf("a hopeless, overspending user was not fired in 8 seasons (trust %d)", lg.Owner.Trust)
	}
	t.Logf("fired after %d seasons, trust %d", fired, lg.Owner.Trust)
	o := lg.Owner
	if o.FiredText == "" || o.Messages[len(o.Messages)-1].Tone != "bad" {
		t.Fatalf("fired without a message: %+v", o)
	}
	// Guards refuse with the engine's sentence.
	if err := lg.CanGoToDraft(); err != ErrOwnerFired {
		t.Fatalf("CanGoToDraft: %v", err)
	}
	for name, err := range map[string]error{
		"CanDraft": lg.CanDraft(true), "CanUserPick": lg.CanUserPick(), "CanFinishOffseason": lg.CanFinishOffseason(true),
		"CanAdvanceFreeAgency": lg.CanAdvanceFreeAgency(),
	} {
		if err != ErrOwnerFired {
			t.Fatalf("%s: %v", name, err)
		}
	}
	jobs := lg.OwnerJobs()
	if len(jobs) != 3 {
		t.Fatalf("jobs %v", jobs)
	}
	seen := map[int]bool{}
	for _, id := range jobs {
		if id == lg.UserTeamID || id < 0 || id >= NumTeams || seen[id] {
			t.Fatalf("bad job list %v (user %d)", jobs, lg.UserTeamID)
		}
		seen[id] = true
	}
	// They are the three worst records without the user's team.
	worst := 0
	order := lg.Season.Standings()
	for i := NumTeams - 1; i >= 0 && worst < 3; i-- {
		if order[i] == lg.UserTeamID {
			continue
		}
		if !seen[order[i]] {
			t.Fatalf("job list %v is not the worst three (order %v)", jobs, order)
		}
		worst++
	}
}

// Offseason2 finishes a begun offseason the hands-off way with the computer
// re-signing the user's players (a test helper for loops that keep playing).
func (lg *League) Offseason2() {
	lg.DraftAutoAll()
	lg.reSignTeam(lg.Teams[lg.UserTeamID], lg.contendFactors())
	lg.finishOffseason(true)
}

func TestOwnerTakeJob(t *testing.T) {
	lg := ownerLeague(9, 0)
	for y := 1; y <= 8 && !lg.OwnerFired(); y++ {
		ruinTeam(lg)
		ownerSeasonSteps(lg)
		if lg.OwnerFired() {
			break
		}
		lg.Offseason2()
		lg.StartSeason()
	}
	if !lg.OwnerFired() {
		t.Fatal("setup: not fired")
	}
	if _, err := lg.TakeJob(lg.UserTeamID); err != ErrOwnerBadJob {
		t.Fatalf("own team as a job: %v", err)
	}
	// Things that belong to the old team.
	old := lg.UserTeamID
	lg.Teams[old].System = "pace"
	lg.Teams[old].Lineup = &Lineup{}
	lg.Proposals = append(lg.Proposals, Proposal{ID: 99, Team: 5})
	lg.SetGMPlan("rebuild")
	points := lg.Draft.ScoutPoints
	job := lg.OwnerJobs()[1]
	msg, err := lg.TakeJob(job)
	if err != nil || msg == "" {
		t.Fatalf("TakeJob: %q %v", msg, err)
	}
	o := lg.Owner
	if lg.UserTeamID != job || o.Fired || o.Trust != OwnerTuning.StartTrust || o.BadSeasons != 0 {
		t.Fatalf("after the job: user %d fired %v trust %d bad %d", lg.UserTeamID, o.Fired, o.Trust, o.BadSeasons)
	}
	if lg.Teams[old].System != "" || lg.Teams[old].Lineup != nil || len(lg.Proposals) != 0 {
		t.Fatal("old team's choices or proposals were not cleared")
	}
	if lg.GMPlan() != "rebuild" || lg.Draft.ScoutPoints != points {
		t.Fatal("assistant plan or scouting points were lost")
	}
	if _, err := lg.TakeJob(job); err != ErrOwnerNotFired {
		t.Fatalf("second TakeJob: %v", err)
	}
	if lg.CanGoToDraft() != nil {
		t.Fatal("the offseason should go on after taking a job")
	}
	// A whole cycle runs to the next season with full rosters.
	ownerNextSeason(lg)
	for _, tm := range lg.Teams {
		if len(tm.Roster) != RosterSize {
			t.Fatalf("team %d has %d players", tm.ID, len(tm.Roster))
		}
	}
	if len(lg.Owner.Goals) != 3 || lg.Owner.Goals[0].Done {
		t.Fatalf("new goals missing: %+v", lg.Owner.Goals)
	}
	ownerSeasonSteps(lg)
	if len(lg.Owner.Ledger) == 0 {
		t.Fatal("no ledger row for the new job")
	}
}

func TestOwnerNoFiring(t *testing.T) {
	lg := ownerLeague(9, 0)
	lg.Options.NoFiring = true
	for y := 1; y <= 8; y++ {
		ruinTeam(lg)
		ownerSeasonSteps(lg)
		if lg.OwnerFired() {
			t.Fatalf("fired with firing switched off (season %d)", y)
		}
		if lg.Owner.Trust < 1 {
			t.Fatalf("trust %d with firing off", lg.Owner.Trust)
		}
		lg.Offseason2()
		lg.StartSeason()
	}
	if lg.Owner.Trust != 1 {
		t.Logf("trust after 8 bad seasons: %d", lg.Owner.Trust)
	}
}

func TestOwnerGoodUserNotFired(t *testing.T) {
	lg := ownerLeague(9, 0)
	for y := 1; y <= 5; y++ {
		setTeamRatings(lg.Teams[0], 90)
		ownerSeasonSteps(lg)
		if lg.OwnerFired() {
			t.Fatalf("a winning user was fired in season %d (trust %d)", y, lg.Owner.Trust)
		}
		lg.Offseason2()
		lg.StartSeason()
	}
	if lg.Owner.Trust <= OwnerTuning.StartTrust {
		t.Fatalf("trust %d after five great seasons", lg.Owner.Trust)
	}
}

func TestOwnerAssistantRespectsBudget(t *testing.T) {
	lg := ownerLeague(2, 0)
	lg.Owner.Budget = 100000
	p := lg.ownerBestPlayer()
	limit, bound := lg.gmResignLimitOwner(GMPlanBalanced, p)
	if !bound || limit != 100000+OwnerTuning.AssistantOverBudget {
		t.Fatalf("limit %d bound %v", limit, bound)
	}
	// Without an owner the old limit stands.
	lg.Owner = nil
	if l2, b2 := lg.gmResignLimitOwner(GMPlanBalanced, p); b2 || l2 < Cap.TaxLine {
		t.Fatalf("no owner: limit %d bound %v", l2, b2)
	}
}

// The owner only observes: with and without it the same seed gives the same
// games, rosters, contracts and transactions over a season and offseason.
func TestOwnerDoesNotChangeTheGame(t *testing.T) {
	run := func(off bool) []string {
		ownerOff = off
		defer func() { ownerOff = false }()
		lg := GenerateLeague(21)
		lg.UserTeamID = 5
		var out []string
		for y := 0; y < 2; y++ {
			lg.StartSeason()
			lg.SimRegularSeason()
			lg.SimPlayoffs()
			for _, v := range []any{lg.Season.Wins, lg.Season.Losses, lg.Season.PointDiff, lg.Season.Stats, lg.Season.Series} {
				b, _ := json.Marshal(v)
				out = append(out, string(b))
			}
			lg.Offseason()
		}
		for _, v := range []any{lg.Teams, lg.FreeAgents, lg.Transactions, lg.Records, lg.Events, lg.Picks, lg.Retired} {
			b, _ := json.Marshal(v)
			out = append(out, string(b))
		}
		if off != (lg.Owner == nil) {
			t.Fatalf("owner presence %v with off=%v", lg.Owner != nil, off)
		}
		return out
	}
	a, b := run(false), run(true)
	if len(a) != len(b) {
		t.Fatal("length")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("piece %d differs with the owner switched on", i)
		}
	}
}

// handsOffSeasons plays the stepped hands-off flow (nothing re-signed, no
// offers) and returns the season (1-based) after which the user was fired, or
// 0, with the trust path.
func handsOffSeasons(lg *League, n int) (int, []int) {
	var path []int
	for y := 1; y <= n; y++ {
		lg.SimRegularSeason()
		lg.SimPlayoffs()
		lg.BeginOffseason()
		path = append(path, lg.Owner.Trust)
		if lg.OwnerFired() {
			return y, path
		}
		lg.DraftAutoUntilUser()
		lg.DraftAutoAll()
		lg.BeginFreeAgency()
		lg.FinishOffseason()
		lg.StartSeason()
	}
	return 0, path
}

func TestOwnerNeglectedTeamIsFired(t *testing.T) {
	lg := ownerLeague(2, 3)
	y, path := handsOffSeasons(lg, 9)
	t.Logf("trust path %v, fired after season %d", path, y)
	if y == 0 || y > 5 || y < 2 {
		t.Fatalf("a neglected team should be let go in seasons 2 to 5, got %d (trust %v)", y, path)
	}
}

// judgeSeason sets a finished season by hand and judges it.
func judgeSeason(lg *League, wins int) {
	lg.Season.Day = SeasonDays
	lg.Season.Champion = 0
	lg.Season.Wins[lg.UserTeamID] = wins
	lg.Season.Losses[lg.UserTeamID] = SeasonGames - wins
	lg.ownerSeasonEnd()
}

func TestOwnerLosingPatienceAndRebuildGrace(t *testing.T) {
	run := func(dir string, buildMet bool, wins int) (int, *OwnerState) {
		lg := GenerateLeague(11)
		lg.UserTeamID = teamWith(lg, OwnerWinNow)
		lg.StartSeason()
		o := lg.Owner
		o.StartDir = dir
		o.Budget = 1 << 30
		o.Goals = []OwnerGoal{{Key: "budget", Kind: "money", Title: "B", Weight: 2},
			{Key: "keep_star", Kind: "building", Title: "K", Weight: 2, Target: lg.ownerBestPlayer().ID}}
		if !buildMet {
			o.Goals[1].Target = -5
		}
		before := o.Trust
		judgeSeason(lg, wins)
		return o.Trust - before, o
	}
	// Never rises in a losing season even with every goal met.
	if d, _ := run("contender", true, OwnerTuning.LosingWins-1); d > 0 {
		t.Fatalf("trust rose by %d in a losing season", d)
	}
	plain, o1 := run("contender", true, 20)
	grace, o2 := run("rebuilding", true, 20)
	noBuild, _ := run("rebuilding", false, 20)
	if grace <= plain || o2.LosingStreak != 0 || o2.GraceUsed != 1 || o1.LosingStreak != 1 {
		t.Fatalf("grace: plain %d (streak %d) grace %d (streak %d used %d)", plain, o1.LosingStreak, grace, o2.LosingStreak, o2.GraceUsed)
	}
	if noBuild >= grace {
		t.Fatalf("no grace when the building goal is missed: %d vs %d", noBuild, grace)
	}
	// The penalty rises with the streak.
	lg := ownerLeague(11, teamWith(GenerateLeague(11), OwnerWinNow))
	lg.Owner.Goals = nil
	var drops []int
	for i := 0; i < 3; i++ {
		lg.Owner.Trust = 100
		judgeSeason(lg, 25)
		drops = append(drops, 100-lg.Owner.Trust)
	}
	if !(drops[0] < drops[1] && drops[1] < drops[2]) {
		t.Fatalf("losing penalty should rise: %v", drops)
	}
	// A patient owner forgives two rebuild seasons.
	lg2 := GenerateLeague(11)
	lg2.UserTeamID = teamWith(lg2, OwnerPatient)
	lg2.StartSeason()
	for i := 0; i < 2; i++ {
		lg2.Owner.StartDir = "rebuilding"
		lg2.Owner.Goals = []OwnerGoal{{Key: "keep_star", Kind: "building", Title: "K", Weight: 1, Target: lg2.ownerBestPlayer().ID}}
		judgeSeason(lg2, 20)
	}
	if lg2.Owner.GraceUsed != 2 || lg2.Owner.LosingStreak != 0 {
		t.Fatalf("patient owner grace: used %d streak %d", lg2.Owner.GraceUsed, lg2.Owner.LosingStreak)
	}
}

func TestOwnerWinsGoalFloorAndProgress(t *testing.T) {
	lg := ownerLeague(2, 3)
	o := lg.Owner
	o.PrevWins, o.PrevLosing = 20, true
	g := lg.ownerPickGoals(lg.OwnerOf(3), o.Budget)[0]
	if g.Key == "wins" && g.Target < 20+OwnerTuning.ProgressWins {
		t.Fatalf("after a losing season the target %d should be at least %d", g.Target, 20+OwnerTuning.ProgressWins)
	}
	o.PrevLosing = false
	if g := lg.ownerPickGoals(lg.OwnerOf(3), o.Budget)[0]; g.Key == "wins" && g.Target < OwnerTuning.MinWinsGoal {
		t.Fatalf("wins goal %d under the floor", g.Target)
	}
}

func TestOwnerSpendFloorGoal(t *testing.T) {
	lg := GenerateLeague(2)
	lg.UserTeamID = 3
	for _, p := range lg.Teams[3].Roster {
		p.Contract = Contract{Salary: Cap.MinSalary, Years: 2}
	}
	lg.StartSeason()
	m := lg.Owner.Goals[1]
	if m.Key != "spend" || m.Target < Cap.SalaryCap*7/10 || !containsStr(m.Title, "payroll of at least") {
		t.Fatalf("stripped payroll should get the spend goal: %+v", m)
	}
	lg.Season.Day = SeasonDays
	lg.Season.Champion = 0
	if met, _, _, _ := lg.ownerGoalState(&lg.Owner.Goals[1]); met {
		t.Fatal("a stripped payroll must not meet the spend goal")
	}
	for _, p := range lg.Teams[3].Roster {
		p.Contract = Contract{Salary: 12000, Years: 2}
	}
	if met, _, _, _ := lg.ownerGoalState(&lg.Owner.Goals[1]); !met {
		t.Fatalf("payroll %d should meet a floor of %d", lg.Payroll(3), m.Target)
	}
	// A healthy payroll gets an ordinary money goal; a taxed one must get under the tax.
	lg2 := GenerateLeague(2)
	lg2.UserTeamID = 3
	for _, p := range lg2.Teams[3].Roster {
		p.Contract = Contract{Salary: 15000, Years: 2}
	}
	lg2.StartSeason()
	if k := lg2.Owner.Goals[1].Key; k != "tax" || lg2.Owner.Goals[1].Title != "Get under the luxury tax" {
		t.Fatalf("taxed team goal: %+v", lg2.Owner.Goals[1])
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestOwnerRebuildGoalNeedsRealProspects(t *testing.T) {
	lg := ownerLeague(2, 3)
	tm := lg.Teams[3]
	g := &OwnerGoal{Key: "rebuild", Kind: "building"}
	for _, p := range tm.Roster {
		p.Age, p.Potential = 29, 50
		p.Ratings = Ratings{Inside: 55, Mid: 55, Three: 55, Playmaking: 55, Rebounding: 55, Defense: 55, Athleticism: 55}
	}
	lg.Season.Day = SeasonDays
	if met, _, _, _ := lg.ownerGoalState(g); met {
		t.Fatal("a roster of veterans must not meet the rebuild goal")
	}
	// Two young players who are not prospects (low overall, low potential).
	for _, p := range tm.Roster[:2] {
		p.Age, p.Potential = 21, 55
	}
	if met, _, _, text := lg.ownerGoalState(g); met || !containsStr(text, "62") {
		t.Fatalf("young scrubs are not prospects (text %q)", text)
	}
	tm.Roster[0].Potential = 80
	tm.Roster[1].Ratings = Ratings{Inside: 70, Mid: 70, Three: 70, Playmaking: 70, Rebounding: 70, Defense: 70, Athleticism: 70}
	if !lg.ownerOwnsFirst() {
		t.Skip("setup: no first-round pick owned")
	}
	if met, _, _, text := lg.ownerGoalState(g); !met {
		t.Fatalf("two real prospects and the pick should meet it: %s", text)
	}
}

func TestOwnerGoalStatusHonest(t *testing.T) {
	lg := ownerLeague(2, 3)
	lg.SimDays(10)
	for i := range lg.Owner.Goals {
		if _, st := lg.OwnerGoalStatus(i); st != "on_track" && st != "at_risk" {
			t.Fatalf("goal %d is %q on day 10; met and missed only come at season end", i, st)
		}
	}
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	for i := range lg.Owner.Goals {
		if _, st := lg.OwnerGoalStatus(i); st == "met" || st == "missed" {
			t.Fatalf("goal %d is %q before it was judged", i, st)
		}
	}
	lg.BeginOffseason()
	for i := range lg.Owner.Goals {
		_, st := lg.OwnerGoalStatus(i)
		if st != map[bool]string{true: "met", false: "missed"}[lg.Owner.Goals[i].Met] {
			t.Fatalf("goal %d: %q after judging", i, st)
		}
	}
}
