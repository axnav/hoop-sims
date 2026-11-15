package engine

import (
	"errors"
	"fmt"
	"math"
)

// Training focus and learning speeds.
//
// A focus moves WHERE a player's growth goes, never how much: every growth
// step is split across the ratings with a weight (focus x learning speed x age
// window) and then scaled back so the player's total positive growth in that
// step equals what it would have been with equal weights. Decline is not
// touched. Learning speeds are derived (nothing saved) from the league seed
// and the player ID.

// Focus is a player's training focus. The zero value, Balanced, is today's
// behaviour.
type Focus int

const (
	FocusBalanced Focus = iota
	FocusShooting
	FocusInside
	FocusPlaymaking
	FocusDefense
	FocusRebounding
	FocusBody
	numFocuses
)

// aptitudeOffset is the RNG stream for learning speeds: one hashed stream per
// player (see RULES.md section 3).
const aptitudeOffset = 90001

// NumRatings is the number of ratings in Ratings.array.
const NumRatings = 7

var focusKeys = [...]string{"balanced", "shooting", "inside", "playmaking", "defense", "rebounding", "body"}
var focusLabels = [...]string{"Balanced", "Shooting", "Inside scoring", "Playmaking", "Defense", "Rebounding", "Body"}

// focusRatings are the rating indexes (order of Ratings.array: Inside, Mid,
// Three, Playmaking, Rebounding, Defense, Athleticism) each focus favours.
var focusRatings = [...][]int{
	FocusBalanced:   nil,
	FocusShooting:   {1, 2},
	FocusInside:     {0},
	FocusPlaymaking: {3},
	FocusDefense:    {5},
	FocusRebounding: {4},
	FocusBody:       {4, 6}, // athleticism and rebounding (the physical ratings)
}

// TrainingTuning holds the numbers for focus and learning speeds.
var TrainingTuning = struct {
	FavouredMult float64 // weight of a favoured rating
	OtherMult    float64 // weight of the others when a focus is set
	SlowMult     float64 // learning speed multipliers
	NormalMult   float64
	FastMult     float64
	SlowShare    float64 // share of (player, rating) pairs that are Slow
	FastShare    float64 // share that are Fast (rest Normal)
	AutoMaxAge   int     // computer teams keep players this old or older Balanced
}{
	FavouredMult: 1.6, OtherMult: 0.85,
	SlowMult: 0.8, NormalMult: 1.0, FastMult: 1.25,
	SlowShare: 0.25, FastShare: 0.25,
	AutoMaxAge: 30,
}

// AllFocuses lists every focus in menu order.
func AllFocuses() []Focus {
	out := make([]Focus, numFocuses)
	for i := range out {
		out[i] = Focus(i)
	}
	return out
}

func (f Focus) valid() bool   { return f >= 0 && f < numFocuses }
func (f Focus) Key() string   { return focusKeys[f] }
func (f Focus) Label() string { return focusLabels[f] }

// FocusFromKey parses a JSON key such as "shooting".
func FocusFromKey(k string) (Focus, bool) {
	for i, s := range focusKeys {
		if s == k {
			return Focus(i), true
		}
	}
	return FocusBalanced, false
}

// windowOpen says whether a rating can still grow at this age. A close age of
// 0 in Progression.GrowthWindowClose means the window never closes;
// athleticism uses Progression.AthleticismGrowthStop (as ratingStep does).
func windowOpen(age, idx int) bool {
	if idx == 6 {
		return age < Progression.AthleticismGrowthStop
	}
	closeAge := Progression.GrowthWindowClose[idx]
	return closeAge == 0 || age < closeAge
}

// AptitudeLabel names a learning speed multiplier.
func AptitudeLabel(x float64) string {
	switch {
	case x < TrainingTuning.NormalMult:
		return "Slow"
	case x > TrainingTuning.NormalMult:
		return "Fast"
	}
	return "Normal"
}

// Aptitude is the player's learning speed multiplier for each rating, the
// same in every call (derived from the seed and his ID, nothing saved).
func (lg *League) Aptitude(p *Player) [NumRatings]float64 {
	t := &TrainingTuning
	r := NewRNG(mix64(lg.Seed, uint64(aptitudeOffset), uint64(p.ID)))
	var out [NumRatings]float64
	for i := range out {
		x := r.Float64()
		switch {
		case x < t.SlowShare:
			out[i] = t.SlowMult
		case x >= 1-t.FastShare:
			out[i] = t.FastMult
		default:
			out[i] = t.NormalMult
		}
	}
	return out
}

