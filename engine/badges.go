package engine

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Badges, in the style of a console basketball game: 37 skill badges with five
// tiers (Bronze, Silver, Gold, Hall of Fame, Legend) and three personality
// badges (Leader, Mentor, Star) without tiers.
//
// A skill badge has a rating formula (single rating, the lower of two, or a
// weighted blend of the seven ratings and the shot-taking tendency), five
// thresholds, and a highest tier by position (position stands in for height).
// The tier comes from the visible ratings each time, so nothing is saved.
// Last season's statistics can lift a few badges one tier (Player.PerfBadges,
// awarded once at season end, never above the position limit).
//
// Every skill badge feeds one hook of the game sim with a share (see hookDefs).
// A player's points on a hook are the sum of tier x share, shaped by a
// diminishing power, and his effect is scale x (points - league mean), so a
// player with an average badge load plays exactly as before. Means are derived
// (never saved) from the current rosters and cached for one day of games.
// Personality badges come from a hashed stream (offset 100000) plus age,
// experience and overall. Draft prospects have no badges. The hidden Impact
// never plays a part: badges look at visible ratings only.

// Tier is the level of a skill badge.
type Tier int

const (
	NoTier Tier = iota
	Bronze
	Silver
	Gold
	HallOfFame
	Legend
)

func (t Tier) Key() string {
	return [...]string{"", "bronze", "silver", "gold", "hof", "legend"}[t]
}

// Name is the display name.
func (t Tier) Name() string {
	return [...]string{"", "Bronze", "Silver", "Gold", "Hall of Fame", "Legend"}[t]
}

// badgeOffset is the RNG stream for personality badges: one hashed stream per
// player (see RULES.md section 3).
const badgeOffset = 100000

// The sim hooks badges feed.
const (
	HRim       = iota // own make chance at the rim
	HMid              // own make chance on mid-range shots
	HThree            // own make chance on threes
	HFT               // own free-throw chance
	HFoul             // own chance to draw a foul at the rim
	HTov              // own turnover chance (lower is better)
	HAstPick          // weight for being the assister
	HUsage            // own share of the team's shots
	HClutch           // own make chance late in a close game
	HAstChance        // team assist chance
	HDef              // team defense rating
	HRimDef           // opponents' make chance at the rim
	HSteal            // opponents' turnover chance
	HOReb             // team offensive rebounding rating
	HDReb             // team defensive rebounding rating
	HPace             // team possessions per game
	HFoulDef          // opponents' chance to draw a foul at the rim
	numHooks
)

type hookDef struct {
	Name string // used in effect texts
	Team bool   // team hook: the minutes-weighted average of the players' effects
}

var hookDefs = [numHooks]hookDef{
	HRim:       {"shots at the rim", false},
	HMid:       {"mid-range shots", false},
	HThree:     {"three-pointers", false},
	HFT:        {"free throws", false},
	HFoul:      {"fouls drawn at the rim", false},
	HTov:       {"fewer turnovers on his possessions", false},
	HAstPick:   {"being the one who gets the assists", false},
	HUsage:     {"his share of the team's shots", false},
	HClutch:    {"shots late in a close game", false},
	HAstChance: {"the team's assist chance", true},
	HDef:       {"team defense", true},
	HRimDef:    {"defense at the rim", true},
	HSteal:     {"steals and forced turnovers", true},
	HOReb:      {"offensive rebounds", true},
	HDReb:      {"defensive rebounds", true},
	HPace:      {"the team's pace", true},
	HFoulDef:   {"fouls avoided on defense (fewer drawn by opponents at the rim)", true},
}

