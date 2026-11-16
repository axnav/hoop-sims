package engine

import (
	"fmt"
	"sort"
	"strings"
)

// Team systems: six styles of play of our own. A system does two things.
//
//  1. It shifts how the team plays (pace, share of threes, how much the stars
//     shoot, defensive effort). The shifts are trade-offs, not free power:
//     Run and Gun is fast but gives some defense away, Defense First is slow
//     but stingy, and so on. Balanced has no shifts.
//  2. It names the ratings and badges it wants. Each player gets a fit score
//     (0-100) and a grade (A-D) from those; the team fit (minutes-weighted
//     average, minus a penalty when the team does not run its coach's
//     favourite system) nudges the team's shooting a little, measured against
//     the league average (see fit.go), so the league as a whole is unchanged.

// SystemTuning holds the numbers behind fit scores. Edit here.
var SystemTuning = struct {
	// A player's rating part (0 to RatingPoints): the system's weighted rating
	// value from RatingLo (0 points) up to RatingHi (full points).
	RatingLo, RatingHi, RatingPoints float64
	// Badge part: BadgePoint per wanted badge tier (Bronze 1 ... Legend 5; the
	// Star badge counts as Gold), up to BadgeMax in all.
	BadgePoint, BadgeMax float64
	// Balanced asks nothing: everyone fits this well.
	BalancedFit float64
	// Grade thresholds: score at least this for A, B, C; below C is D.
	GradeA, GradeB, GradeC float64
	// A rating value (the system's weighted blend) at or above High is "strong";
	// below Low is "low", in the reasons.
	StrongRating, LowRating float64
}{
	RatingLo: 45, RatingHi: 80, RatingPoints: 60,
	BadgePoint: 5, BadgeMax: 40,
	BalancedFit: 58,
	GradeA:      72, GradeB: 56, GradeC: 40,
	StrongRating: 70, LowRating: 52,
}

// System is one style of play.
type System struct {
	Key, Name, Desc string
	// Shifts. Pace is possessions per game for the team (the game adds both
	// teams' shifts), ThreeMult multiplies the weight of three-point shots in
	// the shot-kind choice, StarExp is added to the shot-share exponent (stars
	// shoot more), Def is team defense rating points.
	Pace, ThreeMult, StarExp, Def float64
	// Bias is added to every player's score so that an average roster fits
	// every system about equally well (the systems differ in what they ask, not
	// in how easy they are). Tuned with `go run ./cmd/sim -mode coaches`.
	Bias float64
	// What it wants: rating weights (sum 1, input order of inputLabels) and badge keys.
	needs  [numInputs]float64
	Badges []string
}

const sysStarKey = "star" // the Star personality badge in a Badges list

// Systems lists the six systems in display order. Balanced first.
var Systems = []System{
	{Key: "balanced", Name: "Balanced", Desc: "No special style. Every player fits fine and nothing is asked of the roster.",
		ThreeMult: 1},
	{Key: "pace_space", Name: "Pace and Space", Desc: "Quick shots from three: more threes and a little more pace, a little less rim protection.",
		Pace: 1, ThreeMult: 1.30, Def: -0.5, Bias: 8,
		needs: [numInputs]float64{2: 0.6, 6: 0.2, 3: 0.2}, Badges: []string{"spot_up", "catch_and_fire", "deep_range"}},
	{Key: "run_gun", Name: "Run and Gun", Desc: "Push the ball every time: the fastest pace, a few more threes, and the price is weaker defense.",
		Pace: 2, ThreeMult: 1.10, Def: -0.5, Bias: 7,
		needs: [numInputs]float64{6: 0.5, 3: 0.3, 2: 0.2}, Badges: []string{"transition_pusher", "outlet_starter", "pickpocket"}},
	{Key: "inside_out", Name: "Inside-Out", Desc: "Feed the big men and kick out: fewer threes and a slower pace, solid at the rim.",
		Pace: -1, ThreeMult: 0.80, Bias: 9,
		needs: [numInputs]float64{0: 0.5, 4: 0.3, 3: 0.2}, Badges: []string{"post_craft", "power_inside", "rim_protector"}},
	{Key: "star_led", Name: "Star-Led", Desc: "The best scorers take most of the shots. Role players watch, and the defense gives a little.",
		ThreeMult: 1, StarExp: 0.25, Def: -0.5, Bias: 24,
		needs: [numInputs]float64{7: 0.4, 1: 0.2, 2: 0.2, 0: 0.2}, Badges: []string{sysStarKey, "off_the_dribble"}},
	{Key: "defense_first", Name: "Defense First", Desc: "Grind it out: a slow pace, fewer threes, less star shooting and the strongest team defense.",
		Pace: -2.5, ThreeMult: 0.95, StarExp: -0.15, Def: 1.0, Bias: 8,
		needs: [numInputs]float64{5: 0.6, 4: 0.2, 6: 0.2}, Badges: []string{"lockdown", "perimeter_wall", "rim_protector"}},
}

