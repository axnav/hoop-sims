package engine

import "testing"

func actionIDs(lg *League, seen bool) []string {
	n, more := lg.NextActions(seen)
	ids := []string{n.ID}
	for _, a := range more {
		ids = append(ids, a.ID)
	}
	return ids
}

func wantPhase(t *testing.T, lg *League, seen bool, p Phase, ids ...string) {
	t.Helper()
	if got := lg.Phase(seen); got != p {
		t.Fatalf("phase = %q, want %q", got, p)
	}
	got := actionIDs(lg, seen)
	if len(got) != len(ids) {
		t.Fatalf("phase %q actions = %v, want %v", p, got, ids)
	}
	for i := range ids {
		if got[i] != ids[i] {
			t.Fatalf("phase %q actions = %v, want %v", p, got, ids)
		}
	}
}

func wantErr(t *testing.T, name string, err error, bad bool) {
	t.Helper()
	if bad && err == nil {
		t.Fatalf("%s: expected an error", name)
	}
	if !bad && err != nil {
		t.Fatalf("%s: unexpected error %v", name, err)
	}
}

func TestPhaseCycle(t *testing.T) {
	lg := GenerateLeague(21)
	lg.UserTeamID = 2
	lg.StartSeason()

	wantPhase(t, lg, true, PhaseRegular, "sim_day", "watch_day", "sim_week", "sim_to_end")
	wantErr(t, "CanSimDay", lg.CanSimDay(true), false)
	wantErr(t, "CanStartPlayoffs early", lg.CanStartPlayoffs(true), true)
	wantErr(t, "playoff day early", lg.CanPlayoffAction("day", true), true)
	wantErr(t, "BeginOffseason early", lg.CanBeginOffseason(true), true)
	wantErr(t, "GoToDraft early", lg.CanGoToDraft(), true)
	wantErr(t, "Draft early", lg.CanDraft(true), true)

	lg.SimRegularSeason()
	wantPhase(t, lg, true, PhaseRegularDone, "start_playoffs")
	wantErr(t, "CanSimDay done", lg.CanSimDay(true), true)
	wantErr(t, "CanStartPlayoffs", lg.CanStartPlayoffs(true), false)
	wantErr(t, "start action", lg.CanPlayoffAction("start", true), false)
	if !lg.Season.MadePlayoffs(lg.UserTeamID) != (lg.Season.ConferenceRank(lg.UserTeamID) > PlayoffSeeds) {
		t.Fatal("MadePlayoffs disagrees with ConferenceRank")
	}

	if err := lg.StartPlayoffs(); err != nil {
		t.Fatal(err)
	}
	if lg.UserSeriesIdx() >= 0 {
		wantPhase(t, lg, true, PhasePlayoffs, "playoff_day", "playoff_watch", "playoff_series", "playoff_round", "playoff_all")
		wantErr(t, "series", lg.CanPlayoffAction("series", true), false)
		wantErr(t, "watch", lg.CanPlayoffAction("watch", true), false)
	} else {
		wantPhase(t, lg, true, PhasePlayoffs, "playoff_day", "playoff_round", "playoff_all")
		wantErr(t, "series", lg.CanPlayoffAction("series", true), true)
	}
	wantErr(t, "day", lg.CanPlayoffAction("day", true), false)
	wantErr(t, "start twice", lg.CanPlayoffAction("start", true), true)
	if err := lg.CanPlayoffAction("bogus", true); err != ErrUnknownPlayoffAction {
		t.Fatalf("bogus action: got %v", err)
	}

	lg.SimAllPlayoffs()
	wantPhase(t, lg, true, PhasePlayoffsDone, "begin_offseason")
	wantErr(t, "playoff day after end", lg.CanPlayoffAction("day", true), true)
	if lg.UserPlayoffResult() == "" {
		t.Fatal("expected a playoff result")
	}
	wantErr(t, "BeginOffseason", lg.CanBeginOffseason(true), false)

	lg.BeginOffseason()
	wantPhase(t, lg, false, PhaseOffseason, "go_draft")
	wantErr(t, "BeginOffseason twice", lg.CanBeginOffseason(false), true)
	wantErr(t, "GoToDraft", lg.CanGoToDraft(), false)
	wantErr(t, "Draft before report", lg.CanDraft(false), true)
	wantErr(t, "FinishOffseason early", lg.CanFinishOffseason(false), true)

	if lg.DraftOnClock() == lg.UserTeamID {
		wantPhase(t, lg, true, PhaseDraft, "make_pick", "draft_auto")
		wantErr(t, "UserPick", lg.CanUserPick(), false)
	} else {
		wantPhase(t, lg, true, PhaseDraft, "draft_run", "draft_auto")
		wantErr(t, "UserPick", lg.CanUserPick(), true)
	}
	wantErr(t, "Draft", lg.CanDraft(true), false)
	wantErr(t, "FinishOffseason mid-draft", lg.CanFinishOffseason(true), true)

	lg.DraftAutoAll()
	wantPhase(t, lg, true, PhaseDraftDone, "begin_free_agency")
	wantErr(t, "UserPick after draft", lg.CanUserPick(), true)
	wantErr(t, "FinishOffseason before free agency", lg.CanFinishOffseason(true), true)
	wantErr(t, "BeginFreeAgency", lg.CanBeginFreeAgency(true), false)
	if err := lg.BeginFreeAgency(); err != nil {
		t.Fatal(err)
	}
	wantPhase(t, lg, true, PhaseFreeAgency, "fa_day", "start_season")
	wantErr(t, "FinishOffseason", lg.CanFinishOffseason(true), false)

	lg.FinishOffseason()
	lg.StartSeason()
	wantPhase(t, lg, true, PhaseRegular, "sim_day", "watch_day", "sim_week", "sim_to_end")
}