// BadgeTuning holds every number behind badges. Edit here.
var BadgeTuning = struct {
	// Power shapes a player's points on a hook (diminishing returns): points^Power.
	Power float64
	// Scale turns (shaped points - league mean) into the effect: make chance or
	// chance for shot hooks, rating points for HDef/HOReb/HDReb, possessions for
	// HPace, a weight multiplier for HAstPick and HUsage.
	Scale [numHooks]float64
	// Ceiling on the size of any one hook effect (positive and negative), per hook.
	Cap [numHooks]float64

	StarShot     float64 // Star: shot-share weight multiplier is 1 + this
	LeaderScale  float64 // Leader: team make chance per unit of (leader minutes share - league mean share)
	LeaderCap    float64
	MentorGrowth float64 // Mentor: teammates this age or younger grow this much faster
	MentorMaxAge int

	// Late and close: the last ClutchShare of regulation, or overtime, with the score within ClutchMargin.
	ClutchShare  float64
	ClutchMargin int

	// Personality badges: the condition, then the chance among those who qualify.
	LeaderYears, LeaderOverall int
	LeaderShare                float64
	MentorAge, MentorYears     int
	MentorShare                float64
	StarOverall                int
	StarShare                  float64

	// Performance boosts: games and minutes needed.
	PerfMinGP, PerfMinMinutes int
}{
	Power: 0.5,
	Scale: [numHooks]float64{
		HRim: 0.007, HMid: 0.011, HThree: 0.008, HFT: 0.010, HFoul: 0.009, HTov: 0.006,
		HAstPick: 0.10, HUsage: 0.08, HClutch: 0.015,
		HAstChance: 0.03, HDef: 2.2, HRimDef: 0.022, HSteal: 0.009, HOReb: 2.2, HDReb: 2.2, HPace: 1.5, HFoulDef: 0.03,
	},
	Cap: [numHooks]float64{
		HRim: 0.04, HMid: 0.04, HThree: 0.04, HFT: 0.06, HFoul: 0.08, HTov: 0.03,
		HAstPick: 0.4, HUsage: 0.3, HClutch: 0.06,
		HAstChance: 0.04, HDef: 4, HRimDef: 0.04, HSteal: 0.015, HOReb: 4, HDReb: 4, HPace: 3, HFoulDef: 0.05,
	},
	StarShot:    0.06,
	LeaderScale: 0.01, LeaderCap: 0.006,
	MentorGrowth: 0.05, MentorMaxAge: 24,
	ClutchShare: 0.12, ClutchMargin: 5,

	LeaderYears: 5, LeaderOverall: 62, LeaderShare: 0.25,
	MentorAge: 30, MentorYears: 8, MentorShare: 0.30,
	StarOverall: 80, StarShare: 0.70,

	PerfMinGP: 41, PerfMinMinutes: 1000,
}

// Inputs to a badge formula: the seven ratings (order of Ratings.array) and
// the shot-taking tendency.
const numInputs = 8

var inputLabels = [numInputs]string{"Inside scoring", "Mid-range", "Three-point", "Playmaking", "Rebounding", "Defense", "Athleticism", "Shot-taking tendency"}
var inputShort = map[string]int{"in": 0, "mid": 1, "thr": 2, "ply": 3, "reb": 4, "def": 5, "ath": 6, "use": 7}

func inputsOf(p *Player) [numInputs]float64 {
	a := p.Ratings.array()
	var out [numInputs]float64
	for i, v := range a {
		out[i] = float64(v)
	}
	out[7] = float64(p.ShotUsage())
	return out
}

type badgeDef struct {
	Key, Name, Group string
	w                [numInputs]float64 // weights (sum 1), or which inputs count when both is set
	both             bool               // the lower of the inputs with a weight
	thr              [5]int             // Bronze, Silver, Gold, Hall of Fame, Legend
	limit            [5]Tier            // highest tier by position PG, SG, SF, PF, C
	hook             int
	share            float64
}

// formula describes the rating formula in words.
func (d *badgeDef) formula() string {
	var parts []string
	for i, w := range d.w {
		if w > 0 {
			if d.both || w == 1 {
				parts = append(parts, inputLabels[i])
			} else {
				parts = append(parts, fmt.Sprintf("%s %.0f%%", inputLabels[i], w*100))
			}
		}
	}
	switch {
	case len(parts) == 1:
		return parts[0]
	case d.both:
		return strings.Join(parts, " and ") + " (the lower)"
	}
	return strings.Join(parts, " + ")
}

func (d *badgeDef) value(in [numInputs]float64) float64 {
	if d.both {
		v := 1000.0
		for i, w := range d.w {
			if w > 0 {
				v = math.Min(v, in[i])
			}
		}
		return v
	}
	v := 0.0
	for i, w := range d.w {
		v += w * in[i]
	}
	return v
}

// ratingTier is the tier the formula earns, before the position limit.
func (d *badgeDef) ratingTier(in [numInputs]float64) Tier {
	v := d.value(in)
	t := NoTier
	for i, th := range d.thr {
		if v >= float64(th) {
			t = Tier(i + 1)
		}
	}
	return t
}

// maxTier is the highest tier this position can reach.
func (d *badgeDef) maxTier(pos Position) Tier {
	if pos < PG || pos > C {
		pos = SF
	}
	return d.limit[pos]
}

