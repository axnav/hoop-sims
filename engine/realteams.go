package engine

// realTeam describes one franchise as it stood in 2010-11.
type realTeam struct {
	Abbr, City, Name string
	East             bool
}

// realTeamTable lists the 30 teams of 2010-11 keyed by that season's data
// abbreviation. The order puts the West on indices 0-14 and the East on 15-29
// (see Conference in season.go), alphabetical by abbreviation within each.
var realTeamTable = []realTeam{
	{"DAL", "Dallas", "Mavericks", false}, {"DEN", "Denver", "Nuggets", false},
	{"GSW", "Golden State", "Warriors", false}, {"HOU", "Houston", "Rockets", false},
	{"LAC", "Los Angeles", "Clippers", false}, {"LAL", "Los Angeles", "Lakers", false},
	{"MEM", "Memphis", "Grizzlies", false}, {"MIN", "Minnesota", "Timberwolves", false},
	{"NOH", "New Orleans", "Hornets", false}, {"OKC", "Oklahoma City", "Thunder", false},
	{"PHO", "Phoenix", "Suns", false}, {"POR", "Portland", "Trail Blazers", false},
	{"SAC", "Sacramento", "Kings", false}, {"SAS", "San Antonio", "Spurs", false},
	{"UTA", "Utah", "Jazz", false},

	{"ATL", "Atlanta", "Hawks", true}, {"BOS", "Boston", "Celtics", true},
	{"CHA", "Charlotte", "Bobcats", true}, {"CHI", "Chicago", "Bulls", true},
	{"CLE", "Cleveland", "Cavaliers", true}, {"DET", "Detroit", "Pistons", true},
	{"IND", "Indiana", "Pacers", true}, {"MIA", "Miami", "Heat", true},
	{"MIL", "Milwaukee", "Bucks", true}, {"NJN", "New Jersey", "Nets", true},
	{"NYK", "New York", "Knicks", true}, {"ORL", "Orlando", "Magic", true},
	{"PHI", "Philadelphia", "76ers", true}, {"TOR", "Toronto", "Raptors", true},
	{"WAS", "Washington", "Wizards", true},
}

// RealTeamAbbrevs returns the data abbreviation of each team index.
func RealTeamAbbrevs() []string {
	out := make([]string, len(realTeamTable))
	for i, t := range realTeamTable {
		out[i] = t.Abbr
	}
	return out
}
