package engine

import "math"

// Detailed attributes and tendencies (HANDOFF section 24, phase 2).
//
// A player keeps his seven group Ratings as the stored truth. Each of the 22
// detailed attributes is shown as the rating of its group plus a stored offset
// (AttrOffsets), clamped to 25..99. The offsets of a group have a weighted mean
// of 0 (see AttrTuning), so the group rating is still the weighted average of
// its attributes and a rating change moves every attribute of the group.
// Tendencies (0..100) are stored as they are. Nothing here is read by the game
// sim yet (phase 3), so games come out exactly the same.
//
// Fictional players draw their offsets on random stream 160000, hashed per
// player (name and ID only), so no existing stream is touched.

const (
	NumAttrs     = 22
	NumTendency  = 7
	attrStream   = 160000
	attrFloor    = 25
	attrCeiling  = 99
	groupInside  = 0 // group numbers match Ratings.array()
	groupMid     = 1
	groupThree   = 2
	groupPlay    = 3
	groupReb     = 4
	groupDefense = 5
	groupAthl    = 6
)

// Attribute indexes into Player.AttrOffsets.
const (
	AttrCloseShot = iota
	AttrLayup
	AttrDunk
	AttrPostScoring
	AttrDrawFoul
	AttrMidRange
	AttrFreeThrow
	AttrThreePoint
	AttrPassAccuracy
	AttrBallHandle
	AttrPassVision
	AttrOffRebound
	AttrDefRebound
	AttrInteriorD
	AttrPerimeterD
	AttrSteal
	AttrBlock
	AttrSpeed
	AttrStrength
	AttrVertical
	AttrStamina
	AttrDurability
)

// Tendency indexes into Player.Tendencies.
const (
	TendThree = iota
	TendMidRange
	TendDrive
	TendPostUp
	TendPass
	TendStealGamble
	TendFoul
)

// AttrKey names one attribute: its JSON key, label and group.
type AttrKey struct {
	Key, Label, Group string
	GroupIdx          int
}

var attrKeys = [NumAttrs]AttrKey{
	{"close_shot", "Close Shot", "Inside", groupInside},
	{"layup", "Layup", "Inside", groupInside},
	{"dunk", "Dunk", "Inside", groupInside},
	{"post_scoring", "Post Scoring", "Inside", groupInside},
	{"draw_foul", "Draw Foul", "Inside", groupInside},
	{"mid_range", "Mid-Range", "Mid", groupMid},
	{"free_throw", "Free Throw", "Mid", groupMid},
	{"three_point", "Three-Point", "Three", groupThree},
	{"pass_accuracy", "Pass Accuracy", "Playmaking", groupPlay},
	{"ball_handle", "Ball Handle", "Playmaking", groupPlay},
	{"pass_vision", "Pass Vision", "Playmaking", groupPlay},
	{"off_rebound", "Offensive Rebound", "Rebounding", groupReb},
	{"def_rebound", "Defensive Rebound", "Rebounding", groupReb},
	{"interior_d", "Interior Defense", "Defense", groupDefense},
	{"perimeter_d", "Perimeter Defense", "Defense", groupDefense},
	{"steal", "Steal", "Defense", groupDefense},
	{"block", "Block", "Defense", groupDefense},
	{"speed", "Speed", "Athleticism", groupAthl},
	{"strength", "Strength", "Athleticism", groupAthl},
	{"vertical", "Vertical", "Athleticism", groupAthl},
	{"stamina", "Stamina", "Athleticism", groupAthl},
	{"durability", "Durability", "Athleticism", groupAthl},
}

var tendencyKeys = [NumTendency][2]string{
	{"three", "Three-Point Shots"},
	{"mid_range", "Mid-Range Shots"},
	{"drive", "Drive to the Rim"},
	{"post_up", "Post Up"},
	{"pass", "Pass"},
	{"steal_gamble", "Gamble for Steals"},
	{"foul", "Foul"},
}