func (d *badgeDef) formulaText(i int) string {
	return fmt.Sprintf("%s %d", d.formula(), d.thr[i])
}

// limitText says the position limits in one sentence ("" when none).
func (d *badgeDef) limitText() string {
	by := map[Tier][]string{}
	for pos := PG; pos <= C; pos++ {
		if t := d.limit[pos]; t < Legend {
			by[t] = append(by[t], pos.String())
		}
	}
	if len(by) == 0 {
		return ""
	}
	var ts []int
	for t := range by {
		ts = append(ts, int(t))
	}
	sort.Ints(ts)
	var parts []string
	for _, t := range ts {
		name := "no badge"
		if t > 0 {
			name = Tier(t).Name() + " at most"
		}
		parts = append(parts, name+" for "+strings.Join(by[Tier(t)], "/"))
	}
	return strings.Join(parts, ", ")
}

// spec is a compact badge definition used by the table below.
type spec struct {
	key, name, group, formula string
	hook                      int
	share                     float64
	limit                     string // five digits, PG SG SF PF C, highest tier
}

// parseFormula reads "in:.7 ath:.3" (weights), "min:in,reb" (the lower) or "in".
func parseFormula(s string) (w [numInputs]float64, both bool) {
	if strings.HasPrefix(s, "min:") {
		for _, k := range strings.Split(s[4:], ",") {
			w[inputShort[k]] = 1
		}
		return w, true
	}
	for _, f := range strings.Fields(s) {
		k, v, ok := strings.Cut(f, ":")
		if !ok {
			w[inputShort[k]] = 1
			continue
		}
		x, _ := strconv.ParseFloat(v, 64)
		w[inputShort[k]] = x
	}
	return w, false
}

const (
	all5   = "55555"
	bigs   = "02355" // inside game: PG none, SG Silver, SF Gold, PF and C Legend
	bigs2  = "12444"
	guards = "55410"
	quick  = "44420"
	noC    = "44440"
	wings  = "55543"
)

