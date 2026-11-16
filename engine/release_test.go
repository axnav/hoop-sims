package engine

import "testing"

func seasonLeague(seed uint64) *League {
	lg := GenerateLeague(seed)
	lg.UserTeamID = 0
	lg.StartSeason()
	lg.SimDays(5)
	// a new league has no free agents
	fa := testPlayer(55, 27, 55)
	fa.ID, fa.Name = 90001, "Free Agent"
	lg.FreeAgents = append(lg.FreeAgents, fa)
	return lg
}

func TestReleaseInSeasonRefills(t *testing.T) {
	lg := seasonLeague(31)
	tm := lg.Teams[0]
	p := tm.Roster[0]
	p.Contract = Contract{Salary: 9000, Years: 2, NextSalary: 20000, NextYears: 3}
	before := lg.Payroll(0)
	pre := lg.ReleasePreview(0, p.ID)
	if !pre.Allowed || pre.DeadSalary != 9000 || pre.DeadYears != 2 || !pre.Refill || pre.RefillName == "" {
		t.Fatalf("preview: %+v", pre)
	}
	nTx := len(lg.Transactions)
	if err := lg.Release(0, p.ID); err != nil {
		t.Fatal(err)
	}
	if len(tm.Roster) != RosterSize {
		t.Fatalf("roster %d", len(tm.Roster))
	}
	if p.TeamID != -1 || p.Contract != (Contract{}) {
		t.Errorf("released player: %+v", p)
	}
	if len(tm.DeadMoney) != 1 || tm.DeadMoney[0] != (DeadContract{p.ID, p.Name, 9000, 2}) {
		t.Fatalf("dead money %+v (the extension must be cancelled)", tm.DeadMoney)
	}
	if got := lg.Payroll(0); got != before+Cap.MinSalary {
		t.Errorf("payroll %d, want %d", got, before+Cap.MinSalary)
	}
	if len(lg.Transactions) != nTx+1 {
		t.Fatal("no transaction logged")
	}
	if tx := lg.Transactions[nTx]; len(tx.Released) != 1 || len(tx.Signed) != 1 || tx.Signed[0].Name != pre.RefillName {
		t.Errorf("transaction %+v", tx)
	}
	if err := lg.Release(0, p.ID); err == nil {
		t.Error("released a player who is not on the team")
	}
	if err := lg.Release(0, lg.Teams[1].Roster[0].ID); err == nil {
		t.Error("released another team's player")
	}
}

func TestReleaseHardCeilingRefusal(t *testing.T) {
	lg := seasonLeague(31)
	tm := lg.Teams[0]
	p := tm.Roster[0]
	// push the payroll to just under the ceiling so even a minimum signing breaks it
	tm.Roster[1].Contract = Contract{Salary: 40000, Years: 2}
	tm.DeadMoney = []DeadContract{{PlayerID: -5, Name: "X", Salary: Cap.HardCeiling - lg.Payroll(0) - 500, Years: 1}}
	if lg.Payroll(0) != Cap.HardCeiling-500 {
		t.Fatalf("payroll %d", lg.Payroll(0))
	}
	pre := lg.ReleasePreview(0, p.ID)
	if pre.Allowed || pre.Reason == "" {
		t.Fatalf("preview should refuse: %+v", pre)
	}
	if err := lg.Release(0, p.ID); err == nil || len(tm.Roster) != RosterSize {
		t.Fatalf("release should be refused: %v", err)
	}
}

func TestReleaseOffseasonNoRefill(t *testing.T) {
	lg := draftDoneLeague(7)
	tm := lg.Teams[0]
	var p *Player
	for _, q := range tm.Roster {
		if q.Contract.Years >= 1 {
			p = q
			break
		}
	}
	n := len(tm.Roster)
	pre := lg.ReleasePreview(0, p.ID)
	if !pre.Allowed || pre.Refill {
		t.Fatalf("preview: %+v", pre)
	}
	pay := lg.Payroll(0)
	if err := lg.Release(0, p.ID); err != nil {
		t.Fatal(err)
	}
	if len(tm.Roster) != n-1 {
		t.Errorf("roster %d, want %d (no refill in the offseason)", len(tm.Roster), n-1)
	}
	if lg.Payroll(0) != pay {
		t.Errorf("payroll changed %d -> %d; dead money should replace the salary", pay, lg.Payroll(0))
	}
}