// AttrTuning holds the numbers that shape attributes and tendencies.
var AttrTuning = AttrTuningT{
	// Weight of each attribute inside its group. Each group sums to 1; the
	// offsets of a group have this weighted mean of 0.
	Weights: [NumAttrs]float64{
		.25, .25, .20, .15, .15, // Inside: close shot, layup, dunk, post scoring, draw foul
		.60, .40, // Mid: mid-range, free throw
		1.0,           // Three: the one attribute never moves off its group
		.30, .40, .30, // Playmaking: pass accuracy, ball handle, pass vision
		.40, .60, // Rebounding: offensive, defensive
		.30, .30, .20, .20, // Defense: interior, perimeter, steal, block
		.25, .20, .25, .15, .15, // Athleticism: speed, strength, vertical, stamina, durability
	},
	// PosOffset is the typical offset by position before the weighted mean is
	// taken off (so only the shape counts): bigs finish and protect the rim,
	// guards are quick and handle the ball.
	PosOffset: map[Position][NumAttrs]float64{
		PG: {0, 3, -8, -10, 0, 0, 3, 0, 2, 8, 3, -2, -3, -8, 5, 5, -8, 8, -10, 0, 4, 0},
		SG: {0, 3, -3, -8, 0, 0, 3, 0, 0, 3, 0, -2, -2, -6, 4, 2, -5, 5, -6, 2, 2, 0},
		SF: {0, 1, 0, -3, 0, 0, 0, 0, 0, 0, 0, 0, 0, -1, 1, 0, 0, 0, 0, 1, 0, 0},
		PF: {1, -2, 4, 5, 0, 0, -1, 0, -1, -4, -1, 2, 2, 3, -3, -2, 3, -3, 5, 0, -2, 0},
		C:  {2, -3, 9, 10, 0, 0, -3, 0, -2, -8, -3, 4, 2, 8, -8, -4, 9, -9, 10, -2, -3, 0},
	},
	NoiseSD:    5.5, // per-player normal draw on each offset
	OffsetCap:  22,  // largest offset either way
	DunkAthl:   0.25,
	FTShoot:    0.3, // free throw follows (Three - Mid)
	StealAthl:  0.2, // steal and block follow (Athleticism - Defense)
	TendNoise:  9,   // per-player normal draw on each tendency
	RealScale:  70,  // real players: offset points per 1.0 of percentile difference
	RealCap:    30,
	RealMinMPG: 12, // stamina scale: minutes a game for 0 and 36 for 1
	RealMaxMPG: 36,
}

// AttrTuningT is the shape of AttrTuning.
type AttrTuningT struct {
	Weights                      [NumAttrs]float64
	PosOffset                    map[Position][NumAttrs]float64
	NoiseSD, OffsetCap           float64
	DunkAthl, FTShoot, StealAthl float64
	TendNoise                    float64
	RealScale, RealCap           float64
	RealMinMPG, RealMaxMPG       float64
}

// AttrKeys lists the 22 attributes in storage order.
func AttrKeys() []AttrKey { return append([]AttrKey(nil), attrKeys[:]...) }

// TendencyKeys lists the seven tendencies as (key, label), in storage order.
func TendencyKeys() [][2]string { return append([][2]string(nil), tendencyKeys[:]...) }

// AttrValue is one displayed attribute.
type AttrValue struct {
	Key, Label, Group string
	Value             int
}

// TendencyValue is one displayed tendency.
type TendencyValue struct {
	Key, Label string
	Value      int
}

// AttrOf is one attribute: his group rating plus the stored offset.
func (p *Player) AttrOf(i int) int {
	rt := p.Ratings.array()
	return clamp(rt[attrKeys[i].GroupIdx]+p.AttrOffsets[i], attrFloor, attrCeiling)
}

// Attributes returns the 22 attributes with their names, 25..99.
func (p *Player) Attributes() []AttrValue {
	out := make([]AttrValue, NumAttrs)
	for i, k := range attrKeys {
		out[i] = AttrValue{k.Key, k.Label, k.Group, p.AttrOf(i)}
	}
	return out
}