// SystemByKey finds a system (nil when the key is unknown).
func SystemByKey(key string) *System {
	for i := range Systems {
		if Systems[i].Key == key {
			return &Systems[i]
		}
	}
	return nil
}

// ValidSystemKey says whether key names a system.
func ValidSystemKey(key string) bool { return SystemByKey(key) != nil }

// balancedKey is the system used when nothing else applies.
const balancedKey = "balanced"

// isBalanced says the system asks nothing of the roster.
func (s *System) isBalanced() bool { return len(s.Badges) == 0 }

// PaceWord, ThreesWord, StarWord and DefenseWord describe the shifts in short words.
func (s *System) PaceWord() string {
	switch {
	case s.Pace >= 2.5:
		return "Much faster"
	case s.Pace > 0.5:
		return "Faster"
	case s.Pace <= -2.5:
		return "Much slower"
	case s.Pace < -0.5:
		return "Slower"
	}
	return "Normal"
}

func (s *System) ThreesWord() string {
	switch {
	case s.ThreeMult >= 1.2:
		return "Many more"
	case s.ThreeMult > 1.03:
		return "More"
	case s.ThreeMult <= 0.85:
		return "Many fewer"
	case s.ThreeMult < 0.97:
		return "Fewer"
	}
	return "Normal"
}

func (s *System) StarWord() string {
	switch {
	case s.StarExp >= 0.2:
		return "Much more"
	case s.StarExp > 0.03:
		return "More"
	case s.StarExp < -0.03:
		return "Less"
	}
	return "Normal"
}

func (s *System) DefenseWord() string {
	switch {
	case s.Def >= 2:
		return "Much better"
	case s.Def > 0.3:
		return "Better"
	case s.Def <= -1.5:
		return "Much worse"
	case s.Def < -0.3:
		return "Worse"
	}
	return "Normal"
}

// Wants lists what the system looks for, in plain words.
func (s *System) Wants() []string {
	out := []string{}
	type wi struct {
		i int
		w float64
	}
	var ws []wi
	for i, w := range s.needs {
		if w > 0 {
			ws = append(ws, wi{i, w})
		}
	}
	sort.SliceStable(ws, func(a, b int) bool { return ws[a].w > ws[b].w })
	for _, x := range ws {
		out = append(out, inputWords(x.i, true))
	}
	var names []string
	for _, k := range s.Badges {
		names = append(names, badgeName(k))
	}
	if len(names) > 0 {
		out = append(out, "Badges: "+strings.Join(names, ", "))
	}
	return out
}

// inputWords names a rating input ("Three-point rating", "Shot-taking tendency").
func inputWords(i int, capital bool) string {
	l := inputLabels[i]
	if i != 7 && i != 6 {
		l += " rating"
	}
	if !capital {
		l = strings.ToLower(l[:1]) + l[1:]
		if i == 0 {
			l = "inside-scoring rating"
		}
	}
	return l
}

func badgeName(key string) string {
	if key == sysStarKey {
		return personalityDefs[BStar].Name
	}
	for i := range badgeDefs {
		if badgeDefs[i].Key == key {
			return badgeDefs[i].Name
		}
	}
	return key
}

var badgeIndexByKey map[string]int

func badgeIdx(key string) int {
	if badgeIndexByKey == nil {
		badgeIndexByKey = map[string]int{}
		for i := range badgeDefs {
			badgeIndexByKey[badgeDefs[i].Key] = i
		}
	}
	if i, ok := badgeIndexByKey[key]; ok {
		return i
	}
	return -1
}

