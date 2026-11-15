package main

import (
	"net/http"

	"hoopsim/engine"
)

// badgesObj lists a player's badges for the player page (an empty list, never
// null; prospects get none). Rules and effects never mention hidden numbers.
func (s *server) badgesObj(p *engine.Player) []M {
	out := []M{}
	for _, b := range s.lg.BadgesOf(p) {
		out = append(out, M{"key": b.Key, "name": b.Name, "group": b.Group, "tier": b.Tier.Key(),
			"kind": b.Kind, "rule": b.Rule, "effect": b.Effect, "next": b.Next})
	}
	return out
}

// addBadgeCount puts "badgeCount" on a non-prospect player row.
func (s *server) addBadgeCount(row M, p *engine.Player) {
	if p == nil || s.lg == nil || s.lg.IsProspect(p) {
		return
	}
	row["badgeCount"] = s.lg.BadgeCount(p)
}

// badgeCatalogue answers GET /api/badges.
func (s *server) badgeCatalogue(r *http.Request) (any, error) {
	list := []M{}
	for _, c := range engine.BadgeCatalogue() {
		list = append(list, M{"key": c.Key, "name": c.Name, "group": c.Group, "kind": c.Kind, "rule": c.Rule, "effect": c.Effect})
	}
	return M{"badges": list}, nil
}