// TendencyValues returns the seven tendencies, 0..100.
func (p *Player) TendencyValues() []TendencyValue {
	out := make([]TendencyValue, NumTendency)
	for i, k := range tendencyKeys {
		out[i] = TendencyValue{k[0], k[1], clamp(p.Tendencies[i], 0, 100)}
	}
	return out
}

// hasAttributes reports whether the player already carries tendencies (they
// are never all zero once set).
func (p *Player) hasAttributes() bool {
	for _, v := range p.Tendencies {
		if v != 0 {
			return true
		}
	}
	return false
}

// ensureAttributes gives every player without them a set made by the
// fictional rule (old saves, real prospects without a season).
func (lg *League) ensureAttributes() {
	each := func(ps []*Player) {
		for _, p := range ps {
			if !p.hasAttributes() {
				fictionalAttributes(p)
			}
		}
	}
	for _, t := range lg.Teams {
		each(t.Roster)
	}
	each(lg.FreeAgents)
	each(lg.Retired)
	if lg.Draft != nil {
		each(lg.Draft.Pool)
	}
	for _, y := range sortedYears(lg.DraftClasses) {
		each(lg.DraftClasses[y])
	}
}

func sortedYears(m map[int][]*Player) []int {
	var ys []int
	for y := range m {
		ys = append(ys, y)
	}
	for i := 1; i < len(ys); i++ {
		for j := i; j > 0 && ys[j] < ys[j-1]; j-- {
			ys[j], ys[j-1] = ys[j-1], ys[j]
		}
	}
	return ys
}

func nameHash(s string) uint64 {
	h := uint64(1469598103934665603)
	for i := 0; i < len(s); i++ {
		h = (h ^ uint64(s[i])) * 1099511628211
	}
	return h
}

// groupBounds lists each group's attribute range [from, to).
var groupBounds = [7][2]int{{0, 5}, {5, 7}, {7, 8}, {8, 11}, {11, 13}, {13, 17}, {17, 22}}

// balanceOffsets turns raw offsets into integers whose weighted mean is
// within 0.5 of 0 in every group (the largest-weight member absorbs the
// rounding). The Three group is a single attribute and stays 0.
func balanceOffsets(raw [NumAttrs]float64, cap float64) [NumAttrs]int {
	var out [NumAttrs]int
	w := &AttrTuning.Weights
	for _, g := range groupBounds {
		lo, hi := g[0], g[1]
		if hi-lo == 1 {
			continue
		}
		mean := 0.0
		for i := lo; i < hi; i++ {
			mean += w[i] * raw[i]
		}
		big := lo
		for i := lo; i < hi; i++ {
			v := math.Max(-cap, math.Min(cap, raw[i]-mean))
			out[i] = int(math.Floor(v + 0.5))
			if w[i] > w[big] {
				big = i
			}
		}
		m := 0.0
		for i := lo; i < hi; i++ {
			m += w[i] * float64(out[i])
		}
		out[big] -= int(math.Floor(m/w[big] + 0.5))
	}
	return out
}

