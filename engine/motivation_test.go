package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withMotivation gives p an ID whose motivations match want (a test helper:
// motivations come from the ID, so this picks one).
func withMotivation(lg *League, p *Player, want Motivation) {
	for id := 500000; ; id++ {
		p.ID = id
		if lg.MotivationOf(p) == want {
			return
		}
	}
}

func allLevel(l Level) Motivation { return Motivation{l, l, l, l} }

// bestWorst returns the team IDs with the best and worst direction.
func bestWorst(lg *League) (best, worst int) {
	cf := lg.contendFactors()
	for i, c := range cf {
		if c > cf[best] {
			best = i
		}
		if c < cf[worst] {
			worst = i
		}
	}
	return
}

func TestMotivationDeterministicAndSpread(t *testing.T) {
	lg := GenerateLeague(3)
	other := GenerateLeague(4)
	var count [4][3]int
	diff := 0
	for id := 1; id <= 3000; id++ {
		p := &Player{ID: id}
		a, b := lg.MotivationOf(p), lg.MotivationOf(p)
		if a != b {
			t.Fatal("not deterministic")
		}
		if a != other.MotivationOf(p) {
			diff++
		}
		for i, l := range []Level{a.Money, a.Winning, a.Role, a.Loyalty} {
			count[i][l]++
		}
	}
	if diff < 1000 {
		t.Errorf("different seeds gave the same motivation for %d of 3000 players", 3000-diff)
	}
	for i := range count {
		lo, mid, hi := float64(count[i][Low])/3000, float64(count[i][Medium])/3000, float64(count[i][High])/3000
		t.Logf("trait %d: low %.2f medium %.2f high %.2f", i, lo, mid, hi)
		if lo < 0.2 || lo > 0.3 || hi < 0.2 || hi > 0.3 || mid < 0.44 || mid > 0.56 {
			t.Errorf("trait %d spread %.2f/%.2f/%.2f", i, lo, mid, hi)
		}
	}
	if High.String() != "High" || Low.String() != "Low" || Medium.String() != "Medium" {
		t.Error("level names")
	}
}

func TestStanceKeys(t *testing.T) {
	lg := GenerateLeague(5)
	lg.UserTeamID = 0
	best, worst := bestWorst(lg)

	// A star on the best team who cares about winning, role and loyalty re-signs.
	star := lg.Teams[best].Roster[0]
	for _, p := range lg.Teams[best].Roster {
		if p.Overall() > star.Overall() {
			star = p
		}
	}
	star.Ratings = Ratings{85, 85, 85, 85, 85, 85, 85}
	star.TeamSince = lg.Year - 6
	withMotivation(lg, star, allLevel(High))
	if st := lg.StanceOf(star); st.Key != "resign" || st.Label != "Will re-sign" || len(st.Reasons) == 0 || len(st.Reasons) > 3 {
		t.Errorf("star on best team: %+v", st)
	}

	// The weakest player on the worst team who wants a role and wins tests.
	tm := lg.Teams[worst]
	scrub := tm.Roster[0]
	for _, p := range tm.Roster {
		if p.Overall() < scrub.Overall() {
			scrub = p
		}
	}
	scrub.Ratings = Ratings{40, 40, 40, 40, 40, 40, 40}
	withMotivation(lg, scrub, allLevel(High))
	st := lg.StanceOf(scrub)
	if st.Key != "test" || st.Label != "Will test free agency" {
		t.Errorf("scrub on worst team: %+v", st)
	}
	joined := strings.Join(st.Reasons, "|")
	if !strings.Contains(joined, "bigger role") {
		t.Errorf("reasons %q", joined)
	}

	// Money: a big market value the team cannot pay shows up in the reasons.
	star2 := lg.Teams[best].Roster[1]
	star2.Ratings = Ratings{90, 90, 90, 90, 90, 90, 90}
	star2.Age = 27
	withMotivation(lg, star2, Motivation{Money: High, Winning: Medium, Role: Medium, Loyalty: Medium})
	star2.Contract = Contract{Salary: 20000, Years: 1}
	for _, p := range lg.Teams[best].Roster {
		if p != star2 {
			p.Contract = Contract{Salary: Cap.TaxLine / 12, Years: 2}
		}
	}
	found := false
	for _, r := range lg.stanceReasonsAll(star2) {
		if strings.Contains(r, "pay") {
			found = true
		}
	}
	if !found {
		t.Error("no money reason for a player the team cannot pay")
	}

	// A player with no team is neutral.
	free := testPlayer(70, 27, 70)
	if st := lg.StanceOf(free); st.Key != "open" {
		t.Errorf("free agent stance %+v", st)
	}
	// Unknown tenure (0) is neutral: no tenure sentence.
	star.TeamSince = 0
	for _, r := range lg.StanceOf(star).Reasons {
		if strings.Contains(r, "been here") || strings.Contains(r, "arrived") {
			t.Errorf("tenure reason with unknown tenure: %q", r)
		}
	}
}

// stanceReasonsAll is every reason (not cut to three): a test view.
func (lg *League) stanceReasonsAll(p *Player) []string {
	saved := MotivationTuning
	defer func() { MotivationTuning = saved }()
	// make the verdict side irrelevant: the list is cut to three, and there
	// are at most four parts, so check by lowering the other parts to zero
	MotivationTuning.StarterPts, MotivationTuning.BenchPts, MotivationTuning.ShortPts = 0, 0, 0
	MotivationTuning.ContendPts, MotivationTuning.BuyerPts, MotivationTuning.SellerPts, MotivationTuning.RebuildPts = 0, 0, 0, 0
	return lg.StanceOf(p).Reasons
}

// extensionFixture returns a league and a user-team star in his last year
// whose stance is not "test".
func extensionFixture(t *testing.T, seed uint64) (*League, *Player) {
	lg := GenerateLeague(seed)
	best, _ := bestWorst(lg)
	lg.UserTeamID = best
	star := lg.Teams[best].Roster[0]
	for _, p := range lg.Teams[best].Roster {
		if p.Overall() > star.Overall() {
			star = p
		}
	}
	star.Ratings = Ratings{78, 78, 78, 78, 78, 78, 78}
	star.Age = 26
	star.Potential = 78
	star.TeamSince = lg.Year - 5
	withMotivation(lg, star, allLevel(High))
	star.Contract = Contract{Salary: MarketSalary(star) * 9 / 10 / 100 * 100, Years: 1}
	if lg.StanceOf(star).Key != "resign" {
		t.Fatalf("fixture stance %+v", lg.StanceOf(star))
	}
	return lg, star
}

func TestExtensionAskAndRefusals(t *testing.T) {
	lg, star := extensionFixture(t, 11)
	u := lg.UserTeamID
	sal, years, err := lg.ExtensionAsk(u, star.ID)
	if err != nil || sal != MarketSalary(star) || years != Cap.MaxContractYears {
		t.Fatalf("ask %d %d %v (market %d)", sal, years, err, MarketSalary(star))
	}

	// "open" costs 10% more.
	saved := MotivationTuning
	MotivationTuning.ResignAt = 99 // everyone below "resign": open
	sal2, _, err := lg.ExtensionAsk(u, star.ID)
	MotivationTuning = saved
	if err != nil || sal2 <= sal || sal2 > sal*115/100 {
		t.Errorf("open ask %d vs %d, %v", sal2, sal, err)
	}

	// Not on the team, not the last year, already extended, free agency.
	if _, _, err := lg.ExtensionAsk(u, -5); err == nil {
		t.Error("extension for a stranger")
	}
	star.Contract.Years = 2
	if _, _, err := lg.ExtensionAsk(u, star.ID); err == nil || !strings.Contains(err.Error(), "last year") {
		t.Errorf("not last year: %v", err)
	}
	star.Contract.Years = 1
	lg.FA = &FreeAgency{}
	if _, _, err := lg.ExtensionAsk(u, star.ID); err == nil || !strings.Contains(err.Error(), "Free agency") {
		t.Errorf("during free agency: %v", err)
	}
	lg.FA = nil

	// More than 140% of his salary.
	star.Contract.Salary = MarketSalary(star) * 6 / 10 / 100 * 100
	if _, _, err := lg.ExtensionAsk(u, star.ID); err == nil || !strings.Contains(err.Error(), "140%") {
		t.Errorf("over 140%%: %v", err)
	}
	star.Contract.Salary = MarketSalary(star) * 9 / 10 / 100 * 100

	// A player who will test free agency.
	tester := testPlayer(40, 27, 40)
	tester.TeamID = u
	tester.Contract = Contract{Salary: Cap.MinSalary, Years: 1}
	tester.ID = 9999
	withMotivation(lg, tester, Motivation{Money: High, Winning: Low, Role: High, Loyalty: High})
	lg.Teams[u].Roster = append(lg.Teams[u].Roster, tester)
	for _, p := range lg.Teams[u].Roster {
		p.Contract.Years = max(p.Contract.Years, 1)
	}
	if lg.StanceOf(tester).Key != "test" {
		t.Fatalf("tester stance %+v", lg.StanceOf(tester))
	}
	if _, _, err := lg.ExtensionAsk(u, tester.ID); err == nil || err.Error() != "He wants to test free agency." {
		t.Errorf("test stance: %v", err)
	}
}

func TestExtendValidation(t *testing.T) {
	lg, star := extensionFixture(t, 12)
	u := lg.UserTeamID
	ask, _, _ := lg.ExtensionAsk(u, star.ID)
	if err := lg.Extend(u, star.ID, ask-100, 3); err == nil {
		t.Error("extended below his ask")
	}
	if err := lg.Extend(u, star.ID, ask, 0); err == nil {
		t.Error("extended for 0 years")
	}
	if err := lg.Extend(u, star.ID, ask, Cap.MaxContractYears+1); err == nil {
		t.Error("extended too long")
	}
	if err := lg.Extend(u, star.ID, lg.ExtensionLimit(star)+100, 3); err == nil {
		t.Error("extended above 140%")
	}
	if err := lg.Extend(u, star.ID, ask, 3); err != nil {
		t.Fatal(err)
	}
	if star.Contract.NextSalary != ask || star.Contract.NextYears != 3 || star.Contract.Years != 1 {
		t.Errorf("contract %+v", star.Contract)
	}
	if err := lg.Extend(u, star.ID, ask, 3); err == nil {
		t.Error("extended twice")
	}
	// Hard ceiling on next season's payroll.
	lg2, s2 := extensionFixture(t, 13)
	for _, p := range lg2.Teams[lg2.UserTeamID].Roster {
		if p != s2 {
			p.Contract = Contract{Salary: Cap.HardCeiling / 8, Years: 3}
		}
	}
	a2, _, _ := lg2.ExtensionAsk(lg2.UserTeamID, s2.ID)
	if err := lg2.Extend(lg2.UserTeamID, s2.ID, a2, 2); err == nil || !strings.Contains(err.Error(), "hard ceiling") {
		t.Errorf("ceiling: %v", err)
	}
}

func TestExtensionTakesOverInOffseason(t *testing.T) {
	lg, star := extensionFixture(t, 14)
	u := lg.UserTeamID
	ask, _, _ := lg.ExtensionAsk(u, star.ID)
	if err := lg.Extend(u, star.ID, ask, 3); err != nil {
		t.Fatal(err)
	}
	lg.StartSeason()
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	pay := lg.Payroll(u)
	lg.BeginOffseason()
	if star.TeamID != u {
		t.Skip("star retired or moved") // age 26 makes this very unlikely
	}
	if star.Contract.Salary != ask || star.Contract.Years != 3 || star.Contract.NextYears != 0 || star.Contract.NextSalary != 0 {
		t.Fatalf("after offseason begins: %+v", star.Contract)
	}
	for _, p := range lg.Expiring(u) {
		if p == star {
			t.Error("extended player is expiring")
		}
	}
	if lg.Payroll(u) < ask {
		t.Errorf("payroll %d (before %d) misses the extension", lg.Payroll(u), pay)
	}
}

func TestReSignRefusesTestStance(t *testing.T) {
	lg := GenerateLeague(15)
	lg.UserTeamID = 0
	lg.StartSeason()
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	lg.BeginOffseason()
	tm := lg.Teams[0]
	p := tm.Roster[len(tm.Roster)-1]
	p.Contract = Contract{}
	p.Ratings = Ratings{55, 55, 55, 55, 55, 55, 55}
	withMotivation(lg, p, allLevel(High))
	// make the team look bad for him: bench player, rebuilding team
	for i, q := range tm.Roster {
		if q != p {
			q.Ratings = Ratings{70 + i%3, 70, 70, 70, 70, 70, 70}
		}
	}
	if lg.StanceOf(p).Key != "test" {
		t.Skipf("fixture stance %+v", lg.StanceOf(p))
	}
	if err := lg.ReSign(0, p.ID); err == nil || err.Error() != "He wants to test free agency." {
		t.Errorf("ReSign: %v", err)
	}
	if lg.CanReSign(p) == nil {
		t.Error("CanReSign allows a test player")
	}
	// computer teams skip such players too
	lg.reSignTeam(tm, lg.contendFactors())
	if p.Contract.Years > 0 {
		t.Error("reSignTeam re-signed a test player")
	}
}

func TestTeamSinceSet(t *testing.T) {
	lg := GenerateLeague(16)
	lg.UserTeamID = 0
	lg.StartSeason()
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	lg.BeginOffseason()
	lg.DraftAutoAll()
	n := 0
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			if p.DraftYear == lg.Year && p.TeamSince == lg.Year {
				n++
			}
		}
	}
	if n == 0 {
		t.Error("no drafted rookie has TeamSince set")
	}
	a, b := lg.Teams[1].Roster[0], lg.Teams[2].Roster[0]
	lg.applyTrade(1, 2, []*Player{a}, []*Player{b})
	if a.TeamSince != lg.Year || b.TeamSince != lg.Year {
		t.Error("trade did not set TeamSince")
	}
}

func TestOldSaveWithoutMotivationFields(t *testing.T) {
	lg := GenerateLeague(17)
	path := filepath.Join(t.TempDir(), "league.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	for _, k := range []string{"TeamSince", "NextYears", "NextSalary", "\"Prev\""} {
		if strings.Contains(string(data), k) {
			t.Fatalf("a fresh league saves %s; zero values must be omitted", k)
		}
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	p := got.Teams[0].Roster[0]
	if p.TeamSince != 0 || p.Contract.NextYears != 0 {
		t.Error("old save did not load with zero fields")
	}
	if st := got.StanceOf(p); st.Key == "" {
		t.Error("no stance")
	}
}