// Fit is one player's fit with a system.
type Fit struct {
	Score   float64
	Reasons []string
}

// Grade is A, B, C or D.
func (f Fit) Grade() string { return GradeOf(f.Score) }

// GradeOf turns a fit score into a letter.
func GradeOf(score float64) string {
	t := &SystemTuning
	switch {
	case score >= t.GradeA:
		return "A"
	case score >= t.GradeB:
		return "B"
	case score >= t.GradeC:
		return "C"
	}
	return "D"
}

// ratingValue is the system's weighted blend of the inputs.
func (s *System) ratingValue(in [numInputs]float64) float64 {
	v := 0.0
	for i, w := range s.needs {
		v += w * in[i]
	}
	return v
}

func clamp01(x float64) float64 { return max(0, min(1, x)) }

// fitScore is a player's score for the system (no reasons; cheap, used per game).
func (s *System) fitScore(in [numInputs]float64, bs BadgeSet) float64 {
	if s.isBalanced() {
		return SystemTuning.BalancedFit
	}
	t := &SystemTuning
	score := t.RatingPoints * clamp01((s.ratingValue(in)-t.RatingLo)/(t.RatingHi-t.RatingLo))
	pts := 0.0
	for _, k := range s.Badges {
		pts += t.BadgePoint * float64(s.badgeTier(k, bs))
	}
	return max(0, min(100, score+min(pts, t.BadgeMax)+s.Bias))
}

// badgeTier is the tier of a wanted badge in the set (the Star badge counts as Gold).
func (s *System) badgeTier(key string, bs BadgeSet) Tier {
	if key == sysStarKey {
		if bs.Star {
			return Gold
		}
		return NoTier
	}
	if i := badgeIdx(key); i >= 0 && i < len(bs.Tier) {
		return bs.Tier[i]
	}
	return NoTier
}

// FitFor is a player's full fit with the system: score and up to three reasons.
func (s *System) FitFor(p *Player, bs BadgeSet) Fit {
	in := inputsOf(p)
	f := Fit{Score: s.fitScore(in, bs)}
	if s.isBalanced() {
		f.Reasons = []string{"Balanced asks nothing special of anyone"}
		return f
	}
	t := &SystemTuning
	type held struct {
		name string
		tier Tier
	}
	var have []held
	for _, k := range s.Badges {
		if tr := s.badgeTier(k, bs); tr > NoTier {
			if k == sysStarKey {
				have = append(have, held{"", tr})
			} else {
				have = append(have, held{badgeName(k), tr})
			}
		}
	}
	sort.SliceStable(have, func(i, j int) bool { return have[i].tier > have[j].tier })
	for _, h := range have {
		if len(f.Reasons) == 2 {
			break
		}
		if h.name == "" {
			f.Reasons = append(f.Reasons, "Is a Star")
		} else {
			f.Reasons = append(f.Reasons, fmt.Sprintf("Has %s (%s)", h.name, h.tier.Name()))
		}
	}
	// The rating that matters most to the system.
	best := -1
	for i, w := range s.needs {
		if w > 0 && (best < 0 || w > s.needs[best]) {
			best = i
		}
	}
	// Also the weakest of the wanted inputs, for a warning.
	worst := -1
	for i, w := range s.needs {
		if w >= 0.2 && (worst < 0 || in[i] < in[worst]) {
			worst = i
		}
	}
	rv := s.ratingValue(in)
	if worst >= 0 && in[worst] < t.LowRating {
		f.Reasons = append(f.Reasons, fmt.Sprintf("Low %s for this system", inputWords(worst, false)))
	} else if best >= 0 && rv >= t.StrongRating {
		f.Reasons = append(f.Reasons, fmt.Sprintf("Strong %s for this system", inputWords(best, false)))
	}
	if len(have) == 0 && len(f.Reasons) < 3 {
		f.Reasons = append(f.Reasons, "No badge this system wants")
	}
	if len(f.Reasons) > 3 {
		f.Reasons = f.Reasons[:3]
	}
	return f
}
