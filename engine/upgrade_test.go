package engine

import (
	"bytes"
	"testing"
	"time"
)

// upgradeLeague is a fictional league, user team 3, ten days into the season.
func upgradeLeague(seed uint64) *League {
	lg := GenerateLeague(seed)
	lg.UserTeamID = 3
	lg.Season = lg.StartSeason()
	lg.SimDays(10)
	return lg
}

func TestFindUpgradesPackagesAreAccepted(t *testing.T) {
	user := 3
	total, possible := 0, 0
	for seed := uint64(1); seed <= 3; seed++ {
		lg := upgradeLeague(seed)
		ps := userBest(lg)
		for _, p := range ps[:5] {
			res, err := lg.FindUpgrades(p.ID)
			if err != nil || !res.Possible || res.Player != p || res.Upgrades == nil {
				t.Fatalf("seed %d %s: %v %+v", seed, p.Name, err, res)
			}
			if len(res.Upgrades) > UpgradeTuning.MaxCandidates {
				t.Fatalf("%d candidates", len(res.Upgrades))
			}
			var prev *Upgrade
			for i := range res.Upgrades {
				u := &res.Upgrades[i]
				total++
				if u.Team == user || u.Player.TeamID != u.Team || u.Gain != u.Player.Overall()-p.Overall() ||
					u.Gain < UpgradeTuning.MinGain || !u.Player.CanPlay(p.Pos) {
					t.Fatalf("bad candidate %+v for %s", u, p.Name)
				}
				if u.Player.Injury != nil && u.Player.Injury.GamesLeft > UpgradeTuning.MaxInjuryDays {
					t.Fatalf("injured candidate %s", u.Player.Name)
				}
				if u.Shopped && u.ShopReason == "" {
					t.Fatalf("shopped without reason: %+v", u)
				}
				if prev != nil { // possible first, then shopped, then gain, then id
					bad := false
					switch {
					case prev.Possible != u.Possible:
						bad = !prev.Possible
					case prev.Shopped != u.Shopped:
						bad = !prev.Shopped
					case prev.Gain != u.Gain:
						bad = prev.Gain < u.Gain
					default:
						bad = prev.Player.ID > u.Player.ID
					}
					if bad {
						t.Fatalf("list out of order at %d", i)
					}
				}
				prev = u
				if !u.Possible {
					if u.Reason == "" || u.Package != nil {
						t.Fatalf("not possible needs a reason and no package: %+v", u)
					}
					continue
				}
				possible++
				pk := u.Package
				if pk == nil || u.Reason != "" || pk.Note == "" || len(pk.Receive) != 1 || pk.Receive[0].PlayerID != u.Player.ID {
					t.Fatalf("bad package %+v", u)
				}
				in := false
				for _, a := range pk.Give {
					in = in || (a.Kind == PlayerAsset && a.PlayerID == p.ID)
				}
				if !in {
					t.Fatalf("package for %s does not include him", p.Name)
				}
				v := EvaluateDeal(lg, pk.Deal(user, u.Team), user)
				if !v.Valid || !v.Accept {
					t.Fatalf("seed %d: %s for %s not accepted: %+v", seed, p.Name, u.Player.Name, v)
				}
			}
		}
	}
	if possible == 0 || total == possible {
		t.Fatalf("expected a mix of possible and not possible, got %d of %d", possible, total)
	}
	t.Logf("%d candidates, %d possible", total, possible)
}

func TestFindUpgradesReadOnly(t *testing.T) {
	lg := upgradeLeague(2)
	before := gmJSON(t, lg)
	for _, p := range userBest(lg)[:3] {
		if _, err := lg.FindUpgrades(p.ID); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(before, gmJSON(t, lg)) {
		t.Fatalf("FindUpgrades changed the league")
	}
	a, _ := lg.FindUpgrades(userBest(lg)[0].ID)
	b, _ := lg.FindUpgrades(userBest(lg)[0].ID)
	if len(a.Upgrades) != len(b.Upgrades) {
		t.Fatalf("answers differ between calls")
	}
	for i := range a.Upgrades {
		if a.Upgrades[i].Player != b.Upgrades[i].Player || a.Upgrades[i].Possible != b.Upgrades[i].Possible {
			t.Fatalf("answers differ between calls")
		}
	}
}

func TestFindUpgradesErrorsAndClosedWindow(t *testing.T) {
	lg := upgradeLeague(1)
	other := lg.Teams[5].Roster[0]
	if _, err := lg.FindUpgrades(other.ID); err == nil {
		t.Fatalf("another team's player should be an error")
	}
	if _, err := lg.FindUpgrades(-5); err == nil {
		t.Fatalf("unknown player should be an error")
	}
	lg.Season.Day = TradeDeadlineDay + 1
	res, err := lg.FindUpgrades(userBest(lg)[0].ID)
	if err != nil || res.Possible || res.Reason == "" || res.Upgrades == nil || len(res.Upgrades) != 0 {
		t.Fatalf("closed window: %v %+v", err, res)
	}
}

// The star of a team has nobody to upgrade to: an empty list with a sentence.
func TestFindUpgradesNobodyBetter(t *testing.T) {
	lg := upgradeLeague(1)
	best := 0
	var star *Player
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			if p.Overall() > best {
				best, star = p.Overall(), p
			}
		}
	}
	lg.UserTeamID = star.TeamID
	res, err := lg.FindUpgrades(star.ID)
	if err != nil || !res.Possible || len(res.Upgrades) != 0 || res.Reason == "" {
		t.Fatalf("%v %+v", err, res)
	}
}

// A full answer for one player must come back quickly (the limit is generous
// so a busy machine does not fail it; it normally takes a fraction of it).
func TestFindUpgradesSpeed(t *testing.T) {
	lg := upgradeLeague(1)
	for _, p := range userBest(lg)[:5] {
		start := time.Now()
		if _, err := lg.FindUpgrades(p.ID); err != nil {
			t.Fatal(err)
		}
		d := time.Since(start)
		t.Logf("%s: %v", p.Name, d)
		if d > 2*time.Second {
			t.Fatalf("%s: took %v", p.Name, d)
		}
	}
}