func TestReleaseExpiringLeavesNothing(t *testing.T) {
	lg := draftDoneLeague(7)
	tm := lg.Teams[0]
	p := tm.Roster[0]
	p.Contract.Years = 0
	pre := lg.ReleasePreview(0, p.ID)
	if pre.DeadSalary != 0 || pre.DeadYears != 0 {
		t.Errorf("preview %+v", pre)
	}
	if err := lg.Release(0, p.ID); err != nil {
		t.Fatal(err)
	}
	if len(tm.DeadMoney) != 0 {
		t.Errorf("dead money %+v", tm.DeadMoney)
	}
}

func TestDeadMoneyCountsDown(t *testing.T) {
	lg := GenerateLeague(33)
	lg.StartSeason()
	tm := lg.Teams[2]
	tm.DeadMoney = []DeadContract{{1, "A", 3000, 1}, {2, "B", 4000, 2}}
	base := lg.Payroll(2) - 7000
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	lg.BeginOffseason()
	if len(tm.DeadMoney) != 1 || tm.DeadMoney[0].Name != "B" || tm.DeadMoney[0].Years != 1 {
		t.Fatalf("after one season: %+v", tm.DeadMoney)
	}
	_ = base
	if lg.Payroll(2) < 4000 {
		t.Error("remaining dead money not in payroll")
	}
	lg.ageDeadMoney()
	if len(tm.DeadMoney) != 0 {
		t.Errorf("not removed: %+v", tm.DeadMoney)
	}
}

func TestComputerCutGetsDeadMoney(t *testing.T) {
	lg := GenerateLeague(35)
	lg.UserTeamID = 0
	lg.StartSeason()
	tm := lg.Teams[5]
	for _, q := range tm.Roster {
		q.Contract = Contract{Salary: 3000, Years: 2}
	}
	// a fourteenth player, so trimRoster must cut one
	extra := testPlayer(50, 25, 50)
	extra.ID, extra.Name = 90002, "Extra"
	lg.forceSign(5, extra, 2000, 2)
	lg.trimRoster(tm)
	if len(tm.Roster) != RosterSize {
		t.Fatalf("roster %d", len(tm.Roster))
	}
	if len(tm.DeadMoney) != 1 || tm.DeadMoney[0].Years < 1 {
		t.Fatalf("computer cut left %+v", tm.DeadMoney)
	}
}

func TestDeadMoneySaveRoundTrip(t *testing.T) {
	lg := GenerateLeague(36)
	lg.Teams[4].DeadMoney = []DeadContract{{9, "Z", 2500, 3}}
	path := t.TempDir() + "/s.json"
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if d := back.Teams[4].DeadMoney; len(d) != 1 || d[0] != (DeadContract{9, "Z", 2500, 3}) {
		t.Fatalf("round trip: %+v", d)
	}
	if back.Payroll(4) != lg.Payroll(4) {
		t.Error("payroll differs after load")
	}
}

func TestReleaseNoFreeAgentLeavesShort(t *testing.T) {
	lg := seasonLeague(31)
	lg.FreeAgents = nil
	p := lg.Teams[0].Roster[0]
	pre := lg.ReleasePreview(0, p.ID)
	if !pre.Allowed || pre.Refill || !pre.Short {
		t.Fatalf("preview: %+v", pre)
	}
	if err := lg.Release(0, p.ID); err != nil || len(lg.Teams[0].Roster) != RosterSize-1 {
		t.Fatalf("release: %v, roster %d", err, len(lg.Teams[0].Roster))
	}
}