var badgeSpecs = []spec{
	// Finishing
	{"rim_finisher", "Rim Finisher", "finishing", "in:.7 ath:.3", HRim, 1.0, "44444"},
	{"power_inside", "Power Inside", "finishing", "min:in,reb", HRim, 0.7, "12455"},
	{"foul_magnet", "Foul Magnet", "finishing", "in:.5 ath:.5", HFoul, 1.0, "44444"},
	{"soft_touch", "Soft Touch", "finishing", "in:.5 mid:.5", HRim, 0.6, "44444"},
	{"lob_target", "Lob Target", "finishing", "ath:.6 in:.4", HRim, 0.5, bigs2},
	{"post_craft", "Post Craft", "finishing", "in:.7 ply:.3", HRim, 0.6, bigs},
	{"above_the_rim", "Above the Rim", "finishing", "ath:.8 in:.2", HRim, 0.6, bigs2},
	{"contact_finisher", "Contact Finisher", "finishing", "min:in,ath", HFoul, 0.7, "44444"},
	// Shooting
	{"spot_up", "Spot-Up", "shooting", "thr", HThree, 1.0, "44444"},
	{"midrange_maestro", "Midrange Maestro", "shooting", "mid", HMid, 1.0, "44444"},
	{"deep_range", "Deep Range", "shooting", "thr:.7 mid:.3", HThree, 0.8, noC},
	{"catch_and_fire", "Catch and Fire", "shooting", "thr:.8 ath:.2", HThree, 0.8, noC},
	{"off_the_dribble", "Off the Dribble", "shooting", "mid:.3 thr:.3 ply:.2 use:.2", HMid, 0.6, quick},
	{"corner_specialist", "Corner Specialist", "shooting", "min:thr,mid", HThree, 0.6, wings},
	{"clutch_shooter", "Clutch Shooter", "shooting", "thr:.35 mid:.35 use:.3", HClutch, 1.0, "44444"},
	{"charity_stripe", "Charity Stripe", "shooting", "mid:.8 thr:.2", HFT, 1.0, "44444"},
	// Playmaking
	{"floor_general", "Floor General", "playmaking", "ply", HAstPick, 1.0, guards},
	{"ball_security", "Ball Security", "playmaking", "ply:.7 mid:.3", HTov, 1.0, "55444"},
	{"lob_passer", "Lob Passer", "playmaking", "ply:.7 ath:.3", HAstChance, 0.7, guards},
	{"needle_threader", "Needle Threader", "playmaking", "ply:.8 mid:.2", HAstChance, 1.0, guards},
	{"transition_pusher", "Transition Pusher", "playmaking", "ath:.5 ply:.5", HPace, 0.8, quick},
	{"breakdown_dribbler", "Breakdown Dribbler", "playmaking", "ply:.5 ath:.3 use:.2", HUsage, 0.6, guards},
	{"steady_hands", "Steady Hands", "playmaking", "ply:.6 def:.4", HTov, 0.8, "44444"},
	// Defense
	{"lockdown", "Lockdown", "defense", "def:.7 ath:.3", HDef, 1.0, "55444"},
	{"rim_protector", "Rim Protector", "defense", "def:.6 reb:.4", HRimDef, 1.0, bigs},
	{"pickpocket", "Pickpocket", "defense", "def:.5 ply:.25 ath:.25", HSteal, 1.0, quick},
	{"passing_lane", "Passing Lane", "defense", "def:.5 ply:.5", HSteal, 0.8, "44444"},
	{"perimeter_wall", "Perimeter Wall", "defense", "min:def,ath", HDef, 0.8, noC},
	{"post_anchor", "Post Anchor", "defense", "def:.5 reb:.25 in:.25", HRimDef, 0.7, bigs},
	{"help_defender", "Help Defender", "defense", "def:.6 ply:.4", HDef, 0.5, "44444"},
	{"charge_taker", "Charge Taker", "defense", "def:.6 ply:.2 reb:.2", HFoulDef, 1.0, "44444"},
	// Rebounding
	{"glass_cleaner", "Glass Cleaner", "rebounding", "reb", HOReb, 1.0, "12455"},
	{"box_out_boss", "Box-Out Boss", "rebounding", "reb:.7 def:.3", HDReb, 1.0, bigs},
	{"tip_master", "Tip Master", "rebounding", "reb:.5 ath:.5", HOReb, 0.7, bigs2},
	{"outlet_starter", "Outlet Starter", "rebounding", "reb:.5 ply:.5", HPace, 0.6, "44444"},
	// General
	{"motor", "Motor", "general", "ath:.5 def:.25 reb:.25", HDReb, 0.5, "44444"},
	{"iron_will", "Iron Will", "general", "in:.14 mid:.14 thr:.14 ply:.14 reb:.14 def:.15 ath:.15", HClutch, 0.6, "44444"},
}

var badgeDefs []badgeDef

// numSkillBadges is the number of skill badges.
var numSkillBadges = len(badgeSpecs)

func init() {
	for _, s := range badgeSpecs {
		d := badgeDef{Key: s.key, Name: s.name, Group: s.group, hook: s.hook, share: s.share}
		d.w, d.both = parseFormula(s.formula)
		for i, c := range s.limit {
			d.limit[i] = Tier(c - '0')
		}
		if th, ok := badgeThresholds[s.key]; ok {
			d.thr = th
		} else {
			d.thr = [5]int{60, 70, 80, 90, 97}
		}
		badgeDefs = append(badgeDefs, d)
	}
	t := &BadgeTuning
	personalityDefs[BLeader].Rule = fmt.Sprintf("A respected veteran (%d+ seasons, overall %d+)", t.LeaderYears, t.LeaderOverall)
	personalityDefs[BLeader].Effect = "A small team boost in make chance while he is on the floor, against the league average"
	personalityDefs[BMentor].Rule = fmt.Sprintf("An old hand who teaches (age %d+, %d+ seasons)", t.MentorAge, t.MentorYears)
	personalityDefs[BMentor].Effect = fmt.Sprintf("Teammates aged %d and under grow %.0f%% faster (one Mentor is enough, no stacking)", t.MentorMaxAge, t.MentorGrowth*100)
	personalityDefs[BStar].Rule = fmt.Sprintf("A franchise-level talent (overall %d+)", t.StarOverall)
	personalityDefs[BStar].Effect = fmt.Sprintf("Takes %.0f%% more of the team's shots", t.StarShot*100)
}

// Performance boosts: last season's numbers can lift one badge one tier.
type perfRule struct {
	Key   string
	Label string
	// value returns the number to compare (ok false = not enough volume).
	value func(*PlayerSeason) (v float64, ok bool)
	thr   float64
	below bool // lower is better
}

