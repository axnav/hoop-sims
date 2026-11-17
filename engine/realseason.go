package engine

import "fmt"

// SeasonLabel turns a season's ending year into the name people use:
// 2011 becomes "2010-11", 2026 becomes "2025-26", 2000 becomes "1999-00".
func SeasonLabel(season int) string {
	return fmt.Sprintf("%d-%02d", season-1, season%100)
}