// growthWeights is the per-rating growth weight for a player today: focus x
// learning speed x age window. Only the proportions matter (reweightGrowth
// scales the total back).
func (lg *League) growthWeights(p *Player) *[NumRatings]float64 {
	t := &TrainingTuning
	apt := lg.Aptitude(p)
	var w [NumRatings]float64
	for i := range w {
		f := 1.0
		if p.Focus != FocusBalanced && p.Focus.valid() {
			f = t.OtherMult
			for _, j := range focusRatings[p.Focus] {
				if j == i {
					f = t.FavouredMult
				}
			}
		}
		w[i] = f * apt[i]
		if !windowOpen(p.Age, i) {
			w[i] = 0
		}
	}
	return &w
}

// reweightGrowth multiplies the positive entries of d by w and scales them so
// their sum is unchanged. Negative entries are untouched. If every positive
// entry sits in a closed window (weight 0) that growth is lost.
func reweightGrowth(d *[NumRatings]float64, w *[NumRatings]float64) {
	base, weighted := 0.0, 0.0
	for i, v := range d {
		if v > 0 {
			base += v
			weighted += v * w[i]
		}
	}
	if base == 0 {
		return
	}
	if weighted <= 0 {
		for i, v := range d {
			if v > 0 {
				d[i] = 0
			}
		}
		return
	}
	scale := base / weighted
	for i, v := range d {
		if v > 0 {
			d[i] = v * w[i] * scale
		}
	}
}

// SetFocus sets the training focus of a player on the given team.
func (lg *League) SetFocus(teamID, playerID int, f Focus) error {
	if teamID < 0 || teamID >= len(lg.Teams) {
		return errors.New("There is no such team.")
	}
	if !f.valid() {
		return errors.New("That is not a training focus.")
	}
	p := lg.findRosterPlayer(teamID, playerID)
	if p == nil {
		return errors.New("That player is not on this team.")
	}
	p.Focus = f
	return nil
}

// autoFocus is the focus a computer team gives a player: the one whose
// favoured ratings have the most room to grow, counted by how much each
// rating matters at his position. Players at AutoMaxAge or older, and any
// player whose favoured windows are all closed, stay Balanced. No randomness.
func autoFocus(p *Player) Focus {
	if p.Age >= TrainingTuning.AutoMaxAge {
		return FocusBalanced
	}
	rt := p.Ratings.array()
	ow := overallWeights[p.Pos]
	best, bestScore := FocusBalanced, 0.0
	for f := FocusShooting; f < numFocuses; f++ {
		score := 0.0
		for _, i := range focusRatings[f] {
			if windowOpen(p.Age, i) {
				score += ow[i] * float64(99-rt[i])
			}
		}
		if score > bestScore {
			best, bestScore = f, score
		}
	}
	return best
}

// autoFocusTeams gives every computer team's players a focus (the user's team
// keeps what the user chose).
func (lg *League) autoFocusTeams() {
	for _, t := range lg.Teams {
		if t.ID == lg.UserTeamID {
			continue
		}
		for _, p := range t.Roster {
			p.Focus = autoFocus(p)
		}
	}
}

// SkillTraining is one rating's line in a TrainingView.
type SkillTraining struct {
	Key, Label string
	Speed      string // Slow, Normal, Fast
	Open       bool
	StepsFor2  string // "about 6", "10+" or "closed"
}

// TrainingView is a rough picture of how fast each rating is growing for a
// player. The estimate: the in-season step size for his age (expectedTrend x
// InSeasonShare / InSeasonSteps, the average overall points per step), split
// over the ratings by his weights the way reweightGrowth does, then
// 2 / (that rating's step). It ignores noise, minutes, leaps and the offseason
// part, so read it as "about", capped at 10+. A closed window says "closed".
func (lg *League) TrainingView(p *Player) []SkillTraining {
	c := &Progression
	apt := lg.Aptitude(p)
	w := lg.growthWeights(p)
	step := expectedTrend(p, float64(p.Overall())) * c.InSeasonShare / float64(c.InSeasonSteps)
	// Share of the step each rating gets: weights over the ratings that can grow.
	sum, n := 0.0, 0.0
	for i := range w {
		sum += w[i]
		n++
	}
	out := make([]SkillTraining, 0, NumRatings)
	for i, k := range ratingKeys {
		line := SkillTraining{Key: k.Key, Label: k.Label, Speed: AptitudeLabel(apt[i]), Open: windowOpen(p.Age, i)}
		switch {
		case !line.Open:
			line.StepsFor2 = "closed"
		case step <= 0 || sum <= 0:
			line.StepsFor2 = "10+"
		default:
			per := step * w[i] * n / sum
			steps := int(math.Ceil(2/per - 1e-9))
			if steps >= 10 {
				line.StepsFor2 = "10+"
			} else {
				line.StepsFor2 = fmt.Sprintf("about %d", max(steps, 1))
			}
		}
		out = append(out, line)
	}
	return out
}