// fictionalAttributes sets offsets and tendencies from position, the player's
// own ratings and a draw on stream 160000 hashed from his name and ID.
func fictionalAttributes(p *Player) {
	t := &AttrTuning
	r := NewRNG(mix64(attrStream, uint64(p.ID), nameHash(p.Name)))
	rt := p.Ratings
	base := t.PosOffset[p.Pos]
	raw := base
	raw[AttrDunk] += t.DunkAthl * float64(rt.Athleticism-rt.Inside)
	raw[AttrPostScoring] -= t.DunkAthl * 0.6 * float64(rt.Athleticism-rt.Inside)
	raw[AttrFreeThrow] += t.FTShoot * float64(rt.Three-rt.Mid)
	raw[AttrSteal] += t.StealAthl * float64(rt.Athleticism-rt.Defense)
	raw[AttrBlock] += t.StealAthl * float64(rt.Athleticism-rt.Defense)
	for i := range raw {
		raw[i] += r.Normal(0, t.NoiseSD)
	}
	p.AttrOffsets = balanceOffsets(raw, t.OffsetCap)

	use := float64(p.ShotUsage())
	posAdj := func(a [5]float64) float64 { return a[p.Pos] }
	inside, mid, three := float64(rt.Inside), float64(rt.Mid), float64(rt.Three)
	play, def, ath := float64(rt.Playmaking), float64(rt.Defense), float64(rt.Athleticism)
	var tend [NumTendency]float64
	tend[TendThree] = 45 + 0.9*(three-(inside+mid)/2) + posAdj([5]float64{4, 8, 2, -8, -24}) + 0.2*(use-55)
	tend[TendMidRange] = 40 + 0.6*(mid-60) - 0.3*(three-60) + posAdj([5]float64{-4, 4, 4, 4, -2})
	tend[TendDrive] = 45 + 0.5*(inside-60) + 0.3*(ath-60) + posAdj([5]float64{6, 4, 3, -2, -6}) + 0.2*(use-55)
	tend[TendPostUp] = 32 + 0.4*(inside-60) + posAdj([5]float64{-26, -22, -6, 14, 28})
	tend[TendPass] = 35 + 0.8*(play-60) + posAdj([5]float64{12, 2, -2, -6, -10}) - 0.3*(use-55)
	tend[TendStealGamble] = 42 + 0.4*(def-60) + 0.3*(ath-60) + posAdj([5]float64{6, 3, 0, -3, -6})
	tend[TendFoul] = 45 + 0.2*(def-60) + posAdj([5]float64{-4, -2, 0, 5, 8})
	for i := range tend {
		v := tend[i] + r.Normal(0, t.TendNoise)
		p.Tendencies[i] = clamp(int(math.Floor(v+0.5)), 1, 99)
	}
}

// ---------------------------------------------------------------- real players

// attrDists are season distributions the real-player builder needs beyond the
// season model (measured over the same qualifying players).
type attrDists struct {
	ar3, pf36, drb dist
}

func (d *realData) attrDistsFor(season int) *attrDists {
	if a := d.adist[season]; a != nil {
		return a
	}
	if d.adist == nil {
		d.adist = map[int]*attrDists{}
	}
	var ar3, pf, drb []float64
	for _, r := range d.all[season] {
		if r.mp < RealTuning.MinQual {
			continue
		}
		ar3 = append(ar3, ar3Of(r))
		pf = append(pf, r.per36(r.pf))
		drb = append(drb, r.drbp)
	}
	a := &attrDists{newDist(ar3), newDist(pf), newDist(drb)}
	d.adist[season] = a
	return a
}

// ar3Of is the three-point attempt rate (x3p_ar).
func ar3Of(r *statRow) float64 {
	if r.fga <= 0 {
		return 0
	}
	return r.x3pa / r.fga
}

func unit(v, lo, hi float64) float64 { return clampF((v-lo)/(hi-lo), 0, 1) }