func rate(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

var perfRules = []perfRule{
	{"charity_stripe", "Free-throw percentage (150+ attempts)", func(p *PlayerSeason) (float64, bool) { return rate(p.FTM, p.FTA) * 100, p.FTA >= 150 }, 88, false},
	{"foul_magnet", "Free throws attempted per 36 minutes", func(p *PlayerSeason) (float64, bool) { return per36(p.FTA, p.Min), true }, 5.6, false},
	{"floor_general", "Assists per 36 minutes", func(p *PlayerSeason) (float64, bool) { return per36(p.Ast, p.Min), true }, 8.0, false},
	{"glass_cleaner", "Rebounds per 36 minutes", func(p *PlayerSeason) (float64, bool) { return per36(p.Reb, p.Min), true }, 11.3, false},
	{"deep_range", "Three-point percentage (150+ attempts)", func(p *PlayerSeason) (float64, bool) { return rate(p.TPM, p.TPA) * 100, p.TPA >= 150 }, 42, false},
	{"catch_and_fire", "Threes made per 36 minutes", func(p *PlayerSeason) (float64, bool) { return per36(p.TPM, p.Min), true }, 3.7, false},
	{"rim_finisher", "Field-goal percentage (500+ attempts)", func(p *PlayerSeason) (float64, bool) { return rate(p.FGM, p.FGA) * 100, p.FGA >= 500 }, 54, false},
	{"ball_security", "Turnovers per 36 minutes (with 5+ assists per 36)", func(p *PlayerSeason) (float64, bool) { return per36(p.Tov, p.Min), per36(p.Ast, p.Min) >= 5 }, 1.6, true},
}

func per36(v, min int) float64 {
	if min <= 0 {
		return 0
	}
	return float64(v) * 36 / float64(min)
}

func (r *perfRule) earned(ps *PlayerSeason) bool {
	v, ok := r.value(ps)
	if !ok {
		return false
	}
	if r.below {
		return v <= r.thr
	}
	return v >= r.thr
}

func perfRuleFor(key string) *perfRule {
	for i := range perfRules {
		if perfRules[i].Key == key {
			return &perfRules[i]
		}
	}
	return nil
}

func (r *perfRule) text() string {
	cmp := "at least"
	if r.below {
		cmp = "at most"
	}
	return fmt.Sprintf("%s %s %g last season (%d games, %d minutes)", r.Label, cmp, r.thr, BadgeTuning.PerfMinGP, BadgeTuning.PerfMinMinutes)
}

// Personality badges.
const (
	BLeader = iota
	BMentor
	BStar
	numPersonality
)

type personalityDef struct {
	Key, Name, Rule, Effect string
}

var personalityDefs = [numPersonality]personalityDef{
	BLeader: {Key: "leader", Name: "Leader"},
	BMentor: {Key: "mentor", Name: "Mentor"},
	BStar:   {Key: "star", Name: "Star"},
}

// BadgeSet is a player's badges at one moment. Tier is what counts (rating
// tier, plus a performance boost, never above the position limit).
type BadgeSet struct {
	Tier                 []Tier
	RatingTier           []Tier
	Leader, Mentor, Star bool
}

// personality works out the three personality badges. lg may be nil (none).
func (lg *League) personality(p *Player) (leader, mentor, star bool) {
	if lg == nil {
		return
	}
	t := &BadgeTuning
	r := NewRNG(mix64(lg.Seed, uint64(badgeOffset), uint64(p.ID)))
	x1, x2, x3 := r.Float64(), r.Float64(), r.Float64()
	ovr := p.Overall()
	leader = p.Experience >= t.LeaderYears && ovr >= t.LeaderOverall && x1 < t.LeaderShare
	mentor = p.Age >= t.MentorAge && p.Experience >= t.MentorYears && x2 < t.MentorShare
	star = ovr >= t.StarOverall && x3 < t.StarShare
	return
}

// badgeSet computes a player's badges (lg may be nil: no personality badges).
// It does not check for prospects; BadgesOf does.
func (lg *League) badgeSet(p *Player) BadgeSet {
	n := len(badgeDefs)
	bs := BadgeSet{Tier: make([]Tier, n), RatingTier: make([]Tier, n)}
	in := inputsOf(p)
	for i := range badgeDefs {
		d := &badgeDefs[i]
		limit := d.maxTier(p.Pos)
		rt := min(d.ratingTier(in), limit)
		bs.RatingTier[i] = rt
		bs.Tier[i] = rt
		if p.PerfBadges[d.Key] > 0 {
			bs.Tier[i] = min(rt+1, limit)
		}
	}
	bs.Leader, bs.Mentor, bs.Star = lg.personality(p)
	return bs
}

// hookF is the player's shaped points on every hook: (sum of tier x share) ^ Power.
func (bs BadgeSet) hookF() [numHooks]float64 {
	var pts [numHooks]float64
	for i, t := range bs.Tier {
		if t > NoTier {
			d := &badgeDefs[i]
			pts[d.hook] += float64(t) * d.share
		}
	}
	for h := range pts {
		pts[h] = math.Pow(pts[h], BadgeTuning.Power)
	}
	return pts
}

// IsProspect says whether p is a draft prospect (in the draft pool or a stored
// future class). Prospects have no badges.
func (lg *League) IsProspect(p *Player) bool {
	if lg.Draft != nil {
		for _, q := range lg.Draft.Pool {
			if q.ID == p.ID {
				return true
			}
		}
	}
	for _, cls := range lg.DraftClasses {
		for _, q := range cls {
			if q.ID == p.ID {
				return true
			}
		}
	}
	return false
}

// Count is the number of badges in the set.
func (bs BadgeSet) Count() int {
	n := 0
	for _, t := range bs.Tier {
		if t > NoTier {
			n++
		}
	}
	for _, b := range []bool{bs.Leader, bs.Mentor, bs.Star} {
		if b {
			n++
		}
	}
	return n
}

// Badge is one badge shown for a player.
type Badge struct {
	Key, Name, Group string
	Tier             Tier   // NoTier for personality badges
	Kind             string // rating, performance or personality
	Rule, Effect     string
	Next             string // what the next tier needs ("" at the top or the position limit)
}

func (d *badgeDef) effectText() string {
	return fmt.Sprintf("Helps %s. Higher tiers help more; a player with more badges than average gains, fewer loses.",
		hookDefs[d.hook].Name)
}

func (d *badgeDef) allRules() string {
	var th []string
	for _, x := range d.thr {
		th = append(th, strconv.Itoa(x))
	}
	s := fmt.Sprintf("%s at %s (Bronze, Silver, Gold, Hall of Fame, Legend)", d.formula(), strings.Join(th, " / "))
	if l := d.limitText(); l != "" {
		s += "; position limit: " + l
	}
	if r := perfRuleFor(d.Key); r != nil {
		s += "; last season's stats can add one tier: " + r.text()
	}
	return s
}

// BadgesOf lists the player's badges, Legend first down to Bronze, then
// personality. Draft prospects get none.
func (lg *League) BadgesOf(p *Player) []Badge {
	out := []Badge{}
	if p == nil || lg.IsProspect(p) {
		return out
	}
	bs := lg.badgeSet(p)
	for i := range badgeDefs {
		d := &badgeDefs[i]
		t := bs.Tier[i]
		if t == NoTier {
			continue
		}
		b := Badge{Key: d.Key, Name: d.Name, Group: d.Group, Tier: t, Kind: "rating", Effect: d.effectText()}
		if bs.RatingTier[i] >= t {
			b.Rule = d.formulaText(int(t) - 1)
		} else {
			b.Kind = "performance"
			b.Rule = "Rating tier plus one for last season: " + perfRuleFor(d.Key).text()
		}
		if t < d.maxTier(p.Pos) {
			b.Next = t.Next().Name() + " at " + d.formulaText(int(t))
		}
		out = append(out, b)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Tier > out[j].Tier })
	for i, on := range []bool{bs.Leader, bs.Mentor, bs.Star} {
		if on {
			d := personalityDefs[i]
			out = append(out, Badge{Key: d.Key, Name: d.Name, Group: "personality", Kind: "personality", Rule: d.Rule, Effect: d.Effect})
		}
	}
	return out
}

