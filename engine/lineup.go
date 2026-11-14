package engine

import (
	"errors"
	"fmt"
	"sort"
)

// Lineup rules.
const (
	TeamMinutes      = 240 // minutes in a game (5 players x 48)
	MaxPlayerMinutes = 42  // most minutes one player may be given
	// MinLineupPlayers is how many players must have minutes, so a game can
	// absorb foul trouble. (The game sim has no fouling-out; this is a
	// realism and safety margin, and keeps total capacity above 240 when
	// someone is hurt: 5 x 42 = 210 would not be enough.)
	MinLineupPlayers = 8
)

// Lineup is a custom starting five and minutes plan, by player ID.
// Starters[i] plays position slot i (PG, SG, SF, PF, C); Minutes holds the
// target minutes per game for every player who should play.
type Lineup struct {
	Starters [5]int
	Minutes  map[int]int
}

// NoPlayer marks a vacated starter slot (player ID 0 is a real player).
const NoPlayer = -1

// PlayerMinutes is one player's minutes in a game.
type PlayerMinutes struct {
	Player *Player
	Min    int
}

// autoMinutes is today's automatic plan: the rotation with minutesByRank.
func (t *Team) autoMinutes() []PlayerMinutes {
	rot := t.Rotation()
	out := make([]PlayerMinutes, len(rot))
	for i, p := range rot {
		m := 0
		if i < len(minutesByRank) {
			m = minutesByRank[i]
		}
		out[i] = PlayerMinutes{p, m}
	}
	return out
}

