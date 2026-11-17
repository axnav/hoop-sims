package engine

import (
	"fmt"
	"os"
	"path/filepath"
)

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

// realRename is one franchise change: from season `From` on, the franchise
// at table index Idx goes by Abbr, City and Name.
type realRename struct {
	From, Idx        int
	Abbr, City, Name string
}

// realRenames lists the renames between 2011 and 2026, oldest first. The
// franchise index (position in realTeamTable) never changes.
var realRenames = []realRename{
	{2013, 24, "BRK", "Brooklyn", "Nets"},
	{2014, 8, "NOP", "New Orleans", "Pelicans"},
	{2015, 17, "CHO", "Charlotte", "Hornets"},
}

// RealTeamInfo is a team's data abbreviation, city and name in one season.
type RealTeamInfo struct {
	Abbr, City, Name string
}

// realTeamsFor returns the 30 franchises as they were named in a season, in
// the fixed franchise order (the 2011 table).
func realTeamsFor(season int) []realTeam {
	out := append([]realTeam(nil), realTeamTable...)
	for _, rn := range realRenames {
		if season >= rn.From {
			out[rn.Idx].Abbr, out[rn.Idx].City, out[rn.Idx].Name = rn.Abbr, rn.City, rn.Name
		}
	}
	return out
}

// RealTeamAbbrevs returns the data abbreviation of each team index in 2011.
func RealTeamAbbrevs() []string { return RealTeamAbbrevsFor(firstRealSeason) }

// RealTeamAbbrevsFor returns the data abbreviation of each team index in a season.
func RealTeamAbbrevsFor(season int) []string {
	ts := realTeamsFor(season)
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Abbr
	}
	return out
}

// RealSeasonInfo describes one start season of the real league.
type RealSeasonInfo struct {
	Season int
	Label  string
	Teams  []RealTeamInfo
}

// RealSeasons lists the start seasons 2011 to 2026, oldest first, with each
// season's team names. It reads only the small team file (no league is built)
// and returns an error if a season has no teams in it.
func RealSeasons(dataDir string) ([]RealSeasonInfo, error) {
	rows, err := readCSV(filepath.Join(dataDir, "Team Abbrev.csv"))
	if err != nil {
		return nil, err
	}
	have := map[int]int{}
	for _, m := range rows {
		if m["lg"] == "NBA" && m["abbreviation"] != "NA" {
			have[int(num(m["season"]))]++
		}
	}
	if _, err := os.Stat(filepath.Join(dataDir, "Player Totals.csv")); err != nil {
		return nil, err
	}
	var out []RealSeasonInfo
	for s := firstRealSeason; s <= lastRealSeason; s++ {
		if have[s] < len(realTeamTable) {
			return nil, fmt.Errorf("no %d teams in %s", s, dataDir)
		}
		info := RealSeasonInfo{Season: s, Label: SeasonLabel(s)}
		for _, t := range realTeamsFor(s) {
			info.Teams = append(info.Teams, RealTeamInfo{t.Abbr, t.City, t.Name})
		}
		out = append(out, info)
	}
	return out, nil
}
