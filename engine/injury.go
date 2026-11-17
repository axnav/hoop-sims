package engine

// Injuries. Every number lives in InjuryTuning. Injuries draw from their own
// RNG stream (injuryOffset), never the game's, so with Options.NoInjuries the
// game results for a seed are identical to a league that has no injury code.

// Injury is a current injury. A nil *Injury on a Player means healthy.
type Injury struct {
	Name      string
	GamesLeft int
	Serious   bool // long absence plus a permanent athletic drop
}

// Injured reports whether the player is hurt right now.
func (p *Player) Injured() bool { return p.Injury != nil }

// injuryOffset keeps the injury streams apart from days (0-99), playoffs
// (1000+), offseason (5000+), AI trades (7000), schedule (9000) and
// proposals (20000+). Regular season day d uses injuryOffset+d; playoff day d
// uses injuryOffset+500+d.
const (
	injuryOffset        = 30000
	injuryPlayoffOffset = 500
)

// InjuryTuning is the config block for injuries.
var InjuryTuning = struct {
	// RatePer30 is the chance a player gets hurt in a game in which he plays
	// 30 minutes. It scales linearly with minutes played.
	RatePer30 float64
	// AgeStart/AgePerYear: risk multiplier is 1 + AgePerYear*(age-AgeStart),
	// kept within [AgeMin, AgeMax].
	AgeStart   int
	AgePerYear float64
	AgeMin     float64
	AgeMax     float64
	// SeriousShare is the share of injuries that are serious (about one or
	// two players per league per season).
	SeriousShare float64
	// Serious injuries: games out, and permanent rating drops.
	SeriousGamesMin, SeriousGamesMax int
	AthleticDropMin, AthleticDropMax int // Athleticism
	ReboundDropMin, ReboundDropMax   int // Rebounding
	// CarryOverGames: a serious injury with more than this many games left
	// survives the offseason.
	CarryOverGames int
}{
	RatePer30: 0.026,
	AgeStart:  27, AgePerYear: 0.03, AgeMin: 0.7, AgeMax: 1.5,
	SeriousShare:    0.004,
	SeriousGamesMin: 40, SeriousGamesMax: 82,
	AthleticDropMin: 2, AthleticDropMax: 4,
	ReboundDropMin: 0, ReboundDropMax: 1,
	CarryOverGames: 20,
}

// injuryType is one row of the table of ordinary injuries.
type injuryType struct {
	name   string
	lo, hi int     // games out
	weight float64 // relative frequency
}

// Roughly: 80% short (1-5), 17% medium (6-20), 3% long (21-60).
var injuryTable = []injuryType{
	{"Sprained ankle", 1, 5, 26}, {"Bruised thigh", 1, 3, 14}, {"Sore knee", 1, 5, 14},
	{"Back spasms", 1, 5, 10}, {"Illness", 1, 3, 10}, {"Jammed finger", 1, 4, 6},
	{"Strained hamstring", 6, 14, 7}, {"Sprained knee", 8, 20, 5}, {"Stress reaction", 10, 20, 3},
	{"Sprained wrist", 6, 12, 2},
	{"Fractured hand", 21, 40, 1.4}, {"Torn meniscus", 30, 60, 0.9}, {"Broken foot", 25, 50, 0.7},
}

var seriousTable = []string{"Torn ACL", "Torn Achilles"}

// InjuryLine describes one injured player.
type InjuryLine struct {
	PlayerID  int
	Name      string
	TeamID    int
	Injury    string
	GamesLeft int
	Serious   bool
}

func injuryLine(p *Player) InjuryLine {
	return InjuryLine{p.ID, p.Name, p.TeamID, p.Injury.Name, p.Injury.GamesLeft, p.Injury.Serious}
}

// InjuryReport lists injured players on one team, or league-wide for teamID < 0.
func (lg *League) InjuryReport(teamID int) []InjuryLine {
	out := []InjuryLine{}
	for _, t := range lg.Teams {
		if teamID >= 0 && t.ID != teamID {
			continue
		}
		for _, p := range t.Roster {
			if p.Injured() {
				out = append(out, injuryLine(p))
			}
		}
	}
	return out
}

// NewInjuriesLastDay lists injuries that happened on the most recent SimDay
// or SimPlayoffDay, for all teams (not saved).
func (lg *League) NewInjuriesLastDay() []InjuryLine {
	return append([]InjuryLine{}, lg.newInjuries...)
}

// afterGame heals both teams' injured players by one game (whether or not
// they would have played), then rolls for new injuries among those who played.
func (lg *League) afterGame(r *RNG, res *GameResult) {
	for _, tb := range []*TeamBox{res.Home, res.Away} {
		for _, p := range tb.Team.Roster {
			if p.Injury != nil {
				p.Injury.GamesLeft--
				if p.Injury.GamesLeft <= 0 {
					p.Injury = nil
				}
			}
		}
	}
	if lg.Options.NoInjuries {
		return
	}
	c := &InjuryTuning
	for _, tb := range []*TeamBox{res.Home, res.Away} {
		for _, l := range tb.Lines {
			p := l.Player
			if l.Min == 0 || p.Injury != nil {
				continue
			}
			age := 1 + c.AgePerYear*float64(p.Age-c.AgeStart)
			age = max(c.AgeMin, min(c.AgeMax, age))
			if !r.Chance(c.RatePer30 * float64(l.Min) / 30 * age) {
				continue
			}
			p.Injury = rollInjury(r, p)
			lg.newInjuries = append(lg.newInjuries, injuryLine(p))
			lg.newsInjury(p) // news.go
		}
	}
}

func rollInjury(r *RNG, p *Player) *Injury {
	c := &InjuryTuning
	if r.Chance(c.SeriousShare) {
		p.Ratings.Athleticism = clamp(p.Ratings.Athleticism-r.Between(c.AthleticDropMin, c.AthleticDropMax), 25, 99)
		p.Ratings.Rebounding = clamp(p.Ratings.Rebounding-r.Between(c.ReboundDropMin, c.ReboundDropMax), 25, 99)
		p.Potential = max(p.Potential, p.Overall())
		return &Injury{Name: Pick(r, seriousTable), GamesLeft: r.Between(c.SeriousGamesMin, c.SeriousGamesMax), Serious: true}
	}
	w := make([]float64, len(injuryTable))
	for i, t := range injuryTable {
		w[i] = t.weight
	}
	t := injuryTable[r.PickWeighted(w)]
	return &Injury{Name: t.name, GamesLeft: r.Between(t.lo, t.hi)}
}

// clearOffseasonInjuries heals everyone except long serious injuries.
func (lg *League) clearOffseasonInjuries() {
	fix := func(p *Player) {
		if p.Injury != nil && !(p.Injury.Serious && p.Injury.GamesLeft > InjuryTuning.CarryOverGames) {
			p.Injury = nil
		}
	}
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			fix(p)
		}
	}
	for _, p := range lg.FreeAgents {
		fix(p)
	}
}