// Next is the tier above (Legend stays Legend).
func (t Tier) Next() Tier { return min(t+1, Legend) }

// BadgeCount is how many badges BadgesOf would list (0 for prospects).
func (lg *League) BadgeCount(p *Player) int {
	if p == nil || lg.IsProspect(p) {
		return 0
	}
	return lg.badgeSet(p).Count()
}

// HasBadge says whether the player has the badge (any tier) with this key.
func (lg *League) HasBadge(p *Player, key string) bool {
	for _, b := range lg.BadgesOf(p) {
		if b.Key == key {
			return true
		}
	}
	return false
}

// CatalogueEntry describes one badge in the catalogue.
type CatalogueEntry struct {
	Key, Name, Group, Kind, Rule, Effect string
}

// BadgeCatalogue lists every badge: the skill badges, then the personality badges.
func BadgeCatalogue() []CatalogueEntry {
	var out []CatalogueEntry
	for i := range badgeDefs {
		d := &badgeDefs[i]
		kind := "rating"
		if perfRuleFor(d.Key) != nil {
			kind = "performance"
		}
		out = append(out, CatalogueEntry{d.Key, d.Name, d.Group, kind, d.allRules(), d.effectText()})
	}
	for _, d := range personalityDefs {
		out = append(out, CatalogueEntry{d.Key, d.Name, "personality", "personality", d.Rule, d.Effect})
	}
	return out
}

