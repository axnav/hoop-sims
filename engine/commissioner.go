package engine

import (
	"errors"
	"fmt"
	"math"
)

// Rating bounds used everywhere in the engine.
const (
	MinRating = 25
	MaxRating = 99
	MinAge    = 18
	MaxAge    = 45
)

// RatingKey is one visible rating: the key the web layer uses and its label.
type RatingKey struct{ Key, Label string }

var ratingKeys = []RatingKey{
	{"inside", "Inside"}, {"mid", "Mid-range"}, {"three", "Three"}, {"playmaking", "Playmaking"},
	{"rebounding", "Rebounding"}, {"defense", "Defense"}, {"athleticism", "Athleticism"},
}

// RatingKeys lists the rating keys in display order.
func RatingKeys() []RatingKey { return append([]RatingKey(nil), ratingKeys...) }

func ratingField(r *Ratings, key string) *int {
	switch key {
	case "inside":
		return &r.Inside
	case "mid":
		return &r.Mid
	case "three":
		return &r.Three
	case "playmaking":
		return &r.Playmaking
	case "rebounding":
		return &r.Rebounding
	case "defense":
		return &r.Defense
	case "athleticism":
		return &r.Athleticism
	}
	return nil
}

// GetRating reads a visible rating by key.
func GetRating(r Ratings, key string) (int, bool) {
	f := ratingField(&r, key)
	if f == nil {
		return 0, false
	}
	return *f, true
}

// SetRating writes a visible rating by key (no range check).
func SetRating(r *Ratings, key string, v int) bool {
	f := ratingField(r, key)
	if f == nil {
		return false
	}
	*f = v
	return true
}

// PlayerEdit is a commissioner edit; nil/empty fields are left alone.
type PlayerEdit struct {
	Age       *int
	Potential *int
	Ratings   map[string]int
	Heal      *bool // true clears the injury
	Overall   *int  // target overall; applied after the rating edits
}

// findPlayer looks in rosters, free agents and the current draft pool.
func (lg *League) findPlayer(id int) *Player {
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			if p.ID == id {
				return p
			}
		}
	}
	for _, p := range lg.FreeAgents {
		if p.ID == id {
			return p
		}
	}
	if lg.Draft != nil {
		for _, p := range lg.Draft.Pool {
			if p.ID == id {
				return p
			}
		}
	}
	return nil
}

// EditPlayer applies a commissioner edit, all fields or none. It never
// touches the hidden Impact. Potential is kept at least the new overall.
func (lg *League) EditPlayer(id int, e PlayerEdit) error {
	if !lg.Options.Commissioner {
		return errors.New("commissioner mode is off")
	}
	p := lg.findPlayer(id)
	if p == nil {
		return fmt.Errorf("no player with id %d", id)
	}
	if e.Age != nil && (*e.Age < MinAge || *e.Age > MaxAge) {
		return fmt.Errorf("age must be %d to %d", MinAge, MaxAge)
	}
	if e.Potential != nil && (*e.Potential < MinRating || *e.Potential > MaxRating) {
		return fmt.Errorf("potential must be %d to %d", MinRating, MaxRating)
	}
	nr := p.Ratings
	for k, v := range e.Ratings {
		if v < MinRating || v > MaxRating {
			return fmt.Errorf("%s must be %d to %d", k, MinRating, MaxRating)
		}
		if !SetRating(&nr, k, v) {
			return fmt.Errorf("unknown rating %q", k)
		}
	}
	if e.Overall != nil && (*e.Overall < MinRating || *e.Overall > MaxRating) {
		return fmt.Errorf("overall must be %d to %d", MinRating, MaxRating)
	}
	// Everything is valid: apply.
	q := *p
	q.Ratings = nr
	if e.Overall != nil {
		q.Ratings = commissionerShiftOverall(&q, *e.Overall)
		nr = q.Ratings
	}
	pot := p.Potential
	if e.Potential != nil {
		pot = *e.Potential
	}
	p.Ratings = nr
	p.Potential = max(pot, q.Overall())
	if e.Age != nil {
		p.Age = *e.Age
	}
	if e.Heal != nil && *e.Heal {
		p.Injury = nil
	}
	return nil
}

// commissionerShiftOverall moves all seven ratings by the same amount until the
// player's overall matches target, or is as close as clamping allows.
func commissionerShiftOverall(q *Player, target int) Ratings {
	base := q.Ratings
	best, bestDiff := base, absInt(q.Overall()-target)
	delta := 0.0
	for i := 0; i < 12 && bestDiff > 0; i++ {
		delta += float64(target - q.Overall())
		q.Ratings = commissionerShiftRatings(base, delta)
		if d := absInt(q.Overall() - target); d < bestDiff {
			best, bestDiff = q.Ratings, d
		}
	}
	q.Ratings = best
	return best
}

func commissionerShiftRatings(r Ratings, delta float64) Ratings {
	for _, k := range []string{"inside", "mid", "three", "playmaking", "rebounding", "defense", "athleticism"} {
		f := ratingField(&r, k)
		*f = clamp(int(math.Round(float64(*f)+delta)), MinRating, MaxRating)
	}
	return r
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
