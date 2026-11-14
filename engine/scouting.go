package engine

import (
	"errors"
	"fmt"
)

// Scouting: until a prospect is drafted, the user sees a range, not his true
// numbers. Each prospect has a fixed scouting error (drawn once when the draft
// is created); spending scouting points narrows the range and moves its
// centre toward the truth. The truth is always inside the range.
//
// RNG streams (all derived from lg.Seed and the draft year, offset 70000):
//
//	70000   the per-prospect errors, drawn once in pool order at draft creation
//	70001+  computer-team draft noise and per-rating errors, each from a tiny
//	        hashed stream per (team, prospect) / (prospect, rating) so they do
//	        not depend on the order of picks or screen views.
const scoutOffset = 70000

// ScoutTuningT holds the scouting tunables.
type ScoutTuningT struct {
	Points      int     // scouting points the user gets each draft
	CostPerStep int     // points spent per level
	OvrHalf     [4]int  // range half-width for overall, by level 0..3
	PotHalf     [4]int  // range half-width for potential, by level 0..3
	RatingExtra int     // individual ratings are this much wider than overall
	CentreShift float64 // centre is truth + round(err * halfwidth * CentreShift)
	ErrSD       float64 // sd of the stored error fraction before clamping to [-1, 1]
}

// ScoutTuning: 12 points, 1 per level, levels 0..3. Level 0 is "no scouting".
var ScoutTuning = ScoutTuningT{
	Points: 12, CostPerStep: 1,
	OvrHalf: [4]int{7, 5, 3, 1}, PotHalf: [4]int{9, 6, 4, 2},
	RatingExtra: 2, CentreShift: 0.8, ErrSD: 0.5,
}

// MaxScoutLevel is the best scouting level.
const MaxScoutLevel = 3

// ComputerDraftNoise is the standard deviation (rating points on the ranking
// score) of a computer team's view of a prospect: about level-1 scouting.
var ComputerDraftNoise = 3.0

// ScoutView is the range the user sees for a prospect.
type ScoutView struct {
	Level                      int
	OvrLo, OvrHi, PotLo, PotHi int
}

func mix64(vals ...uint64) uint64 {
	h := uint64(0x9e3779b97f4a7c15)
	for _, v := range vals {
		h ^= v + 0x9e3779b97f4a7c15 + (h << 6) + (h >> 2)
		h ^= h >> 30
		h *= 0xbf58476d1ce4e5b9
		h ^= h >> 27
		h *= 0x94d049bb133111eb
		h ^= h >> 31
	}
	return h
}

// initScouting draws the errors and gives the user their points.
func (lg *League) initScouting(d *Draft) {
	r := lg.dayRNG(lg.Year, scoutOffset)
	d.OvrErr, d.PotErr, d.ScoutLevel = map[int]float64{}, map[int]float64{}, map[int]int{}
	for _, p := range d.Pool {
		d.OvrErr[p.ID] = clampF(r.Normal(0, ScoutTuning.ErrSD), -1, 1)
		d.PotErr[p.ID] = clampF(r.Normal(0, ScoutTuning.ErrSD), -1, 1)
	}
	d.ScoutPoints = ScoutTuning.Points
}

// ensureDraft fills what a draft saved by an older build lacks (a single
// round order and no scouting state), so such a save can be finished. It does
// nothing for a draft made by this build.
func (lg *League) ensureDraft() {
	d := lg.Draft
	if d == nil {
		return
	}
	if len(d.Order2) != len(d.Order) {
		d.Order2 = append([]int(nil), d.Order...)
	}
	if d.OvrErr == nil || d.PotErr == nil {
		d.OvrErr, d.PotErr = map[int]float64{}, map[int]float64{}
		for _, p := range d.Pool {
			r := NewRNG(mix64(lg.Seed, uint64(lg.Year), scoutOffset+4, uint64(p.ID)))
			d.OvrErr[p.ID] = clampF(r.Normal(0, ScoutTuning.ErrSD), -1, 1)
			d.PotErr[p.ID] = clampF(r.Normal(0, ScoutTuning.ErrSD), -1, 1)
		}
		d.ScoutPoints = ScoutTuning.Points
	}
	if d.ScoutLevel == nil {
		d.ScoutLevel = map[int]int{}
	}
}

