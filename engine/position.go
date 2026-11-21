package engine

import (
	"errors"
	"fmt"
)

// CanSetPositions says whether the user may move this player's positions now:
// his own players, or anyone in commissioner mode; never a draft prospect.
func (lg *League) CanSetPositions(p *Player) bool {
	if p == nil || lg.IsProspect(p) {
		return false
	}
	if lg.Options.Commissioner {
		return true
	}
	if t := lg.Teams[lg.UserTeamID]; t != nil {
		return t.player(p.ID) != nil
	}
	return false
}

// SetPositions sets a player's main and second position; his overall becomes
// the overall at the new main position. second nil clears the second position.
// Potential stays at least the new overall. No effect on simulated games yet
// beyond what the overall itself does.
func (lg *League) SetPositions(playerID int, main Position, second *Position) error {
	p := lg.findPlayer(playerID)
	if p == nil {
		return fmt.Errorf("no player with id %d", playerID)
	}
	if lg.IsProspect(p) {
		return errors.New("a draft prospect's position cannot be changed")
	}
	if !lg.CanSetPositions(p) {
		return errors.New("you can only change the position of your own players")
	}
	if main < PG || main > C {
		return errors.New("unknown position")
	}
	var alt []Position
	if second != nil {
		if *second < PG || *second > C {
			return errors.New("unknown second position")
		}
		if *second == main {
			return errors.New("the second position must differ from the main one")
		}
		alt = []Position{*second}
	}
	if p.NaturalPos == nil {
		np := p.Pos
		p.NaturalPos, p.NaturalAlt = &np, p.AltPos
	}
	p.Pos, p.AltPos = main, alt
	if main == *p.NaturalPos && samePositions(alt, p.NaturalAlt) {
		// Back to exactly his natural positions: the old rule applies again.
		p.NaturalPos, p.NaturalAlt = nil, nil
	}
	ovr := p.Overall()
	p.Potential = max(p.Potential, ovr)
	if p.EditedCeiling <= ovr {
		p.EditedCeiling = 0
	}
	return nil
}

func samePositions(a, b []Position) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
