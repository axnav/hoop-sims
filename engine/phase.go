package engine

import (
	"errors"
	"fmt"
)

// Phase says which stage of the yearly cycle the league is in. The strings go
// out in the JSON API, so keep their spelling.
type Phase string

const (
	PhaseRegular      Phase = "regular"       // regular-season days left
	PhaseRegularDone  Phase = "regular_done"  // season over, playoffs not started
	PhasePlayoffs     Phase = "playoffs"      // playoffs in progress
	PhasePlayoffsDone Phase = "playoffs_done" // champion decided
	PhaseOffseason    Phase = "offseason"     // offseason report not yet seen
	PhaseDraft        Phase = "draft"         // draft in progress
	PhaseDraftDone    Phase = "draft_done"    // draft finished, free agency not begun
	PhaseFreeAgency   Phase = "free_agency"   // free-agency days (lg.FA != nil)
)

// Action is one thing the user may do next: a stable ID and a button label.
type Action struct{ ID, Label string }

// ErrUnknownPlayoffAction is returned by CanPlayoffAction for an action name
// that does not exist (callers treat it as a bad request, not a conflict).
var ErrUnknownPlayoffAction = errors.New("unknown playoff action")

// Phase returns the league's stage. reportSeen says whether the user has
// looked at the offseason report; until then a league with a draft set up is
// still in PhaseOffseason. (The engine does not store this flag; the caller does.)
func (lg *League) Phase(reportSeen bool) Phase {
	st := lg.Season
	switch {
	case lg.FA != nil:
		return PhaseFreeAgency
	case lg.Draft != nil && !reportSeen:
		return PhaseOffseason
	case lg.Draft != nil && lg.DraftDone():
		return PhaseDraftDone
	case lg.Draft != nil:
		return PhaseDraft
	case !st.PlayoffsStarted && !st.Done():
		return PhaseRegular
	case !st.PlayoffsStarted:
		return PhaseRegularDone
	case st.Champion >= 0:
		return PhasePlayoffsDone
	}
	return PhasePlayoffs
}

// NextActions returns the main "continue" action for the current phase and
// the extra menu actions. next is the zero Action and more is nil when there
// are none.
func (lg *League) NextActions(reportSeen bool) (next Action, more []Action) {
	switch lg.Phase(reportSeen) {
	case PhaseRegular:
		next = Action{"sim_day", "Sim next day"}
		more = []Action{{"watch_day", "Watch next game"}, {"sim_week", "Sim week"}, {"sim_to_end", "Sim to end of season"}}
	case PhaseRegularDone:
		next = Action{"start_playoffs", "Start playoffs"}
	case PhasePlayoffs:
		next = Action{"playoff_day", "Sim next playoff day"}
		if lg.UserSeriesIdx() >= 0 {
			more = append(more, Action{"playoff_watch", "Watch my next game"}, Action{"playoff_series", "Sim my series"})
		}
		more = append(more, Action{"playoff_round", "Sim round"}, Action{"playoff_all", "Sim all playoffs"})
	case PhasePlayoffsDone:
		next = Action{"begin_offseason", "Begin offseason"}
	case PhaseOffseason:
		next = Action{"go_draft", "Go to the draft"}
	case PhaseDraft:
		if lg.DraftOnClock() == lg.UserTeamID {
			next = Action{"make_pick", fmt.Sprintf("Make your pick (#%d)", lg.Draft.Made+1)}
		} else {
			next = Action{"draft_run", "Run the draft"}
		}
		more = []Action{{"draft_auto", "Auto-draft the rest"}}
	case PhaseDraftDone:
		next = Action{"begin_free_agency", "Start free agency"}
	case PhaseFreeAgency:
		if lg.FA.DaysLeft() > 0 {
			next = Action{"fa_day", "Next day of free agency"}
			more = []Action{{"start_season", "Skip to the new season"}}
		} else {
			next = Action{"start_season", fmt.Sprintf("Start the %d season", lg.Year+1)}
		}
	}
	return next, more
}

// ---- guards: each returns nil when the step is allowed right now ----

func (lg *League) CanSimDay(reportSeen bool) error {
	if lg.Phase(reportSeen) != PhaseRegular {
		return errors.New("no regular-season days left to sim")
	}
	return nil
}