func (d *Draft) prospect(id int) *Player {
	for _, p := range d.Pool {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func shift(e float64, w int) int { return round(e * float64(w) * ScoutTuning.CentreShift) }

func clampRating(v int) int { return clamp(v, MinRating, MaxRating) }

// ScoutView returns the range shown to the user for a prospect still in the
// pool.
func (lg *League) ScoutView(prospectID int) (ScoutView, bool) {
	lg.ensureDraft()
	if lg.Draft == nil {
		return ScoutView{}, false
	}
	p := lg.Draft.prospect(prospectID)
	if p == nil {
		return ScoutView{}, false
	}
	d := lg.Draft
	lv := d.ScoutLevel[p.ID]
	wo, wp := ScoutTuning.OvrHalf[lv], ScoutTuning.PotHalf[lv]
	co := p.Overall() + shift(d.OvrErr[p.ID], wo)
	cp := p.Potential + shift(d.PotErr[p.ID], wp)
	v := ScoutView{Level: lv,
		OvrLo: clampRating(co - wo), OvrHi: clampRating(co + wo),
		PotLo: clampRating(cp - wp), PotHi: clampRating(cp + wp)}
	v.PotLo = max(v.PotLo, v.OvrLo)
	v.PotHi = max(v.PotHi, v.PotLo)
	return v, true
}

// ScoutRatings returns a lo/hi range per rating key (see RatingKeys) for a
// prospect still in the pool, at his current scouting level.
func (lg *League) ScoutRatings(prospectID int) (map[string][2]int, bool) {
	lg.ensureDraft()
	if lg.Draft == nil {
		return nil, false
	}
	p := lg.Draft.prospect(prospectID)
	if p == nil {
		return nil, false
	}
	lv := lg.Draft.ScoutLevel[p.ID]
	w := ScoutTuning.OvrHalf[lv] + ScoutTuning.RatingExtra
	out := map[string][2]int{}
	for i, k := range RatingKeys() {
		truth, _ := GetRating(p.Ratings, k.Key)
		r := NewRNG(mix64(lg.Seed, uint64(lg.Year), scoutOffset+3, uint64(p.ID), uint64(i)))
		h := clampF(r.Normal(0, ScoutTuning.ErrSD), -1, 1)
		e := clampF(0.5*lg.Draft.OvrErr[p.ID]+0.5*h, -1, 1)
		c := truth + shift(e, w)
		out[k.Key] = [2]int{clampRating(c - w), clampRating(c + w)}
	}
	return out, true
}

// Scout spends one scouting point (CostPerStep) on a prospect.
func (lg *League) Scout(prospectID int) error {
	lg.ensureDraft()
	d := lg.Draft
	if d == nil {
		return errors.New("there is no draft in progress")
	}
	if d.prospect(prospectID) == nil {
		return fmt.Errorf("prospect %d is not in the draft pool (already drafted?)", prospectID)
	}
	if d.ScoutLevel[prospectID] >= MaxScoutLevel {
		return errors.New("this prospect is already fully scouted")
	}
	if d.ScoutPoints < ScoutTuning.CostPerStep {
		return errors.New("no scouting points left")
	}
	d.ScoutPoints -= ScoutTuning.CostPerStep
	d.ScoutLevel[prospectID]++
	return nil
}

// ScoutPointsLeft is the user's unspent scouting points (0 with no draft).
func (lg *League) ScoutPointsLeft() int {
	lg.ensureDraft()
	if lg.Draft == nil {
		return 0
	}
	return lg.Draft.ScoutPoints
}

// userDraftScore ranks a prospect by the centre of the user's view, with the
// same formula as draftScore.
func (lg *League) userDraftScore(p *Player) float64 {
	d, c := lg.Draft, &Progression
	lv := d.ScoutLevel[p.ID]
	wo, wp := ScoutTuning.OvrHalf[lv], ScoutTuning.PotHalf[lv]
	co := p.Overall() + shift(d.OvrErr[p.ID], wo)
	cp := p.Potential + shift(d.PotErr[p.ID], wp)
	return c.DraftOvrWeight*float64(co) + c.DraftPotWeight*float64(cp)
}

// computerDraftScore is a computer team's noisy view of draftScore. The noise
// is fixed per (team, prospect, draft).
func (lg *League) computerDraftScore(team int, p *Player) float64 {
	r := NewRNG(mix64(lg.Seed, uint64(lg.Year), scoutOffset+1, uint64(team), uint64(p.ID)))
	return draftScore(p) + r.Normal(0, ComputerDraftNoise)
}

// autoChoice returns the pool index the given team picks automatically: the
// user's team by the centre of its scouting view, others by a noisy view.
func (lg *League) autoChoice(team int) int {
	d := lg.Draft
	score := func(p *Player) float64 { return lg.computerDraftScore(team, p) }
	if team == lg.UserTeamID {
		score = lg.userDraftScore
	}
	bi, best := 0, score(d.Pool[0])
	for i := 1; i < len(d.Pool); i++ {
		if s := score(d.Pool[i]); s > best {
			bi, best = i, s
		}
	}
	return bi
}
