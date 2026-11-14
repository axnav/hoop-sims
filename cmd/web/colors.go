package main

import (
	"fmt"
	"math"
	"strings"
)

// realColors holds each 2010-11 franchise's primary and secondary colour (values only).
var realColors = map[string][2]string{
	"DAL": {"#00538C", "#B8C4CA"}, "DEN": {"#4D90CD", "#FDB927"}, "GSW": {"#1D428A", "#FFC72C"},
	"HOU": {"#CE1141", "#C4CED4"}, "LAC": {"#C8102E", "#1D428A"}, "LAL": {"#552583", "#FDB927"},
	"MEM": {"#5D76A9", "#12173F"}, "MIN": {"#236192", "#78BE20"}, "NOH": {"#00778B", "#FFB81C"},
	"OKC": {"#007AC1", "#EF3B24"}, "PHO": {"#E56020", "#1D1160"}, "POR": {"#E03A3E", "#9A9EA1"},
	"SAC": {"#5A2D81", "#63727A"}, "SAS": {"#8D9093", "#000000"}, "UTA": {"#2A5FA8", "#F9A01B"},
	"ATL": {"#E03A3E", "#C1D32F"}, "BOS": {"#007A33", "#BA9653"}, "CHA": {"#F26B1D", "#1D5FA4"},
	"CHI": {"#CE1141", "#9A9EA1"}, "CLE": {"#860038", "#FDBB30"}, "DET": {"#C8102E", "#1D42BA"},
	"IND": {"#2B5FA8", "#FDBB30"}, "MIA": {"#98002E", "#F9A01B"}, "MIL": {"#00A04A", "#EEE1C6"},
	"NJN": {"#2A5FA8", "#CD1041"}, "NYK": {"#006BB6", "#F58426"}, "ORL": {"#0077C0", "#C4CED4"},
	"PHI": {"#006BB6", "#ED174C"}, "TOR": {"#753BBD", "#BEC0C2"}, "WAS": {"#2A5FA8", "#E31837"},
}

func hslHex(h, s, l float64) string {
	h = math.Mod(h, 360)
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return fmt.Sprintf("#%02X%02X%02X", int((r+m)*255+.5), int((g+m)*255+.5), int((b+m)*255+.5))
}

// fictionalColors derives stable, well-spread colours from the team ID (golden-angle hues).
func fictionalColors(id int) [2]string {
	h := math.Mod(float64(id)*137.508+20, 360)
	return [2]string{hslHex(h, 0.62, 0.42), hslHex(h+35, 0.75, 0.72)}
}

func fictionalAbbr(city string) string {
	s := strings.ToUpper(strings.ReplaceAll(city, " ", ""))
	if len(s) > 3 {
		s = s[:3]
	}
	return s
}