func (lg *League) CanStartPlayoffs(reportSeen bool) error {
	if lg.Phase(reportSeen) != PhaseRegularDone {
		return errors.New("the regular season is not finished")
	}
	return nil
}

// CanPlayoffAction checks a playoff action name: "start", "day", "series",
// "round", "all" or "watch". Unknown names give ErrUnknownPlayoffAction.
func (lg *League) CanPlayoffAction(action string, reportSeen bool) error {
	if action == "start" {
		return lg.CanStartPlayoffs(reportSeen)
	}
	if lg.Phase(reportSeen) != PhasePlayoffs {
		return errors.New("no playoffs to sim")
	}
	switch action {
	case "day", "round", "all":
	case "series":
		if lg.UserSeriesIdx() < 0 {
			return errors.New("your team has no series in progress")
		}
	case "watch":
		if lg.UserSeriesIdx() < 0 {
			return errors.New("your team has no game to watch")
		}
	default:
		return ErrUnknownPlayoffAction
	}
	return nil
}

func (lg *League) CanBeginOffseason(reportSeen bool) error {
	if lg.Phase(reportSeen) != PhasePlayoffsDone {
		return errors.New("the playoffs are not finished")
	}
	return nil
}

// CanGoToDraft is allowed whenever an offseason (draft set up) is in progress.
func (lg *League) CanGoToDraft() error {
	if lg.Draft == nil {
		return errors.New("no offseason in progress")
	}
	return nil
}

func (lg *League) CanFinishOffseason(reportSeen bool) error {
	switch lg.Phase(reportSeen) {
	case PhaseFreeAgency:
		return nil
	case PhaseDraftDone:
		return errors.New("free agency has not begun yet")
	}
	return errors.New("the draft is not finished")
}

// CanAdvanceFreeAgency is allowed while free-agency days remain.
func (lg *League) CanAdvanceFreeAgency() error {
	if lg.FA == nil {
		return errors.New("free agency is not open")
	}
	if lg.FA.DaysLeft() == 0 {
		return errors.New("free agency is over; start the new season")
	}
	return nil
}

// CanDraft is allowed once the offseason report has been seen and a draft exists.
func (lg *League) CanDraft(reportSeen bool) error {
	if lg.Draft == nil || !reportSeen {
		return errors.New("no draft in progress")
	}
	return nil
}

// CanUserPick is allowed when the draft is running and the user is on the clock.
func (lg *League) CanUserPick() error {
	if lg.DraftDone() || lg.DraftOnClock() != lg.UserTeamID {
		return errors.New("you are not on the clock")
	}
	return nil
}

// ---- playoff seeding and home court ----

// ConferenceRank is the team's 1-based place in its conference standings, or 0.
func (s *Season) ConferenceRank(team int) int {
	for i, t := range s.ConferenceStandings(Conference(team)) {
		if t == team {
			return i + 1
		}
	}
	return 0
}

// PlayoffSeeds is how many teams per conference reach the playoffs.
const PlayoffSeeds = 8

// MadePlayoffs says whether the team's conference rank is a playoff seed.
func (s *Season) MadePlayoffs(team int) bool { return s.ConferenceRank(team) <= PlayoffSeeds }

// HostOf is the team (High or Low) hosting game index g (0-based) of the series.
func (sr *Series) HostOf(g int) int {
	if homePattern[g] == 'A' {
		return sr.Low
	}
	return sr.High
}

// UserPlayoffResult says how the user's playoff run went ("" before the
// playoffs start).
func (lg *League) UserPlayoffResult() string {
	st := lg.Season
	u := lg.UserTeamID
	if !st.PlayoffsStarted {
		return ""
	}
	best := 0
	lost := false
	for _, sr := range st.Series {
		if sr.High != u && sr.Low != u {
			continue
		}
		if sr.Round >= best {
			best = sr.Round
			lost = sr.Done && sr.Winner != u
		}
	}
	switch {
	case st.Champion == u:
		return "Champions"
	case best == 0:
		return "Missed the playoffs"
	case lost:
		return "Eliminated in " + RoundName(best)
	}
	return "Still alive in " + RoundName(best)
}