// ValidBadgeKey says whether key names a badge.
func ValidBadgeKey(key string) bool {
	for _, c := range BadgeCatalogue() {
		if c.Key == key {
			return true
		}
	}
	return false
}

// awardPerfBadges is called once when the regular season ends: it replaces
// every player's saved performance boosts with the ones last season's
// statistics earned (value 1 = one tier up).
func (lg *League) awardPerfBadges() {
	t := &BadgeTuning
	for _, p := range lg.allPlayers() {
		p.PerfBadges = nil
		ps := lg.Season.Stats[p.ID]
		if ps == nil || ps.GP < t.PerfMinGP || ps.Min < t.PerfMinMinutes {
			continue
		}
		for i := range perfRules {
			if perfRules[i].earned(ps) {
				if p.PerfBadges == nil {
					p.PerfBadges = map[string]int{}
				}
				p.PerfBadges[perfRules[i].Key] = 1
			}
		}
	}
}

// mentorTeams marks the teams that have a Mentor on the roster right now.
func (lg *League) mentorTeams() map[int]bool {
	out := map[int]bool{}
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			if _, m, _ := lg.personality(p); m {
				out[t.ID] = true
				break
			}
		}
	}
	return out
}

// growthMult is the growth multiplier for a player: 1 + MentorGrowth when he
// is young enough and a teammate is a Mentor. mentors is from mentorTeams; one
// Mentor is enough, they do not stack.
func (lg *League) growthMult(p *Player, mentors map[int]bool) float64 {
	t := &BadgeTuning
	if p.Age > t.MentorMaxAge || p.TeamID < 0 || !mentors[p.TeamID] {
		return 1
	}
	// A Mentor is at least MentorAge, so he is never young enough to mentor himself.
	return 1 + t.MentorGrowth
}

// badgeMeans is the league average of every hook (shaped points, minutes
// weighted over players who get minutes) plus the average share of minutes
// played by Leaders. Derived from the current rosters, never saved.
type badgeMeans struct {
	hook   [numHooks]float64
	leader float64
}

// hookWeights says how much each player's badge points on each hook count in
// the league mean: the share of the team's shots (or of one shot kind, or of
// the assists, or of the minutes for team hooks) he gets, before badge effects.
func hookWeights(pms []PlayerMinutes) [][numHooks]float64 {
	n := len(pms)
	out := make([][numHooks]float64, n)
	shoot, ast := make([]float64, n), make([]float64, n)
	kinds := make([][3]float64, n)
	var sTot, aTot, mTot float64
	for i, pm := range pms {
		rt := pm.Player.Effective()
		top := max(rt.Inside, rt.Mid, rt.Three)
		second := rt.Inside + rt.Mid + rt.Three - top - min(rt.Inside, rt.Mid, rt.Three)
		scoring := (float64(top) + float64(second)) / 2
		fm := float64(pm.Min)
		shoot[i] = fm * math.Pow(scoring, shootExp) * (0.8 + float64(pm.Player.ShotUsage())/250)
		ast[i] = fm * math.Pow(float64(rt.Playmaking), 2.5)
		in, mid, three := float64(rt.Inside), float64(rt.Mid), float64(rt.Three)
		k := [3]float64{in * in, mid * mid * midShotWeight, three * three * threeShotWt}
		ks := k[0] + k[1] + k[2]
		kinds[i] = [3]float64{k[0] / ks, k[1] / ks, k[2] / ks}
		sTot += shoot[i]
		aTot += ast[i]
		mTot += fm
	}
	for i, pm := range pms {
		s := shoot[i] / sTot
		w := &out[i]
		w[HRim], w[HMid], w[HThree] = s*kinds[i][0], s*kinds[i][1], s*kinds[i][2]
		w[HFT], w[HFoul] = w[HRim], w[HRim]
		w[HTov], w[HClutch], w[HUsage] = s, s, s
		w[HAstPick] = ast[i] / aTot
		m := float64(pm.Min) / mTot
		for _, h := range []int{HAstChance, HDef, HRimDef, HSteal, HOReb, HDReb, HPace, HFoulDef} {
			w[h] = m
		}
	}
	return out
}

