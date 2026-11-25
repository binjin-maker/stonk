package main

import (
	"fmt"
	"math"
)

type TheoryInfo struct {
	ID      string
	Name    string
	Explain string
}

type Theory struct {
	ID      string
	Verdict string
	Score   float64
	Detail  string
}

var theoryCatalog = []TheoryInfo{
	{"news", "News Catalyst", "This app's own event model: scores every news event on the company and the world events hitting its sector, weighted by source credibility, freshness and whether it's temporary, well-known or permanent."},
	{"box", "Box Theory (Darvas)", "Nicolas Darvas's method: stocks trade inside 'boxes' (a ceiling and a floor). A close above the box top is a breakout and a buy; a close below the floor is a sell. Inside the box, wait."},
	{"dow", "Dow Theory", "Charles Dow: a trend is up while price keeps making higher highs and higher lows, and down while making lower highs and lower lows. Trends persist until clearly reversed."},
	{"ma", "Golden / Death Cross", "Moving-average trend theory: 50-day above 200-day is a 'golden cross' (bull market), below is a 'death cross' (bear market). Price above both averages confirms strength."},
	{"rsi", "RSI (Overbought / Oversold)", "Wilder's Relative Strength Index (14-day). Above 70 means overbought and due for a pullback; below 30 means oversold and due for a bounce. 50–70 is healthy bullish momentum."},
	{"macd", "MACD Momentum", "Gerald Appel's MACD (12/26/9). When the MACD line is above its signal line and rising, momentum is bullish; crossing below is bearish."},
	{"boll", "Bollinger Bands", "John Bollinger: price tends to stay within 2 standard deviations of its 20-day average. Tagging the upper band = stretched; lower band = washed out. A 'squeeze' (narrow bands) often comes before a big move."},
	{"meanrev", "Mean Reversion", "Prices that move far from their average tend to snap back toward it. Measures how many standard deviations price is from its 20-day mean."},
	{"mom", "Momentum (Jegadeesh-Titman)", "Academic momentum effect: stocks that went up over the past 6 months (skipping the latest month) tend to keep outperforming for months, and losers keep losing."},
	{"fib", "Fibonacci Retracement", "After a big move, pullbacks often stall at 38.2%, 50% or 61.8% of the move. A pullback holding in that zone signals the trend resumes; breaking 78.6% signals reversal."},
	{"obv", "Volume / OBV (Granville)", "Joe Granville's On-Balance Volume: volume leads price. Rising OBV while price is flat = smart money accumulating; falling OBV while price rises = distribution."},
	{"contra", "Contrarian (Buffett)", "\"Be fearful when others are greedy and greedy when others are fearful.\" Heavy bad news plus a big drop (without a fatal problem) is a buying opportunity; euphoria plus a big run-up is a warning."},
	{"emh", "Efficient Market Hypothesis", "Eugene Fama: all public information is already in the price, so nobody can reliably predict moves from news or charts. Expected move ≈ the market's normal drift. The skeptic's baseline."},
}

func verdict(s float64) string {
	switch {
	case s >= 1:
		return "UP"
	case s >= 0.4:
		return "LEAN UP"
	case s <= -1:
		return "DOWN"
	case s <= -0.4:
		return "LEAN DOWN"
	}
	return "NEUTRAL"
}

func th(id string, score float64, f string, a ...any) Theory {
	score = clamp(score, -2, 2)
	return Theory{ID: id, Score: math.Round(score*100) / 100, Verdict: verdict(score), Detail: fmt.Sprintf(f, a...)}
}

func sma(x []float64, n int) float64 {
	if len(x) < n || n <= 0 {
		return math.NaN()
	}
	s := 0.0
	for _, v := range x[len(x)-n:] {
		s += v
	}
	return s / float64(n)
}

func stdev(x []float64, n int) float64 {
	if len(x) < n {
		return math.NaN()
	}
	m := sma(x, n)
	v := 0.0
	for _, y := range x[len(x)-n:] {
		v += (y - m) * (y - m)
	}
	return math.Sqrt(v / float64(n))
}

func emaSeries(x []float64, n int) []float64 {
	out := make([]float64, len(x))
	k := 2 / float64(n+1)
	for i, v := range x {
		if i == 0 {
			out[i] = v
		} else {
			out[i] = v*k + out[i-1]*(1-k)
		}
	}
	return out
}

func maxf(x []float64) float64 {
	m := math.Inf(-1)
	for _, v := range x {
		m = math.Max(m, v)
	}
	return m
}
func minf(x []float64) float64 {
	m := math.Inf(1)
	for _, v := range x {
		m = math.Min(m, v)
	}
	return m
}

