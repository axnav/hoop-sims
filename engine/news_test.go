package engine

import (
	"os"
	"strings"
	"testing"
)

func readFile(p string) ([]byte, error)  { return os.ReadFile(p) }
func writeFile(p string, b []byte) error { return os.WriteFile(p, b, 0o644) }

func TestNewsTrimmedAndIDsRise(t *testing.T) {
	old := NewsTuning.Kept
	NewsTuning.Kept = 20
	defer func() { NewsTuning.Kept = old }()
	lg := GenerateLeague(2)
	lg.StartSeason()
	lg.SimDays(30)
	if len(lg.News) != 20 {
		t.Fatalf("want exactly Kept=20 stories after a month, got %d", len(lg.News))
	}
	first := lg.News[0].ID
	if first <= 1 {
		t.Fatalf("expected old stories to be trimmed away, first ID %d", first)
	}
	for i := 1; i < len(lg.News); i++ {
		if lg.News[i].ID != lg.News[i-1].ID+1 {
			t.Fatalf("IDs not consecutive at %d: %d then %d", i, lg.News[i-1].ID, lg.News[i].ID)
		}
	}
	last := lg.News[len(lg.News)-1].ID
	lg.SimDays(5)
	if lg.News[len(lg.News)-1].ID < last {
		t.Fatal("IDs went backwards after trimming")
	}
}

func TestNewsContentAndFilters(t *testing.T) {
	lg := awardsFixture()
	if len(lg.News) == 0 {
		t.Fatal("no news in two seasons")
	}
	kinds := map[string]bool{}
	for _, it := range lg.News {
		kinds[it.Kind] = true
		if it.Text == "" || it.ID <= 0 || it.Stage == "" {
			t.Fatalf("bad story %+v", it)
		}
		if len(it.PlayerIDs) != len(it.Names) {
			t.Fatalf("names do not match players: %+v", it)
		}
		if (it.Stage == "season") != (it.Day >= 0) {
			t.Fatalf("day %d in stage %s", it.Day, it.Stage)
		}
		if strings.Contains(strings.ToLower(it.Text), "impact") {
			t.Fatalf("story mentions impact: %s", it.Text)
		}
	}
	for _, k := range []string{NwAward, NwInjury, NwStreak, NwLeague, NwPlayoffs, NwDraft, NwSigning} {
		if !kinds[k] {
			t.Errorf("no %s story in two seasons", k)
		}
	}
	for _, f := range NewsFilterKeys() {
		items, counts, total := lg.NewsFor(f, 5)
		if len(counts) != 8 || total != counts[f] || len(items) > 5 || len(items) > total {
			t.Fatalf("filter %s: %d items, total %d, counts %v", f, len(items), total, counts)
		}
		for i, it := range items {
			if i > 0 && it.ID >= items[i-1].ID {
				t.Fatalf("filter %s not newest first", f)
			}
			if !newsInFilter(&it, f, lg.UserTeamID) {
				t.Fatalf("filter %s returned a %s story", f, it.Kind)
			}
		}
	}
	if _, c, total := lg.NewsFor("nonsense", 0); total != c["all"] || total != len(lg.News) {
		t.Fatal("an unknown filter should behave as all")
	}
	if ValidNewsFilter("nonsense") || !ValidNewsFilter("mine") {
		t.Fatal("ValidNewsFilter wrong")
	}
	mine, _, _ := lg.NewsFor("mine", 0)
	for _, it := range mine {
		ok := false
		for _, tm := range it.TeamIDs {
			ok = ok || tm == lg.UserTeamID
		}
		if !ok {
			t.Fatalf("mine returned %+v", it)
		}
	}
}

// Playing the same league with and without reading the news must not matter:
// results for a seed do not depend on this feature (it draws no random numbers).
func TestNewsDoesNotChangeResults(t *testing.T) {
	a, b := GenerateLeague(1), GenerateLeague(1)
	a.StartSeason()
	b.StartSeason()
	NewsTuning.Kept = 1
	a.SimRegularSeason()
	NewsTuning.Kept = 400
	b.SimRegularSeason()
	for i := range a.Season.Wins {
		if a.Season.Wins[i] != b.Season.Wins[i] || a.Season.PointDiff[i] != b.Season.PointDiff[i] {
			t.Fatalf("team %d differs", i)
		}
	}
}
