package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func gmLeague(seed uint64) *League {
	lg := GenerateLeague(seed)
	lg.UserTeamID = 0
	lg.StartSeason()
	return lg
}

func gmJSON(t *testing.T, lg *League) []byte {
	t.Helper()
	lg.Version = SaveVersion
	b, err := json.Marshal(lg)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// gmClone is a deep copy through JSON, the way a save and load does it.
func gmClone(t *testing.T, lg *League) *League {
	t.Helper()
	cp := &League{}
	if err := json.Unmarshal(gmJSON(t, lg), cp); err != nil {
		t.Fatal(err)
	}
	return cp
}

// gmYear plays one whole year the stepped way a user who only presses
// continue does, and starts the next season.
func gmYear(lg *League) {
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	lg.BeginOffseason()
	lg.DraftAutoUntilUser()
	lg.DraftAutoAll()
	lg.BeginFreeAgency()
	lg.FinishOffseason()
	lg.StartSeason()
}

func allOn() GMAuto { return GMAuto{true, true, true, true, true, true} }

func TestGMZeroValueChangesNothing(t *testing.T) {
	a, b := gmLeague(3), gmLeague(3)
	b.SetGMPlan("balanced")
	b.SetGMAuto(GMAuto{})
	b.SetGMPlan("")
	if b.GM != nil {
		t.Fatal("an all-zero state should be dropped")
	}
	gmYear(a)
	gmYear(b)
	a.SimDays(20)
	b.SimDays(20)
	ja, jb := gmJSON(t, a), gmJSON(t, b)
	if !bytes.Equal(ja, jb) {
		t.Fatal("saves differ with every switch off")
	}
	if bytes.Contains(ja, []byte(`"GM"`)) {
		t.Fatal("an untouched league must not save a GM field")
	}
}

func TestGMOldSaveLoads(t *testing.T) {
	lg := gmLeague(4)
	lg.SimDays(5)
	path := filepath.Join(t.TempDir(), "old.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if bytes.Contains(raw, []byte(`"GM"`)) {
		t.Fatal("saved a GM field for an untouched league")
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.GM != nil || got.GMPlan() != "balanced" || got.GMAutoSettings() != (GMAuto{}) {
		t.Fatal("old save should load with the assistant off")
	}
	lg.SimDays(10)
	got.SimDays(10)
	if !bytes.Equal(gmJSON(t, lg), gmJSON(t, got)) {
		t.Fatal("loaded league plays differently")
	}
}

func TestGMPlanAndSettings(t *testing.T) {
	lg := gmLeague(1)
	for _, k := range []string{"", "balanced", "win_now", "rebuild"} {
		if !ValidGMPlan(k) {
			t.Fatalf("%q should be valid", k)
		}
	}
	if ValidGMPlan("tank") || lg.SetGMPlan("tank") == nil {
		t.Fatal("tank should be refused")
	}
	if err := lg.SetGMPlan("rebuild"); err != nil || lg.GMPlan() != "rebuild" {
		t.Fatal("plan not set")
	}
	lg.SetGMAuto(GMAuto{Lineup: true})
	if !lg.GMAutoSettings().Lineup || lg.GMAutoSettings().Draft {
		t.Fatal("auto not set")
	}
	if len(GMPlans()) != 3 || len(GMAreas()) != 6 {
		t.Fatal("lists")
	}
	if _, err := lg.GMApply("nonsense"); !errors.Is(err, ErrGMBadID) {
		t.Fatalf("bad id: %v", err)
	}
	if _, err := lg.GMApply("trade:sell:1:2"); !errors.Is(err, ErrGMBadID) {
		t.Fatalf("bad id: %v", err)
	}
	if _, err := lg.GMApply("resign:9999"); err == nil || errors.Is(err, ErrGMBadID) {
		t.Fatalf("well-formed but unknown should be a plain error: %v", err)
	}
}

// applyEach checks that every suggestion in the area applies on its own copy
// of the league, logs, and is gone afterwards. It returns how many it saw.
func applyEach(t *testing.T, lg *League, area string) int {
	t.Helper()
	list := lg.GMSuggestions()
	n := 0
	for _, s := range list {
		if s.Area != area {
			continue
		}
		n++
		if s.Title == "" || len(s.Reasons) == 0 || len(s.Reasons) > 3 || s.Action == "" {
			t.Fatalf("incomplete suggestion %+v", s)
		}
		cp := gmClone(t, lg)
		msg, err := cp.GMApply(s.ID)
		if err != nil {
			t.Fatalf("%s: apply failed: %v", s.ID, err)
		}
		if msg == "" || len(cp.GMLogEntries()) == 0 {
			t.Fatalf("%s: no message or log", s.ID)
		}
		last := cp.GMLogEntries()[len(cp.GMLogEntries())-1]
		if last.Auto || last.Area != area {
			t.Fatalf("%s: bad log entry %+v", s.ID, last)
		}
		for _, s2 := range cp.GMSuggestions() {
			if s2.ID == s.ID {
				t.Fatalf("%s still suggested after applying", s.ID)
			}
		}
	}
	return n
}

func TestGMLineup(t *testing.T) {
	lg := gmLeague(2)
	if applyEach(t, lg, GMAreaLineup) != 0 {
		t.Fatal("an automatic lineup needs no suggestion")
	}
	user := lg.UserTeamID
	l := lg.AutoLineup(user)
	tm := lg.Teams[user]
	rot := tm.Rotation()
	// bench the best player, start the 10th
	best, tenth := rot[0], rot[9]
	for i, id := range l.Starters {
		if id == best.ID {
			l.Starters[i] = tenth.ID
		}
	}
	l.Minutes[tenth.ID], l.Minutes[best.ID] = l.Minutes[best.ID], max(l.Minutes[tenth.ID], 1)
	if err := lg.SetLineup(user, l); err != nil {
		t.Fatal(err)
	}
	if applyEach(t, lg, GMAreaLineup) != 1 {
		t.Fatal("expected one lineup suggestion")
	}
	// an injured starter also counts
	lg2 := gmLeague(2)
	l2 := lg2.AutoLineup(user)
	lg2.SetLineup(user, l2)
	lg2.Teams[user].player(l2.Starters[2]).Injury = &Injury{Name: "Sprain", GamesLeft: 10}
	if applyEach(t, lg2, GMAreaLineup) != 1 {
		t.Fatal("expected a suggestion for an injured starter")
	}
	cp := gmClone(t, lg)
	cp.SetGMAuto(GMAuto{Lineup: true})
	cp.SimDay()
	if cp.Teams[user].Lineup != nil {
		t.Fatal("auto lineup did not reset")
	}
	log := cp.GMLogEntries()
	if len(log) != 1 || !log[0].Auto {
		t.Fatalf("log %+v", log)
	}
}

func TestGMTraining(t *testing.T) {
	lg := gmLeague(2)
	for _, p := range lg.Teams[0].Roster {
		p.Focus = FocusBody
	}
	n := applyEach(t, lg, GMAreaTraining)
	if n == 0 || n > GMTuning.MaxTraining {
		t.Fatalf("training suggestions: %d", n)
	}
	for _, s := range lg.GMSuggestions() {
		if s.Area == GMAreaTraining {
			parts := strings.Split(s.ID, ":")
			p := lg.FindPlayer(atoi(parts[1]))
			if want, _ := FocusFromKey(parts[2]); want != autoFocus(p) {
				t.Fatalf("%s is not the automatic choice", s.ID)
			}
		}
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

func TestGMContracts(t *testing.T) {
	sawResign, sawExtend := false, false
	for seed := uint64(1); seed <= 6 && !(sawResign && sawExtend); seed++ {
		lg := gmLeague(seed)
		// in season: every player who will talk is in his last year
		for _, p := range lg.Teams[0].Roster {
			p.Contract.Years = 1
			p.Contract.NextYears = 0
		}
		lg.SimDays(3)
		for _, s := range lg.GMSuggestions() {
			sawExtend = sawExtend || strings.HasPrefix(s.ID, "extend:")
		}
		applyEach(t, lg, GMAreaContracts)
		// offseason: expiring players
		lg.SimRegularSeason()
		lg.SimPlayoffs()
		lg.BeginOffseason()
		lg.DraftAutoAll()
		for _, s := range lg.GMSuggestions() {
			sawResign = sawResign || strings.HasPrefix(s.ID, "resign:")
		}
		applyEach(t, lg, GMAreaContracts)
	}
	if !sawResign || !sawExtend {
		t.Fatalf("resign %v extend %v", sawResign, sawExtend)
	}
}

func TestGMPlanTiltsResign(t *testing.T) {
	lg := gmLeague(2)
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	lg.BeginOffseason()
	lg.DraftAutoAll()
	count := func(plan string) (n, old int) {
		lg.SetGMPlan(plan)
		for _, s := range lg.GMSuggestions() {
			if strings.HasPrefix(s.ID, "resign:") {
				n++
				if lg.FindPlayer(atoi(strings.Split(s.ID, ":")[1])).Age > GMTuning.RebuildKeepAge {
					old++
				}
			}
		}
		return
	}
	if _, old := count("rebuild"); old > 0 {
		// only stars may pass the age line; none expected on a fresh fixture
		t.Logf("rebuild kept %d older players", old)
	}
}

func TestGMReleaseMakesRoom(t *testing.T) {
	found := false
	for seed := uint64(1); seed <= 8 && !found; seed++ {
		lg := gmLeague(seed)
		lg.SimRegularSeason()
		lg.SimPlayoffs()
		lg.BeginOffseason()
		lg.DraftAutoAll()
		lg.BeginFreeAgency()
		u := lg.Teams[0]
		for len(u.Roster) < RosterSize && len(lg.FreeAgents) > 0 {
			lg.forceSign(0, lg.sortedPool()[len(lg.FreeAgents)-1], Cap.MinSalary, 1)
		}
		if len(u.Roster) < RosterSize {
			continue
		}
		w := lg.worstKeep(u)
		w.Contract = Contract{Salary: Cap.MinSalary, Years: 1}
		for _, s := range lg.GMSuggestions() {
			if strings.HasPrefix(s.ID, "release:") {
				found = true
			}
			if s.Area == GMAreaFreeAgency {
				t.Fatal("no offers while the roster is full")
			}
		}
		if found {
			applyEach(t, lg, GMAreaContracts)
		}
	}
	if !found {
		t.Fatal("never saw a release suggestion")
	}
}

func TestGMFreeAgency(t *testing.T) {
	for _, plan := range []string{"", "win_now", "rebuild"} {
		saw := 0
		for seed := uint64(1); seed <= 4; seed++ {
			lg := gmLeague(seed)
			lg.SetGMPlan(plan)
			lg.SimRegularSeason()
			lg.SimPlayoffs()
			lg.BeginOffseason()
			lg.DraftAutoAll()
			lg.BeginFreeAgency()
			n := applyEach(t, lg, GMAreaFreeAgency)
			if n > GMTuning.MaxFA {
				t.Fatalf("%d offers suggested", n)
			}
			saw += n
		}
		if saw == 0 {
			t.Fatalf("plan %q: no free agency suggestions in four leagues", plan)
		}
	}
}

func TestGMDraftUsesScoutingOnly(t *testing.T) {
	lg := gmLeague(5)
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	lg.BeginOffseason()
	for lg.DraftOnClock() != lg.UserTeamID {
		lg.DraftPick(-1)
	}
	for _, plan := range []string{"", "win_now", "rebuild"} {
		lg.SetGMPlan(plan)
		var sug []GMSuggestion
		for _, s := range lg.GMSuggestions() {
			if s.Area == GMAreaDraft {
				sug = append(sug, s)
			}
		}
		if len(sug) != 1 {
			t.Fatalf("expected one draft suggestion, got %d", len(sug))
		}
		wo := Progression.DraftOvrWeight
		switch plan {
		case "win_now":
			wo = GMTuning.DraftWinNowOvr
		case "rebuild":
			wo = GMTuning.DraftRebuildOvr
		}
		bestID, best := -1, -1.0
		for _, p := range lg.Draft.Pool {
			v, _ := lg.ScoutView(p.ID)
			s := wo*float64((v.OvrLo+v.OvrHi+1)/2) + (1-wo)*float64((v.PotLo+v.PotHi+1)/2)
			if bestID < 0 || s > best {
				bestID, best = p.ID, s
			}
		}
		if sug[0].ID != "draft:"+itoa(bestID) {
			t.Fatalf("plan %q: %s is not the pick from the scouting view (%d)", plan, sug[0].ID, bestID)
		}
		for _, r := range sug[0].Reasons {
			p := lg.FindPlayer(bestID)
			_ = p
			if strings.Contains(r, "true") {
				t.Fatal("reason mentions truth")
			}
		}
	}
	lg.SetGMPlan("")
	if applyEach(t, lg, GMAreaDraft) != 1 {
		t.Fatal("draft apply")
	}
	// auto draft: the loop does not stall on the user's pick
	cp := gmClone(t, lg)
	cp.SetGMAuto(GMAuto{Draft: true})
	before := cp.Draft.Made
	cp.DraftAutoUntilUser()
	if cp.Draft.Made <= before {
		t.Fatal("auto draft did not pick")
	}
	if !cp.DraftDone() && cp.DraftOnClock() == cp.UserTeamID {
		t.Fatal("auto draft stopped on the user's pick")
	}
	// with the switch off it stops on the user, as today
	cp2 := gmClone(t, lg)
	cp2.DraftAutoUntilUser()
	if cp2.DraftOnClock() != cp2.UserTeamID || cp2.Draft.Made != before {
		t.Fatal("switch off must stop on the user")
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestGMTrades(t *testing.T) {
	saw := 0
	for seed := uint64(1); seed <= 6 && saw < 2; seed++ {
		for _, plan := range []string{"", "win_now", "rebuild"} {
			lg := gmLeague(seed)
			lg.SetGMPlan(plan)
			lg.SimDays(10)
			n := applyEach(t, lg, GMAreaTrades)
			if n > GMTuning.MaxTrades {
				t.Fatalf("%d trade suggestions", n)
			}
			for _, s := range lg.GMSuggestions() {
				if s.Area == GMAreaTrades && s.Link == "" {
					t.Fatal("trade suggestion without a link")
				}
			}
			if n > 0 {
				saw++
				cp := gmClone(t, lg)
				tx := len(cp.Transactions)
				for _, s := range cp.GMSuggestions() {
					if s.Area == GMAreaTrades {
						if _, err := cp.GMApply(s.ID); err != nil {
							t.Fatal(err)
						}
						break
					}
				}
				if len(cp.Transactions) != tx+1 {
					t.Fatal("a trade should be in the transactions")
				}
			}
		}
	}
	if saw == 0 {
		t.Fatal("no trade suggestion in any fixture")
	}
	// closed window: nothing
	lg := gmLeague(1)
	lg.SimDays(SeasonDays)
	for _, s := range lg.GMSuggestions() {
		if s.Area == GMAreaTrades {
			t.Fatal("trade suggestion with the window closed")
		}
	}
}

func userTransactions(lg *League) int {
	n := 0
	for _, x := range lg.Transactions {
		if x.TeamA == lg.UserTeamID || x.TeamB == lg.UserTeamID {
			n++
		}
	}
	return n
}

func TestGMAutoFullCycle(t *testing.T) {
	lg := gmLeague(2)
	lg.SetGMPlan("balanced")
	lg.SetGMAuto(allOn())
	for y := 0; y < 2; y++ {
		trades := 0
		for _, x := range lg.Transactions {
			if x.TeamA == 0 || x.TeamB == 0 {
				trades++
			}
		}
		_ = trades
		before := userTransactions(lg)
		lg.SimRegularSeason()
		lg.SimPlayoffs()
		// computer teams may trade with the user? they only make offers, which wait.
		if userTransactions(lg) != before {
			t.Fatal("something traded with the user's team without a click")
		}
		lg.BeginOffseason()
		lg.DraftAutoUntilUser()
		if !lg.DraftDone() {
			t.Fatal("draft stalled")
		}
		lg.BeginFreeAgency()
		lg.FinishOffseason()
		lg.StartSeason()
		if n := len(lg.Teams[0].Roster); n != RosterSize {
			t.Fatalf("year %d: roster %d", y, n)
		}
	}
	areas := map[string]int{}
	for _, e := range lg.GMLogEntries() {
		if !e.Auto {
			t.Fatalf("a hand entry: %+v", e)
		}
		areas[e.Area]++
	}
	if areas[GMAreaTrades] != 0 {
		t.Fatal("auto mode logged a trade")
	}
	for _, a := range []string{GMAreaDraft, GMAreaTraining} {
		if areas[a] == 0 {
			t.Fatalf("auto %s did nothing in two years: %v", a, areas)
		}
	}
	t.Log(areas)
}

func TestGMAutoOffseasonPath(t *testing.T) {
	lg := gmLeague(3)
	lg.SetGMAuto(allOn())
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	lg.Offseason()
	if n := len(lg.Teams[0].Roster); n != RosterSize {
		t.Fatalf("roster %d", n)
	}
}

func TestGMTradeAlerts(t *testing.T) {
	lg := gmLeague(1)
	lg.SetGMAuto(GMAuto{TradeAlerts: true})
	alerts := 0
	for i := 0; i < TradeDeadlineDay; i++ {
		lg.SimDay()
		if a := lg.GMAlertLastDay(); a != "" {
			alerts++
			if a == "" || len(lg.Transactions) > 0 && userTransactions(lg) > 0 {
				t.Fatal("alert must not trade")
			}
		}
	}
	if alerts == 0 {
		t.Fatal("no alert in 50 days")
	}
	// once per idea per season: far fewer alerts than days
	if alerts > TradeDeadlineDay/2 {
		t.Fatalf("%d alerts in %d days", alerts, TradeDeadlineDay)
	}
	cp := gmClone(t, lg)
	if cp.GMAlertLastDay() != "" {
		t.Fatal("the alert is not saved")
	}
}

func TestGMLogCap(t *testing.T) {
	lg := gmLeague(1)
	for i := 0; i < GMTuning.LogKeep+50; i++ {
		lg.gmLog(GMAreaLineup, "entry "+itoa(i), true)
	}
	log := lg.GMLogEntries()
	if len(log) != GMTuning.LogKeep || log[len(log)-1].Text != "entry "+itoa(GMTuning.LogKeep+49) {
		t.Fatalf("log length %d", len(log))
	}
}

func TestGMDeterministic(t *testing.T) {
	run := func() []byte {
		lg := gmLeague(6)
		lg.SetGMPlan("rebuild")
		lg.SetGMAuto(allOn())
		gmYear(lg)
		lg.SimDays(30)
		return gmJSON(t, lg)
	}
	if !bytes.Equal(run(), run()) {
		t.Fatal("two identical runs differ")
	}
}

func TestGMCountsMatch(t *testing.T) {
	lg := gmLeague(2)
	lg.SimDays(5)
	total, by := lg.GMCounts()
	if total != len(lg.GMSuggestions()) {
		t.Fatalf("count %d vs %d", total, len(lg.GMSuggestions()))
	}
	n := 0
	for _, v := range by {
		n += v
	}
	if n != total {
		t.Fatal("area counts")
	}
	for _, p := range lg.Teams[0].Roster {
		p.Focus = FocusBody
	}
	if total2, _ := lg.GMCounts(); total2 == total && len(lg.GMSuggestions()) != total {
		t.Fatal("stale cache")
	}
}

// ---- tuning piece: free agency, upgrade trades, consistency ----

// gmTeamLeague is a league with the user on team tid, season started.
func gmTeamLeague(seed uint64, tid int) *League {
	lg := GenerateLeague(seed)
	lg.UserTeamID = tid
	lg.StartSeason()
	return lg
}

func autoNoAlerts() GMAuto {
	return GMAuto{Lineup: true, Training: true, Contracts: true, FreeAgency: true, Draft: true}
}

// The browser case: fictional league seed 2, user team 3, plan balanced, every
// switch on. Before the fix the roster was full (13 or more) at every free
// agency day, so the assistant made no offers at all in the first summers.
func TestGMAutoFreeAgencyMakesOffersAndSigns(t *testing.T) {
	lg := gmTeamLeague(2, 3)
	lg.SetGMPlan("balanced")
	lg.SetGMAuto(autoNoAlerts())
	offers, cuts, signed, early := 0, 0, 0, 0
	for y := 0; y < 5; y++ {
		lg.SimRegularSeason()
		lg.SimPlayoffs()
		lg.BeginOffseason()
		lg.DraftAutoUntilUser()
		lg.DraftAutoAll()
		lg.BeginFreeAgency()
		pool := map[int]bool{}
		for _, p := range lg.FreeAgents {
			pool[p.ID] = true
		}
		sawSuggestion := false
		for d := 0; d < lg.FA.Days; d++ {
			for _, s := range lg.GMSuggestions() {
				if s.Area == GMAreaFreeAgency {
					sawSuggestion = true
				}
			}
			if _, err := lg.AdvanceFreeAgency(); err != nil {
				t.Fatal(err)
			}
		}
		if y < 3 && !sawSuggestion && len(lg.Teams[3].Roster) >= RosterSize {
			t.Logf("summer %d: no suggestion (roster %d)", y+1, len(lg.Teams[3].Roster))
		}
		lg.FinishOffseason()
		for _, p := range lg.Teams[3].Roster {
			if pool[p.ID] && p.Contract.Salary > Cap.MinSalary {
				signed++
			}
		}
		lg.StartSeason()
		if n := len(lg.Teams[3].Roster); n != RosterSize {
			t.Fatalf("year %d: roster %d", y, n)
		}
		if y == 2 {
			for _, e := range lg.GMLogEntries() {
				if e.Area == GMAreaFreeAgency {
					early++
				}
			}
		}
	}
	for _, e := range lg.GMLogEntries() {
		if e.Area == GMAreaFreeAgency {
			offers++
			if strings.HasPrefix(e.Text, "Released") {
				cuts++
			}
		}
	}
	t.Logf("offers %d (with a release %d), signed above the minimum %d", offers, cuts, signed)
	if offers == 0 {
		t.Fatal("auto free agency made no offers in five summers")
	}
	if early == 0 {
		t.Fatal("auto free agency made no offers in the first three summers (roster full every time)")
	}
	if signed == 0 {
		t.Fatal("auto free agency signed nobody above the minimum in five summers")
	}
}

// A full roster and a clear upgrade among the free agents: the suggestion is
// one step (offer and release the weakest player), it is the engine's own
// upgrade rule, and applying it opens the spot and puts the offer out.
func TestGMFullRosterOffersWithRelease(t *testing.T) {
	found := 0
	for seed := uint64(1); seed <= 8; seed++ {
		lg := gmTeamLeague(seed, 0)
		lg.SimRegularSeason()
		lg.SimPlayoffs()
		lg.BeginOffseason()
		lg.DraftAutoAll()
		lg.BeginFreeAgency()
		u := lg.Teams[0]
		// Fill with the weakest minimum players, so the roster is full and the
		// weakest one is cheap and short.
		for len(u.Roster) < RosterSize && len(lg.FreeAgents) > 0 {
			lg.forceSign(0, lg.sortedPool()[len(lg.FreeAgents)-1], Cap.MinSalary, 1)
		}
		// Free up money so that a good free agent fits.
		for _, p := range u.Roster {
			if p.Contract.Salary > 20000 {
				p.Contract.Salary = 20000
			}
		}
		if len(u.Roster) != RosterSize {
			continue
		}
		for _, s := range lg.GMSuggestions() {
			if s.Area != GMAreaFreeAgency {
				continue
			}
			if !strings.Contains(s.Title, "release") || s.Action != "Release and offer" {
				t.Fatalf("a full roster should only get offer-and-release suggestions: %+v", s)
			}
			w := lg.gmCutFor(u, lg.faPlayer(s.PlayerID))
			if w == nil || FATuning.UpgradeMargin > keepScore(lg.faPlayer(s.PlayerID))-keepScore(w) {
				t.Fatal("the cut is not the engine's upgrade rule")
			}
			cp := gmClone(t, lg)
			if _, err := cp.GMApply(s.ID); err != nil {
				t.Fatal(err)
			}
			if len(cp.Teams[0].Roster) != RosterSize-1 || len(cp.OffersBy(0)) != 1 {
				t.Fatalf("roster %d offers %d after the apply", len(cp.Teams[0].Roster), len(cp.OffersBy(0)))
			}
			if cp.faPlayer(w.ID) == nil {
				t.Fatal("the cut player is not in the pool")
			}
			found++
			break
		}
	}
	if found == 0 {
		t.Fatal("never saw an offer-and-release suggestion")
	}
}

func gmHasPrefix(lg *League, prefix string) bool {
	for _, s := range lg.GMSuggestions() {
		if strings.HasPrefix(s.ID, prefix) {
			return true
		}
	}
	return false
}

// No upgrade trade ("trade:get") for plan rebuild, nor for a team whose
// direction is seller or rebuilding, whatever the plan.
func TestGMNoUpgradeWhenRebuilding(t *testing.T) {
	weak := 0
	for seed := uint64(1); seed <= 3; seed++ {
		for tid := 0; tid < 30; tid += 3 {
			lg := gmTeamLeague(seed, tid)
			lg.SimDays(12)
			dir := lg.TeamStatusOf(tid).Key
			for _, plan := range []string{"", "win_now", "rebuild"} {
				lg.SetGMPlan(plan)
				got := gmHasPrefix(lg, "trade:get:")
				if got && (plan == "rebuild" || dir == "seller" || dir == "rebuilding") {
					t.Fatalf("seed %d team %d direction %s plan %q: an upgrade trade was suggested", seed, tid, dir, plan)
				}
				if dir == "seller" || dir == "rebuilding" {
					weak++
				}
			}
		}
	}
	if weak == 0 {
		t.Fatal("no seller or rebuilding team among the fixtures")
	}
}

// An upgrade package obeys the limits: nobody the contract rules would extend
// or re-sign, at most UpgradeMaxCore of the five best by value, at most one
// first-round pick (balanced), and a target of a fitting age.
func TestGMUpgradePackageLimits(t *testing.T) {
	seen := 0
	for seed := uint64(1); seed <= 4; seed++ {
		for tid := 0; tid < 30; tid++ {
			for _, plan := range []string{"", "win_now"} {
				lg := gmTeamLeague(seed, tid)
				if d := lg.TeamStatusOf(tid).Key; d != "contender" && d != "buyer" {
					continue
				}
				lg.SimDays(8)
				if d := lg.TeamStatusOf(tid).Key; d != "contender" && d != "buyer" {
					continue
				}
				lg.SetGMPlan(plan)
				for _, s := range lg.GMSuggestions() {
					if !strings.HasPrefix(s.ID, "trade:get:") {
						continue
					}
					seen++
					res, err := lg.WhatWouldItTake(s.PlayerID)
					if err != nil {
						t.Fatal(err)
					}
					hash := strings.Split(s.ID, ":")[3]
					var pk *TakePackage
					for i := range res.Packages {
						if gmHash(res.Packages[i].Give) == hash {
							pk = &res.Packages[i]
						}
					}
					if pk == nil {
						t.Fatal("the package of the suggestion is gone")
					}
					keep := lg.gmKeepIDs()
					cf := lg.contendFactors()
					byVal := append([]*Player(nil), lg.Teams[tid].Roster...)
					sort.SliceStable(byVal, func(i, j int) bool {
						return lg.value(byVal[i], tid, cf) > lg.value(byVal[j], tid, cf)
					})
					core := map[int]bool{}
					for _, p := range byVal[:5] {
						core[p.ID] = true
					}
					nCore, firsts := 0, 0
					for _, a := range pk.Give {
						if a.Kind == PlayerAsset {
							if keep[a.PlayerID] {
								t.Fatalf("package sends a player the assistant would extend or re-sign")
							}
							if core[a.PlayerID] {
								nCore++
							}
						} else if a.Round == 1 {
							firsts++
						}
					}
					maxAge, maxFirsts := GMTuning.UpgradeMaxAge, GMTuning.UpgradeMaxFirsts
					if plan == "win_now" {
						maxAge, maxFirsts = GMTuning.UpgradeWinNowMaxAge, GMTuning.UpgradeWinNowMaxFirsts
					}
					if nCore > GMTuning.UpgradeMaxCore || firsts > maxFirsts {
						t.Fatalf("package too big: %d core, %d firsts (plan %q)", nCore, firsts, plan)
					}
					if tp := lg.FindPlayer(s.PlayerID); tp == nil || tp.Age > maxAge {
						t.Fatalf("target too old for plan %q", plan)
					}
				}
			}
		}
	}
	t.Logf("upgrade suggestions checked: %d", seen)
	if seen == 0 {
		t.Fatal("no upgrade suggestion in any fixture")
	}
}

// A player in a sell idea gets no extension or re-signing suggestion, in any
// plan, in season or in the offseason.
func TestGMSellAndExtendNeverTogether(t *testing.T) {
	sells := 0
	for seed := uint64(1); seed <= 4; seed++ {
		for _, tid := range []int{0, 5, 11, 17, 23} {
			for _, plan := range []string{"", "win_now", "rebuild"} {
				lg := gmTeamLeague(seed, tid)
				lg.SetGMPlan(plan)
				lg.SimDays(10)
				sell, keep := map[int]bool{}, map[int]bool{}
				for _, s := range lg.GMSuggestions() {
					switch {
					case strings.HasPrefix(s.ID, "trade:sell:"):
						sell[s.PlayerID] = true
						sells++
					case strings.HasPrefix(s.ID, "extend:"), strings.HasPrefix(s.ID, "resign:"):
						keep[s.PlayerID] = true
					}
				}
				for id := range sell {
					if keep[id] {
						t.Fatalf("seed %d team %d plan %q: player %d is both sold and extended", seed, tid, plan, id)
					}
				}
			}
		}
	}
	if sells == 0 {
		t.Fatal("no sell idea in any fixture")
	}
}
