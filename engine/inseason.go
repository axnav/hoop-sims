package engine

import "math"

// In-season development (see Progression.InSeason*): small rating steps
// during the regular season, an advance on the yearly change.

const inSeasonOffset = 80000 // random stream: one hashed stream per player and step

// DevLine is one in-season overall change of a user-team player.
type DevLine struct {
	PlayerID        int
	Name            string
	Overall, Change int
}

// DevChangesLastDay lists user-team overall changes made on the most recent
// SimDay (not saved).
func (lg *League) DevChangesLastDay() []DevLine {
	return append([]DevLine{}, lg.newDev...)
}

// expectedTrend is the expected yearly change in overall points for a player
// whose overall at the start of the year is base (no noise, no leap or stall,
// no minutes bonus). The one copy of the formula, used by the yearly
// progression and by the in-season steps.
func expectedTrend(p *Player, base float64) float64 {
	c := &Progression
	if p.EditedCeiling > 0 && p.Age >= c.PrimeStartAge {
		// Commissioner-edited potential: an older player still grows toward it.
		return (float64(p.EditedCeiling) - base) * c.RateAge24
	}
	switch {
	case p.Age < c.PrimeStartAge:
		rate := c.RateAge26
		if p.Age <= 21 {
			rate = c.RateAge21
		} else if p.Age <= 24 {
			rate = c.RateAge24
		}
		return (p.TrueCeiling() - base) * rate
	case p.Age < c.DeclineStartAge:
		return float64(p.Impact) * c.PrimeImpactMult
	}
	return -float64(p.Age-c.DeclineBaseAge)*c.DeclinePerYear + float64(p.Impact)*c.DeclineImpactMult
}

// ratingStep applies the per-rating rules to a change d for the rating at
// index idx (order of Ratings.array) of a player of the given age.
func ratingStep(age, idx int, d float64) float64 {
	c := &Progression
	declining := age >= c.DeclineStartAge
	switch idx {
	case 6: // athleticism
		if declining {
			d *= c.AthleticismDeclineMult
		} else if age >= c.AthleticismGrowthStop && d > 0 {
			d = 0
		}
	case 1, 2: // mid, three
		if declining {
			d *= c.ShootingDeclineMult
		}
	}
	return d
}

// stepAge is the age used for per-rating rules: a player growing toward an
// edited potential is treated as not yet declining.
func stepAge(p *Player) int {
	if p.EditedCeiling > 0 && p.Age >= Progression.DeclineStartAge {
		return Progression.DeclineStartAge - 1
	}
	return p.Age
}

// athleticismGrowth: false for athleticism at an age where it no longer grows.
func athleticismGrows(age, idx int) bool {
	return !(idx == 6 && age >= Progression.AthleticismGrowthStop)
}

// inSeasonDev runs one in-season step (1-based) for every player on a team or
// in free agency. Each player has his own hashed random stream.
func (lg *League) inSeasonDev(step int) {
	c := &Progression
	user := map[int]bool{}
	for _, p := range lg.Teams[lg.UserTeamID].Roster {
		user[p.ID] = true
	}
	mentors := lg.mentorTeams()
	for _, p := range lg.allPlayers() {
		gain := 0.0
		if p.SeasonDev != nil {
			gain = float64(p.SeasonDev.Overall)
		}
		oldOvr, oldPot, oldRt := p.Overall(), p.Potential, p.Ratings
		base := float64(oldOvr) - gain
		r := NewRNG(mix64(lg.Seed, uint64(lg.Season.Year), uint64(inSeasonOffset+step), uint64(p.ID)))
		s := expectedTrend(p, base)*c.InSeasonShare/float64(c.InSeasonSteps) + r.Normal(0, c.InSeasonNoiseSD)
		sign := 1.0
		if s < 0 {
			sign = -1
		} else {
			s *= lg.growthMult(p, mentors) * lg.devMult(p.TeamID)
		}
		// Growth is split over the ratings by training weights (training.go),
		// total unchanged; a loss is spread equally.
		var x [NumRatings]float64
		for i := range x {
			x[i] = math.Abs(s)
			if s > 0 && !athleticismGrows(p.Age, i) {
				x[i] = 0
			}
		}
		if s > 0 {
			reweightGrowth(&x, lg.growthWeights(p))
		}
		a := p.Ratings.array()
		for i := range a {
			whole, frac := math.Modf(x[i])
			k := whole
			if r.Float64() < frac {
				k++
			}
			a[i] = clamp(a[i]+round(ratingStep(stepAge(p), i, sign*k)), 25, 99)
		}
		p.Ratings = Ratings{a[0], a[1], a[2], a[3], a[4], a[5], a[6]}
		newOvr := p.Overall()
		if p.EditedCeiling > 0 && newOvr >= p.EditedCeiling {
			p.EditedCeiling = 0
		}
		if p.EditedCeiling > 0 {
			p.Potential = p.EditedCeiling
		} else if p.Age >= c.PotentialEqualAge {
			p.Potential = newOvr
		} else {
			p.Potential = clamp(max(p.Potential, newOvr), 25, 99)
		}
		n, o := p.Ratings.array(), oldRt.array()
		var d [7]int
		changed := newOvr != oldOvr || p.Potential != oldPot
		for i := range d {
			d[i] = n[i] - o[i]
			changed = changed || d[i] != 0
		}
		if changed {
			if p.SeasonDev == nil {
				p.SeasonDev = &DevChange{}
			}
			sd := p.SeasonDev.Ratings.array()
			for i := range sd {
				sd[i] += d[i]
			}
			p.SeasonDev.Ratings = Ratings{sd[0], sd[1], sd[2], sd[3], sd[4], sd[5], sd[6]}
			p.SeasonDev.Overall += newOvr - oldOvr
			p.SeasonDev.Potential += p.Potential - oldPot
		}
		if user[p.ID] && newOvr != oldOvr {
			lg.newDev = append(lg.newDev, DevLine{p.ID, p.Name, newOvr, newOvr - oldOvr})
		}
	}
}
