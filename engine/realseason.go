package engine

import "fmt"

// SeasonLabel turns a season's ending year into the name people use:
// 2011 becomes "2010-11", 2026 becomes "2025-26", 2000 becomes "1999-00".
func SeasonLabel(season int) string {
	// TODO(human): build the label from the starting year (season - 1) and the
	// last two digits of the ending year. The line below is a placeholder.
	return fmt.Sprint(season)
}
