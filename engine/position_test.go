package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

func bestOfListed(p *Player) int {
	best := 0
	for _, pos := range p.Positions() {
		best = max(best, p.OverallAt(pos))
	}
	return best
}

func TestMovedAndUnmovedOverall(t *testing.T) {
	lg := GenerateLeague(7)
	lg.UserTeamID = 0
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			if p.NaturalPos != nil || p.Overall() != bestOfListed(p) {
				t.Fatalf("%s: unmoved overall %d, best of listed %d", p.Name, p.Overall(), bestOfListed(p))
			}
		}
	}
	own := lg.Teams[0].Roster[0]
	target := C
	if own.Pos == C {
		target = PG
	}
	badgesBefore := lg.badgeSet(own)
	if err := lg.SetPositions(own.ID, target, nil); err != nil {
		t.Fatal(err)
	}
	if own.NaturalPos == nil || own.Overall() != own.OverallAt(target) || own.Potential < own.Overall() {
		t.Fatalf("moved overall %d, at %v %d", own.Overall(), target, own.OverallAt(target))
	}
	badgesAfter := lg.badgeSet(own)
	if !reflect.DeepEqual(badgesBefore, badgesAfter) {
		t.Fatalf("badge tiers changed by a move")
	}
	// the flag survives a save round-trip
	b, err := json.Marshal(own)
	if err != nil {
		t.Fatal(err)
	}
	var back Player
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.NaturalPos == nil || back.Overall() != own.Overall() || back.NatPos() != own.NatPos() {
		t.Fatalf("flag lost in round-trip")
	}
	// back to his natural positions clears the flag
	var alt *Position
	if len(own.NaturalAlt) > 0 {
		a := own.NaturalAlt[0]
		alt = &a
	}
	if err := lg.SetPositions(own.ID, *own.NaturalPos, alt); err != nil {
		t.Fatal(err)
	}
	if own.NaturalPos != nil || own.Overall() != bestOfListed(own) {
		t.Fatalf("moving back did not restore the old rule")
	}
}

func TestSetPositions(t *testing.T) {
	lg := GenerateLeague(7)
	lg.UserTeamID = 0
	own := lg.Teams[0].Roster[0]
	other := lg.Teams[1].Roster[0]
	if err := lg.SetPositions(own.ID, PG, &[]Position{PG}[0]); err == nil {
		t.Fatal("second equal to main accepted")
	}
	if err := lg.SetPositions(other.ID, PG, nil); err == nil {
		t.Fatal("another team's player accepted without commissioner")
	}
	lg.Options.Commissioner = true
	if err := lg.SetPositions(other.ID, PG, nil); err != nil {
		t.Fatal(err)
	}
	lg.Draft = &Draft{}
	pr := GeneratePlayer(NewRNG(1), 99999, PG, 50)
	lg.Draft.Pool = append(lg.Draft.Pool, pr)
	if err := lg.SetPositions(pr.ID, C, nil); err == nil {
		t.Fatal("prospect accepted")
	}
}