// realAttributes sets offsets and tendencies from a real stat row and the
// player's body, as percentiles within that season. Ratings are untouched.
func (d *realData) realAttributes(p *Player, r *statRow) {
	t := &AttrTuning
	m := d.model(r.season)
	ad := d.attrDistsFor(r.season)
	ci := d.career[r.pid]
	ht := unit(ci.ht, 70, 86) // 5'10 to 7'2
	if ci.ht == 0 {
		ht = map[Position]float64{PG: .2, SG: .35, SF: .5, PF: .7, C: .85}[p.Pos]
	}
	wt := unit(ci.wt, 170, 290)
	if ci.wt == 0 {
		wt = ht
	}
	big := p.Pos == PF || p.Pos == C
	ageF := unit(33-r.age, 0, 15)

	reg2 := m.reg2.pct(m.in2(r))
	ftp := m.ft.pct(m.inFT(r))
	a3 := m.a3.pct(r.per36(r.x3pa))
	a2 := m.a2.pct(r.per36(r.x2pa))
	ftr := m.ftr.pct(r.ftRate())
	orb := m.orb.pct(r.orbp)
	ast := m.ast.pct(r.astp)
	tov := 1 - m.tov.pct(r.tovp)
	stl := m.stl.pct(r.stlp)
	blk := m.blk.pct(r.blkp)
	dbpm := m.dbpm.pct(r.dbpm)
	ar3 := ad.ar3.pct(ar3Of(r))
	drb := 0.5*ad.drb.pct(r.drbp) + 0.5*unit(r.drbp, 8, 30)
	athl := 0.4*ftr + 0.3*a2 + 0.3*stl

	var s [NumAttrs]float64
	s[AttrCloseShot] = 0.7*reg2 + 0.3*ht
	s[AttrLayup] = 0.5*reg2 + 0.3*a2 + 0.2*ftr
	s[AttrDunk] = 0.25*ht + 0.25*orb + 0.4*athl + 0.1*ageF
	s[AttrPostScoring] = 0.4*ht + 0.3*(1-ar3) + 0.3*reg2
	s[AttrDrawFoul] = ftr
	s[AttrMidRange] = 0.5*reg2 + 0.3*ftp + 0.2*a3
	s[AttrFreeThrow] = ftp
	s[AttrPassAccuracy] = 0.5*ast + 0.5*tov
	s[AttrBallHandle] = 0.5*tov + 0.3*ast + 0.2*(1-ht)
	s[AttrPassVision] = ast
	s[AttrOffRebound] = orb
	s[AttrDefRebound] = drb
	s[AttrInteriorD] = 0.45*blk + 0.35*ht + 0.2*dbpm
	if big {
		s[AttrPerimeterD] = 0.25*stl + 0.25*dbpm + 0.5*(1-ht)
	} else {
		s[AttrPerimeterD] = 0.5*stl + 0.5*dbpm
	}
	s[AttrSteal] = stl
	s[AttrBlock] = blk
	s[AttrSpeed] = 0.8*(1-ht) + 0.1*stl + 0.1*ageF
	s[AttrStrength] = wt
	s[AttrVertical] = 0.4*blk + 0.4*orb + 0.2*ageF
	s[AttrStamina] = unit(mpgOf(r), t.RealMinMPG, t.RealMaxMPG)
	g, n := 0.0, 0.0
	for y := r.season - 2; y <= r.season; y++ {
		if x := d.all[y][r.pid]; x != nil {
			g += x.g
			n++
		}
	}
	s[AttrDurability] = unit(g/math.Max(n, 1)/82, 0.4, 1)

	var raw [NumAttrs]float64
	for _, gb := range groupBounds {
		mean := 0.0
		for i := gb[0]; i < gb[1]; i++ {
			mean += t.Weights[i] * s[i]
		}
		for i := gb[0]; i < gb[1]; i++ {
			raw[i] = t.RealScale * (s[i] - mean)
		}
	}
	p.AttrOffsets = balanceOffsets(raw, t.RealCap)

	// Tendencies: percentile 0..1 mapped to 5..95.
	tp := func(v float64) int { return clamp(int(math.Floor(5+90*clampF(v, 0, 1)+0.5)), 1, 99) }
	p.Tendencies[TendThree] = tp(ar3)
	p.Tendencies[TendMidRange] = tp(0.4*(1-ar3) + 0.3*(1-ftr) + 0.3*ftp)
	p.Tendencies[TendDrive] = tp(0.65*ftr + 0.35*a2)
	p.Tendencies[TendPostUp] = tp(0.45*ht + 0.3*(1-ar3) + 0.25*reg2)
	p.Tendencies[TendPass] = tp(ast)
	p.Tendencies[TendStealGamble] = tp(stl)
	p.Tendencies[TendFoul] = tp(ad.pf36.pct(r.per36(r.pf)))
}