func (t *Team) player(id int) *Player {
	for _, p := range t.Roster {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// pruneLineup removes players who left the roster from the custom lineup. A
// vacated starter slot becomes NoPlayer (EffectiveMinutes treats it as unavailable).
// Roster changes happen in many places, so it is called lazily: at the start
// of EffectiveMinutes and when saving.
func (t *Team) pruneLineup() {
	l := t.Lineup
	if l == nil {
		return
	}
	for i, id := range l.Starters {
		if t.player(id) == nil {
			l.Starters[i] = NoPlayer
		}
	}
	for id := range l.Minutes {
		if t.player(id) == nil {
			delete(l.Minutes, id)
		}
	}
}

// EffectiveMinutes is who plays and for how long in the next game: starters
// first (slot order), then the rest by minutes, best first. Without a custom
// lineup it is exactly the automatic rotation. With one, unavailable
// (departed or injured) players are replaced and their minutes handed out
// again so the total stays 240 and nobody exceeds MaxPlayerMinutes.
func (t *Team) EffectiveMinutes() []PlayerMinutes {
	t.pruneLineup()
	auto := t.autoMinutes()
	l := t.Lineup
	if l == nil {
		return auto
	}
	rank := map[*Player]int{} // automatic order among healthy players
	for i, pm := range auto {
		rank[pm.Player] = i
	}
	var avail []*Player // healthy roster in automatic order
	for _, pm := range auto {
		if !pm.Player.Injured() {
			avail = append(avail, pm.Player)
		}
	}
	// Healthy players with minutes; fewer than five means use the automatic plan.
	listed := 0
	for _, p := range avail {
		if l.Minutes[p.ID] > 0 {
			listed++
		}
	}
	if listed < 5 || len(avail)*MaxPlayerMinutes < TeamMinutes || len(avail) < 6 {
		return auto
	}
	isAvail := map[*Player]bool{}
	for _, p := range avail {
		isAvail[p] = true
	}
	mins := map[*Player]int{}
	for _, p := range avail {
		mins[p] = min(MaxPlayerMinutes, max(0, l.Minutes[p.ID]))
	}
	var starters []*Player
	used := map[*Player]bool{}
	for _, id := range l.Starters {
		if p := t.player(id); p != nil && isAvail[p] && !used[p] {
			starters = append(starters, p)
			used[p] = true
		} else {
			starters = append(starters, nil)
		}
	}
	// Promote for each missing starter: the listed player with the most
	// minutes who is not starting, else the best by automatic ranking.
	for i, s := range starters {
		if s != nil {
			continue
		}
		var best *Player
		for _, p := range avail {
			if used[p] {
				continue
			}
			if best == nil || mins[p] > mins[best] {
				best = p // avail is in automatic order, so ties keep the better-ranked
			}
		}
		starters[i] = best
		used[best] = true
	}
	// Hand out missing minutes: one at a time, round robin in automatic
	// order, first among players already playing (or starting), then anyone.
	total := 0
	for _, p := range avail {
		total += mins[p]
	}
	for pass := 0; pass < 2 && total < TeamMinutes; pass++ {
		for total < TeamMinutes {
			gave := false
			for _, p := range avail {
				if total >= TeamMinutes {
					break
				}
				if mins[p] >= MaxPlayerMinutes || (pass == 0 && mins[p] == 0 && !used[p]) {
					continue
				}
				mins[p]++
				total++
				gave = true
			}
			if !gave {
				break
			}
		}
	}
	out := make([]PlayerMinutes, 0, len(avail))
	for _, p := range starters {
		out = append(out, PlayerMinutes{p, mins[p]})
	}
	var bench []*Player
	for _, p := range avail {
		if !used[p] {
			bench = append(bench, p)
		}
	}
	sort.SliceStable(bench, func(i, j int) bool { return mins[bench[i]] > mins[bench[j]] })
	for _, p := range bench {
		out = append(out, PlayerMinutes{p, mins[p]})
	}
	return out
}

// AutoLineup is the lineup the automatic logic would use right now, as a
// starting point for the user (injured players are left out).
func (lg *League) AutoLineup(teamID int) Lineup {
	t := lg.Teams[teamID]
	auto := t.autoMinutes()
	l := Lineup{Minutes: map[int]int{}}
	var five []*Player
	for i, pm := range auto {
		if i < 5 {
			five = append(five, pm.Player)
		}
		if pm.Min > 0 {
			l.Minutes[pm.Player.ID] = pm.Min
		}
	}
	// Seat the five in position slots, each by the best fit still free.
	used := map[*Player]bool{}
	for pos := PG; pos <= C; pos++ {
		var best *Player
		for _, p := range five {
			if used[p] {
				continue
			}
			better := best == nil
			if !better {
				if p.CanPlay(pos) != best.CanPlay(pos) {
					better = p.CanPlay(pos)
				} else {
					better = p.OverallAt(pos) > best.OverallAt(pos)
				}
			}
			if better {
				best = p
			}
		}
		if best != nil {
			used[best] = true
			l.Starters[pos] = best.ID
		}
	}
	return l
}

// SetLineup validates and stores a custom lineup for a team.
func (lg *League) SetLineup(teamID int, l Lineup) error {
	if teamID < 0 || teamID >= len(lg.Teams) {
		return errors.New("That team does not exist.")
	}
	t := lg.Teams[teamID]
	seen := map[int]bool{}
	for _, id := range l.Starters {
		p := t.player(id)
		if p == nil {
			return errors.New("Every starter must be a player on your roster.")
		}
		if seen[id] {
			return fmt.Errorf("%s is listed as a starter twice.", p.Name)
		}
		seen[id] = true
	}
	total, playing := 0, 0
	for id, m := range l.Minutes {
		p := t.player(id)
		if p == nil {
			return errors.New("Minutes were given to a player who is not on your roster.")
		}
		if m < 0 || m > MaxPlayerMinutes {
			return fmt.Errorf("%s has %d minutes; each player needs between 0 and %d.", p.Name, m, MaxPlayerMinutes)
		}
		total += m
		if m > 0 {
			playing++
		}
	}
	for _, id := range l.Starters {
		if l.Minutes[id] < 1 {
			return fmt.Errorf("%s starts, so he needs at least 1 minute.", t.player(id).Name)
		}
	}
	if total != TeamMinutes {
		return fmt.Errorf("Minutes add up to %d; they must add up to exactly %d.", total, TeamMinutes)
	}
	if playing < MinLineupPlayers {
		return fmt.Errorf("At least %d players need minutes (you have %d), so a game can cover foul trouble and tired legs.", MinLineupPlayers, playing)
	}
	cp := Lineup{Starters: l.Starters, Minutes: make(map[int]int, len(l.Minutes))}
	for id, m := range l.Minutes {
		cp.Minutes[id] = m
	}
	t.Lineup = &cp
	return nil
}

// ClearLineup puts a team back on the automatic lineup.
func (lg *League) ClearLineup(teamID int) {
	if teamID >= 0 && teamID < len(lg.Teams) {
		lg.Teams[teamID].Lineup = nil
	}
}
