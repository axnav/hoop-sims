package main

import (
	"fmt"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/tabwriter"

	"hoopsim/engine"
)

// runRealRatings is the measuring tool for real-data ratings: one season
// (team roster plus league top 40, then the summary line) or "all" (summary
// lines for 2011-2025 and their averages).
func runRealRatings(seed uint64, season, team string, ref2k bool) {
	if ref2k {
		runRef2K(seed, season)
		return
	}
	const dir = "data/real"
	if season == "all" {
		var n, t15, t30, ref, u20, corr float64
		worst, worstS := 99, 0
		for y := 2011; y <= 2025; y++ {
			rep, err := engine.RealRatingsReport(dir, seed, y)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Println(rep.Summary())
			n++
			t15 += float64(rep.RefTop15)
			t30 += float64(rep.RefTop30)
			ref += float64(rep.Ref)
			u20 += float64(rep.Under20In40)
			corr += rep.CorrVORP
			if rep.RefTop30 < worst {
				worst, worstS = rep.RefTop30, y
			}
		}
		fmt.Printf("average  ref-in-top15 %.1f/%.0f  ref-in-top30 %.1f/%.0f  under20mpg-in-top40 %.2f  corr(ovr,vorp) %.3f  worst top30 %d (%d)\n",
			t15/n, ref/n, t30/n, ref/n, u20/n, corr/n, worst, worstS)
		return
	}
	y, err := strconv.Atoi(season)
	if err != nil {
		fmt.Fprintln(os.Stderr, "season must be a year or all")
		os.Exit(1)
	}
	rep, err := engine.RealRatingsReport(dir, seed, y)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if team != "" {
		fmt.Fprintln(w, "Name\tAge\tPos\tmpg\tVORP\tOvr\tPot\tIns\tMid\tThr\tPlay\tReb\tDef\tAth")
		for _, r := range rep.Rows {
			if !strings.Contains(strings.ToLower(r.Team), strings.ToLower(team)) {
				continue
			}
			fmt.Fprintf(w, "%s\t%d\t%s\t%.1f\t%.1f\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
				r.Name, r.Age, r.Pos, r.MPG, r.VORP, r.Ovr, r.Pot, r.Rat[0], r.Rat[1], r.Rat[2], r.Rat[3], r.Rat[4], r.Rat[5], r.Rat[6])
		}
		w.Flush()
		fmt.Println()
	}
	fmt.Fprintln(w, "#\tName\tTeam\tmpg\tVORP\tOvr\t")
	for i, r := range rep.Rows {
		if i >= 40 {
			break
		}
		star := ""
		if r.Top {
			star = "*"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%.1f\t%.1f\t%d\t%s\n", i+1, r.Name, r.Team, r.MPG, r.VORP, r.Ovr, star)
	}
	w.Flush()
	fmt.Println(rep.Summary())
	fmt.Printf("league mean %.1f  players 90+ %d  (%d listed)\n", rep.Mean, rep.N90, len(rep.Rows))
}

// applyTune sets RealTuning fields from "Name=value,Name=value" (float or bool
// fields), so candidate settings can be compared without editing code.
func applyTune(spec string) error {
	v := reflect.ValueOf(&engine.RealTuning).Elem()
	for _, kv := range strings.Split(spec, ",") {
		k, val, ok := strings.Cut(kv, "=")
		f := v.FieldByName(strings.TrimSpace(k))
		if !ok || !f.IsValid() {
			return fmt.Errorf("bad tuning %q", kv)
		}
		switch f.Kind() {
		case reflect.Float64:
			x, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return err
			}
			f.SetFloat(x)
		case reflect.Bool:
			x, err := strconv.ParseBool(val)
			if err != nil {
				return err
			}
			f.SetBool(x)
		default:
			return fmt.Errorf("field %s is not a number or bool", k)
		}
	}
	return nil
}

// runRef2K compares one season (or the four reference seasons) with ref2k.csv.
func runRef2K(seed uint64, season string) {
	const dir = "data/real"
	years := []int{2014, 2018, 2022, 2025}
	detail := true
	if season != "all" {
		y, err := strconv.Atoi(season)
		if err != nil {
			fmt.Fprintln(os.Stderr, "season must be a year or all")
			os.Exit(1)
		}
		years = []int{y}
	} else {
		detail = false
	}
	for _, y := range years {
		rep, err := engine.RealRatingsReport(dir, seed, y)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		res, err := engine.Ref2KReport(dir, rep)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		worst := ""
		for i := 0; i < 3 && i < len(res.Rows); i++ {
			r := res.Rows[i]
			worst += fmt.Sprintf(" %s %+.1f;", r.Name, r.Gap)
		}
		fmt.Printf("2k %d  n %d  fit 2k=%.2f+%.3f*ours  spearman %.3f  mae %.2f  top10-out-of-top20 %d (worst rank %d)  worst:%s\n",
			y, res.N, res.Icept, res.Slope, res.Spearman, res.MAE, res.TopTenOut, res.TopTenWorstRank, worst)
		if detail {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "Name\tours\tmapped\t2K\tgap\trank\tmpg\tG\tid\tour-scale value of 2K")
			for i, r := range res.Rows {
				if i >= 12 && math.Abs(r.Gap) <= 5 {
					break
				}
				fmt.Fprintf(w, "%s\t%d\t%.1f\t%d\t%+.1f\t%d\t%.1f\t%d\t%s\t%.0f\n", r.Name, r.Ours, r.Mapped, r.K2, r.Gap, r.Rank, r.MPG, r.G, r.ID, (float64(r.K2)-res.Icept)/res.Slope)
			}
			w.Flush()
		}
	}
}
