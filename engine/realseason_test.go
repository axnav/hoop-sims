package engine

import "testing"

func TestSeasonLabel(t *testing.T) {
	for season, want := range map[int]string{2011: "2010-11", 2026: "2025-26", 2000: "1999-00", 2010: "2009-10"} {
		if got := SeasonLabel(season); got != want {
			t.Errorf("SeasonLabel(%d) = %q, want %q", season, got, want)
		}
	}
}