// badgeMeansNow returns the cached means, building them when missing. SimDay and
// the playoff day clear the cache first, so the means are a function of the
// state at the start of the day (and the same after a save and load).
func (lg *League) badgeMeansNow() *badgeMeans {
	if lg.bmeans != nil {
		return lg.bmeans
	}
	m := &badgeMeans{}
	teams := 0.0
	var den [numHooks]float64
	for _, t := range lg.Teams {
		pms := t.EffectiveMinutes()
		if len(pms) == 0 {
			continue
		}
		teams++
		ws := hookWeights(pms)
		var tot float64
		for _, pm := range pms {
			tot += float64(pm.Min)
		}
		for i, pm := range pms {
			f := lg.badgeSet(pm.Player).hookF()
			for h := range f {
				m.hook[h] += ws[i][h] * f[h]
				den[h] += ws[i][h]
			}
			if l, _, _ := lg.personality(pm.Player); l {
				m.leader += float64(pm.Min) / tot
			}
		}
	}
	if teams > 0 {
		for h := range m.hook {
			if den[h] > 0 {
				m.hook[h] /= den[h]
			}
		}
		m.leader /= teams
	}
	lg.bmeans = m
	return m
}

func clampf(v, c float64) float64 { return math.Max(-c, math.Min(c, v)) }

// sideBadges are the per-game badge numbers for one team, computed once.
type sideBadges struct {
	kind                       [][3]float64 // per player: make-chance effect at rim, mid, three
	ftAdd, foulAdd, tovCut     []float64
	astMult, usage             []float64
	clutch                     []float64
	astT, defT, rimT, stealT   float64 // team effects
	orebT, drebT, paceT, foulT float64
	leader                     float64
}

func newSideBadges(n int) *sideBadges {
	sb := &sideBadges{kind: make([][3]float64, n), ftAdd: make([]float64, n), foulAdd: make([]float64, n),
		tovCut: make([]float64, n), astMult: make([]float64, n), usage: make([]float64, n), clutch: make([]float64, n)}
	for i := range sb.astMult {
		sb.astMult[i], sb.usage[i] = 1, 1
	}
	return sb
}

// add fills the numbers for player index i (minutes fm) from his badges,
// centred on the league means.
func (sb *sideBadges) add(bs BadgeSet, m *badgeMeans, i int, fm, totMin float64) {
	t := &BadgeTuning
	f := bs.hookF()
	eff := func(h int) float64 { return clampf(t.Scale[h]*(f[h]-m.hook[h]), t.Cap[h]) }
	sb.kind[i][0] = eff(HRim)
	sb.kind[i][1] = eff(HMid)
	sb.kind[i][2] = eff(HThree)
	sb.ftAdd[i] = eff(HFT)
	sb.foulAdd[i] = eff(HFoul)
	sb.tovCut[i] = eff(HTov)
	sb.astMult[i] = 1 + eff(HAstPick)
	sb.usage[i] = 1 + eff(HUsage)
	sb.clutch[i] = eff(HClutch)
	w := fm / totMin
	sb.astT += w * eff(HAstChance)
	sb.defT += w * eff(HDef)
	sb.rimT += w * eff(HRimDef)
	sb.stealT += w * eff(HSteal)
	sb.orebT += w * eff(HOReb)
	sb.drebT += w * eff(HDReb)
	sb.paceT += w * eff(HPace)
	sb.foulT += w * eff(HFoulDef)
	if bs.Star {
		sb.usage[i] *= 1 + t.StarShot
	}
	if bs.Leader {
		sb.leader += w
	}
}

// finish turns the Leader minutes share into the team boost.
func (sb *sideBadges) finish(m *badgeMeans) {
	t := &BadgeTuning
	sb.leader = clampf(t.LeaderScale*(sb.leader-m.leader), t.LeaderCap)
}
