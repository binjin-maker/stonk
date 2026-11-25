package main

import (
	"fmt"
	"math"
	"strings"
)

type Pattern struct {
	Name string
	Code string
	Dir  int
	W    float64
}

func candleAt(o, h, l, c []float64, i int) []Pattern {
	if i < 3 || i >= len(c) {
		return nil
	}
	body := func(k int) float64 { return math.Abs(c[k] - o[k]) }
	rng := func(k int) float64 { return math.Max(h[k]-l[k], 1e-9) }
	up := func(k int) bool { return c[k] > o[k] }
	upper := func(k int) float64 { return h[k] - math.Max(o[k], c[k]) }
	lower := func(k int) float64 { return math.Min(o[k], c[k]) - l[k] }
	avgBody := 0.0
	for k := max(0, i-10); k < i; k++ {
		avgBody += body(k)
	}
	avgBody /= float64(min(10, i))
	trend := (c[i-1] - c[max(0, i-6)]) / c[max(0, i-6)]
	var out []Pattern
	b, r := body(i), rng(i)
	switch {
	case b <= 0.1*r:
		out = append(out, Pattern{"Doji (indecision)", "D", 0, 0.3})
	case lower(i) >= 2*b && upper(i) <= 0.35*b+0.05*r && trend < -0.01:
		out = append(out, Pattern{"Hammer (bullish reversal)", "H", 1, 1})
	case upper(i) >= 2*b && lower(i) <= 0.35*b+0.05*r && trend > 0.01:
		out = append(out, Pattern{"Shooting star (bearish reversal)", "SS", -1, 1})
	case lower(i) >= 2*b && upper(i) <= 0.35*b+0.05*r && trend > 0.01:
		out = append(out, Pattern{"Hanging man (bearish warning)", "HM", -1, 0.6})
	case upper(i) >= 2*b && lower(i) <= 0.35*b+0.05*r && trend < -0.01:
		out = append(out, Pattern{"Inverted hammer (bullish warning)", "IH", 1, 0.6})
	}
	if !up(i-1) && up(i) && o[i] <= c[i-1] && c[i] >= o[i-1] && b > body(i-1) {
		out = append(out, Pattern{"Bullish engulfing", "BE", 1, 1.2})
	}
	if up(i-1) && !up(i) && o[i] >= c[i-1] && c[i] <= o[i-1] && b > body(i-1) {
		out = append(out, Pattern{"Bearish engulfing", "bE", -1, 1.2})
	}
	if !up(i-2) && body(i-2) > avgBody && body(i-1) < 0.5*avgBody && up(i) && c[i] > (o[i-2]+c[i-2])/2 {
		out = append(out, Pattern{"Morning star (bullish reversal)", "MS", 1, 1.4})
	}
	if up(i-2) && body(i-2) > avgBody && body(i-1) < 0.5*avgBody && !up(i) && c[i] < (o[i-2]+c[i-2])/2 {
		out = append(out, Pattern{"Evening star (bearish reversal)", "ES", -1, 1.4})
	}
	if up(i) && up(i-1) && up(i-2) && c[i] > c[i-1] && c[i-1] > c[i-2] && b > 0.6*avgBody && body(i-1) > 0.6*avgBody && body(i-2) > 0.6*avgBody {
		out = append(out, Pattern{"Three white soldiers (strong bullish)", "3W", 1, 1.3})
	}
	if !up(i) && !up(i-1) && !up(i-2) && c[i] < c[i-1] && c[i-1] < c[i-2] && b > 0.6*avgBody && body(i-1) > 0.6*avgBody && body(i-2) > 0.6*avgBody {
		out = append(out, Pattern{"Three black crows (strong bearish)", "3B", -1, 1.3})
	}
	if b > 2.2*avgBody && avgBody > 0 {
		if up(i) {
			out = append(out, Pattern{"Big bullish candle", "BB", 1, 0.5})
		} else {
			out = append(out, Pattern{"Big bearish candle", "bB", -1, 0.5})
		}
	}
	return out
}

func candleTheory(q *Quote) Theory {
	n := len(q.Closes)
	if len(q.Opens) != n || n < 12 {
		return th("candles", 0, "Not enough candle data.")
	}
	score := 0.0
	var seen []string
	for k := 0; k < 4; k++ {
		i := n - 1 - k
		decay := []float64{1, 0.7, 0.45, 0.3}[k]
		for _, p := range candleAt(q.Opens, q.Highs, q.Lows, q.Closes, i) {
			score += float64(p.Dir) * p.W * decay
			ago := map[int]string{0: "today", 1: "yesterday"}[k]
			if ago == "" {
				ago = fmt.Sprintf("%d sessions ago", k)
			}
			seen = append(seen, p.Name+" "+ago)
		}
	}
	if len(seen) == 0 {
		return th("candles", 0, "No notable candlestick patterns in the last 4 sessions.")
	}
	return th("candles", score, "%s.", strings.Join(seen, "; "))
}