func computeTheories(q *Quote, s *Signal) []Theory {
	var out []Theory
	out = append(out, th("news", s.Expected/2.5, "%s, expecting %+.1f%% (%d%% confidence).", s.Outlook, s.Expected, s.Confidence))
	if q == nil || !q.OK || len(q.Closes) < 30 {
		out = append(out, th("emh", 0, "Not enough price history to run chart theories. EMH says: assume the market's normal drift."))
		return out
	}
	c, h, l, v := q.Closes, q.Highs, q.Lows, q.Volumes
	n := len(c)
	px := c[n-1]

	if n >= 26 {
		top := maxf(h[n-23 : n-3])
		bot := minf(l[n-23 : n-3])
		height := (top - bot) / bot * 100
		volUp := false
		if sma(v, 20) > 0 && v[n-1] > 1.3*sma(v, 20) {
			volUp = true
		}
		switch {
		case px > top:
			sc := 1.5
			ex := ""
			if volUp {
				sc, ex = 2, " on heavy volume (strong confirmation)"
			}
			out = append(out, th("box", sc, "Broke OUT above the box top $%.2f (box $%.2f–$%.2f)%s. Darvas would buy here with a stop just under $%.2f.", top, bot, top, ex, top))
		case px < bot:
			out = append(out, th("box", -1.7, "Broke DOWN below the box floor $%.2f (box $%.2f–$%.2f). Darvas would sell / stay out.", bot, bot, top))
		default:
			p := (px - bot) / math.Max(top-bot, 1e-9) * 100
			tight := ""
			if height < 4 {
				tight = " Very tight box: a breakout is often close."
			}
			out = append(out, th("box", (p-50)/150, "Inside the box $%.2f–$%.2f (%.0f%% of the way up, box height %.1f%%). No signal until it breaks out.%s", bot, top, p, height, tight))
		}
	}

	{
		start := max(0, n-160)
		var hs, ls []float64
		w := 5
		for i := start + w; i < n-w; i++ {
			isH, isL := true, true
			for j := i - w; j <= i+w; j++ {
				if c[j] > c[i] {
					isH = false
				}
				if c[j] < c[i] {
					isL = false
				}
			}
			if isH {
				hs = append(hs, c[i])
			}
			if isL {
				ls = append(ls, c[i])
			}
		}
		if len(hs) >= 2 && len(ls) >= 2 {
			hh := hs[len(hs)-1] > hs[len(hs)-2]
			hl := ls[len(ls)-1] > ls[len(ls)-2]
			switch {
			case hh && hl:
				out = append(out, th("dow", 1.4, "Higher highs ($%.2f > $%.2f) and higher lows ($%.2f > $%.2f): primary UPTREND intact.", hs[len(hs)-1], hs[len(hs)-2], ls[len(ls)-1], ls[len(ls)-2]))
			case !hh && !hl:
				out = append(out, th("dow", -1.4, "Lower highs ($%.2f < $%.2f) and lower lows ($%.2f < $%.2f): primary DOWNTREND.", hs[len(hs)-1], hs[len(hs)-2], ls[len(ls)-1], ls[len(ls)-2]))
			case hh:
				out = append(out, th("dow", 0.3, "Higher high but a lower low: trend is unconfirmed / transitioning."))
			default:
				out = append(out, th("dow", -0.3, "Higher low but a lower high: trend is unconfirmed / transitioning."))
			}
		} else {
			out = append(out, th("dow", 0, "Not enough clear swing points to call a Dow trend."))
		}
	}

	{
		fN, sN, lbl := 50, 200, "50/200-day"
		if n < 205 {
			fN, sN, lbl = 20, 50, "20/50-day (short history)"
		}
		if n > sN+10 {
			f, sl := sma(c, fN), sma(c, sN)
			fPrev, sPrev := sma(c[:n-10], fN), sma(c[:n-10], sN)
			cross := ""
			if f > sl && fPrev <= sPrev {
				cross = " A fresh GOLDEN CROSS happened in the last 2 weeks."
			} else if f < sl && fPrev >= sPrev {
				cross = " A fresh DEATH CROSS happened in the last 2 weeks."
			}
			sc := 0.0
			if f > sl {
				sc += 1
			} else {
				sc -= 1
			}
			if px > f && px > sl {
				sc += 0.5
			} else if px < f && px < sl {
				sc -= 0.5
			}
			state := "golden cross (bullish)"
			if f < sl {
				state = "death cross (bearish)"
			}
			out = append(out, th("ma", sc, "%s averages: $%.2f vs $%.2f = %s. Price $%.2f.%s", lbl, f, sl, state, px, cross))
		}
	}

	if n > 15 {
		var g, ls float64
		for i := 1; i <= 14; i++ {
			d := c[i] - c[i-1]
			if d > 0 {
				g += d
			} else {
				ls -= d
			}
		}
		g, ls = g/14, ls/14
		for i := 15; i < n; i++ {
			d := c[i] - c[i-1]
			gg, ll := math.Max(d, 0), math.Max(-d, 0)
			g = (g*13 + gg) / 14
			ls = (ls*13 + ll) / 14
		}
		rsi := 100.0
		if ls > 0 {
			rsi = 100 - 100/(1+g/ls)
		}
		var sc float64
		var d string
		switch {
		case rsi >= 70:
			sc, d = -1.3-(rsi-70)/30, "OVERBOUGHT: pullback risk is high."
		case rsi <= 30:
			sc, d = 1.3+(30-rsi)/30, "OVERSOLD: bounce likely."
		case rsi >= 50:
			sc, d = 0.5, "Healthy bullish momentum."
		default:
			sc, d = -0.5, "Weak momentum."
		}
		out = append(out, th("rsi", sc, "RSI(14) = %.0f. %s", rsi, d))
	}

	if n > 35 {
		e12, e26 := emaSeries(c, 12), emaSeries(c, 26)
		macd := make([]float64, n)
		for i := range c {
			macd[i] = e12[i] - e26[i]
		}
		sig := emaSeries(macd, 9)
		hist := macd[n-1] - sig[n-1]
		histPrev := macd[n-4] - sig[n-4]
		sc := 0.0
		d := ""
		if hist > 0 {
			sc = 1
			d = "MACD above signal (bullish)"
		} else {
			sc = -1
			d = "MACD below signal (bearish)"
		}
		if hist > histPrev {
			sc += 0.4
			d += ", momentum strengthening"
		} else {
			sc -= 0.4
			d += ", momentum fading"
		}
		if (hist > 0) != (histPrev > 0) {
			d += ". Fresh crossover in the last few days"
		}
		out = append(out, th("macd", sc, "%s (hist %.3f).", d, hist))
	}

	if n > 60 {
		m, sd := sma(c, 20), stdev(c, 20)
		up, lo := m+2*sd, m-2*sd
		pb := (px - lo) / math.Max(up-lo, 1e-9)
		bw := (up - lo) / m
		var bws []float64
		for i := n - 120; i < n; i++ {
			if i >= 20 {
				mm, ss := sma(c[:i], 20), stdev(c[:i], 20)
				bws = append(bws, 4*ss/mm)
			}
		}
		squeeze := false
		if len(bws) > 20 {
			cnt := 0
			for _, b := range bws {
				if b < bw {
					cnt++
				}
			}
			squeeze = float64(cnt)/float64(len(bws)) < 0.15
		}
		switch {
		case pb > 1:
			out = append(out, th("boll", -1.2, "Closed ABOVE the upper band ($%.2f). Stretched; expect a pullback toward $%.2f.", up, m))
		case pb < 0:
			out = append(out, th("boll", 1.2, "Closed BELOW the lower band ($%.2f). Washed out; expect a bounce toward $%.2f.", lo, m))
		case squeeze:
			out = append(out, th("boll", 0, "Bollinger SQUEEZE: bands are unusually narrow ($%.2f–$%.2f). A big move is coming, but the bands don't say which way.", lo, up))
		default:
			out = append(out, th("boll", (0.5-pb)*0.8, "Inside the bands ($%.2f–$%.2f), %%B = %.2f.", lo, up, pb))
		}
	}

	if n > 25 {
		m, sd := sma(c, 20), stdev(c, 20)
		if sd > 0 {
			z := (px - m) / sd
			d := "close to its average; no pull either way"
			if z > 1.5 {
				d = "well above average; snap-back risk"
			} else if z < -1.5 {
				d = "well below average; snap-back upside"
			}
			out = append(out, th("meanrev", -z/2, "%.1f standard deviations from the 20-day mean ($%.2f): %s.", z, m, d))
		}
	}

	if n > 130 {
		r := (c[n-22]/c[n-127] - 1) * 100
		out = append(out, th("mom", r/10, "6-month return (skipping last month): %+.1f%%. %s", r, map[bool]string{true: "Winners tend to keep winning.", false: "Losers tend to keep losing."}[r >= 0]))
	} else if n > 60 {
		r := (c[n-22]/c[0] - 1) * 100
		out = append(out, th("mom", r/10, "Return over available history (skipping last month): %+.1f%%.", r))
	}

	{
		hiI, loI := 0, 0
		for i := range c {
			if c[i] > c[hiI] {
				hiI = i
			}
			if c[i] < c[loI] {
				loI = i
			}
		}
		hi, lo := c[hiI], c[loI]
		rng := hi - lo
		if rng > 0 {
			if loI < hiI {
				ret := (hi - px) / rng
				switch {
				case ret < 0.236:
					out = append(out, th("fib", 0.5, "Uptrend from $%.2f to $%.2f; price is near the highs (%.0f%% retraced). Trend intact.", lo, hi, ret*100))
				case ret < 0.382:
					out = append(out, th("fib", 0.6, "Shallow %.0f%% pullback of the $%.2f→$%.2f rally, above the 38.2%% level. Strong trend.", ret*100, lo, hi))
				case ret <= 0.65:
					out = append(out, th("fib", 1.1, "Pulled back %.0f%% of the $%.2f→$%.2f rally, into the 38.2–61.8%% Fibonacci zone. Classic spot for the uptrend to resume.", ret*100, lo, hi))
				case ret <= 0.786:
					out = append(out, th("fib", -0.3, "Retraced %.0f%%: past the golden ratio. Uptrend is on thin ice.", ret*100))
				default:
					out = append(out, th("fib", -1.3, "Retraced %.0f%% of the rally: beyond 78.6%%, the uptrend has likely failed.", ret*100))
				}
			} else {
				ret := (px - lo) / rng
				switch {
				case ret < 0.236:
					out = append(out, th("fib", -0.5, "Downtrend from $%.2f to $%.2f; price still near the lows (%.0f%% recovered).", hi, lo, ret*100))
				case ret < 0.382:
					out = append(out, th("fib", -0.6, "Weak %.0f%% bounce off the $%.2f low, still below the 38.2%% level of the decline from $%.2f. Downtrend in control.", ret*100, lo, hi))
				case ret <= 0.65:
					out = append(out, th("fib", -1, "Bounced %.0f%% of the $%.2f→$%.2f decline, into the 38.2–61.8%% zone. Classic resistance where downtrends resume.", ret*100, hi, lo))
				case ret <= 0.786:
					out = append(out, th("fib", 0.3, "Recovered %.0f%% of the decline: pushing through resistance.", ret*100))
				default:
					out = append(out, th("fib", 1.3, "Recovered %.0f%% of the decline: beyond 78.6%%, the downtrend has likely ended.", ret*100))
				}
			}
		}
	}

	if n > 25 && sma(v, 20) > 0 {
		obv := make([]float64, n)
		for i := 1; i < n; i++ {
			obv[i] = obv[i-1]
			if c[i] > c[i-1] {
				obv[i] += v[i]
			} else if c[i] < c[i-1] {
				obv[i] -= v[i]
			}
		}
		avgV := sma(v, 20)
		dObv := (obv[n-1] - obv[n-21]) / (avgV * 20)
		dPx := (px/c[n-21] - 1) * 100
		switch {
		case dObv > 0.15 && dPx < 2:
			out = append(out, th("obv", 1.3, "Volume is flowing IN (OBV up) while price is flat/down (%+.1f%%): accumulation. Granville says price follows.", dPx))
		case dObv < -0.15 && dPx > -2:
			out = append(out, th("obv", -1.3, "Volume is flowing OUT (OBV down) while price holds (%+.1f%%): distribution, smart money exiting.", dPx))
		case dObv > 0.15:
			out = append(out, th("obv", 0.8, "OBV rising with price: volume confirms the up move."))
		case dObv < -0.15:
			out = append(out, th("obv", -0.8, "OBV falling with price: volume confirms the down move."))
		default:
			out = append(out, th("obv", 0, "Volume flow is balanced. No accumulation or distribution signal."))
		}
	}

	{
		fear := s.NewsMove < -2.5 && s.R20 < -8
		greed := s.NewsMove > 2.5 && s.R20 > 12
		switch {
		case fear && s.Flight >= 3:
			out = append(out, th("contra", -0.5, "Lots of fear, but the news is potentially fatal (flight risk severe). Contrarians avoid catching this knife."))
		case fear:
			out = append(out, th("contra", 1.4, "Others are fearful (bad news, %+.1f%% this month) but the damage doesn't look fatal. Contrarian BUY zone.", s.R20))
		case greed:
			out = append(out, th("contra", -1.2, "Others are greedy (hype, %+.1f%% this month). Contrarians take profits here.", s.R20))
		case s.R20 < -15:
			out = append(out, th("contra", 0.8, "Down %.1f%% in a month without matching bad news: possibly oversold on sentiment.", -s.R20))
		case s.R20 > 20:
			out = append(out, th("contra", -0.8, "Up %.1f%% in a month: crowd is getting excited.", s.R20))
		default:
			out = append(out, th("contra", 0, "Sentiment isn't extreme either way. No contrarian edge."))
		}
	}

	out = append(out, th("emh", 0.1, "All public info, including the %d articles found, is assumed already in the $%.2f price. Expected move ≈ normal market drift (~+0.2%% over 2 weeks), with ±%.1f%% random noise.", s.NArticles, px, s.Vol*math.Sqrt(10)))
	return out
}
