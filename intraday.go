package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Series struct {
	Opens, Highs, Lows, Closes, Volumes []float64
	Times                               []int64
	Interval                            string
	Listed                              int64
}

var (
	intraMu    sync.Mutex
	intraCache = map[string]struct {
		s  *Series
		at time.Time
	}{}
)

func fetchSeries(sym, rng, interval string) (*Series, error) {
	key := sym + "|" + rng + "|" + interval
	intraMu.Lock()
	if c, ok := intraCache[key]; ok && time.Since(c.at) < 60*time.Second {
		intraMu.Unlock()
		return c.s, nil
	}
	intraMu.Unlock()
	b, err := httpGet("https://query1.finance.yahoo.com/v8/finance/chart/" + url.PathEscape(sym) + "?range=" + rng + "&interval=" + interval + "&includePrePost=false")
	if err != nil {
		return nil, err
	}
	var r struct {
		Chart struct {
			Result []struct {
				Meta       struct{ FirstTradeDate int64 }
				Timestamp  []int64
				Indicators struct {
					Quote []struct{ Open, High, Low, Close, Volume []*float64 }
				}
			}
		}
	}
	if err := json.Unmarshal(b, &r); err != nil || len(r.Chart.Result) == 0 || len(r.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, err
	}
	res := r.Chart.Result[0]
	q := res.Indicators.Quote[0]
	s := &Series{Interval: interval, Listed: res.Meta.FirstTradeDate}
	at := func(a []*float64, i int) float64 {
		if i < len(a) && a[i] != nil {
			return *a[i]
		}
		return 0
	}
	for i, t := range res.Timestamp {
		c := at(q.Close, i)
		if c <= 0 {
			continue
		}
		o, h, l := at(q.Open, i), at(q.High, i), at(q.Low, i)
		if o <= 0 {
			o = c
		}
		if h < c || h < o {
			h = max(c, o)
		}
		if l <= 0 || l > c || l > o {
			l = min(c, o)
		}
		s.Opens, s.Highs, s.Lows, s.Closes = append(s.Opens, o), append(s.Highs, h), append(s.Lows, l), append(s.Closes, c)
		s.Volumes, s.Times = append(s.Volumes, at(q.Volume, i)), append(s.Times, t)
	}
	intraMu.Lock()
	intraCache[key] = struct {
		s  *Series
		at time.Time
	}{s, time.Now()}
	intraMu.Unlock()
	return s, nil
}

func (a *App) hCandles(w http.ResponseWriter, r *http.Request) {
	t := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("t")))
	rng := r.URL.Query().Get("r")
	spec := map[string][2]string{"1D": {"1d", "5m"}, "5D": {"5d", "15m"}, "1MH": {"1mo", "60m"}}[rng]
	if t == "" || spec[0] == "" {
		http.Error(w, "bad request", 400)
		return
	}
	s, err := fetchSeries(t, spec[0], spec[1])
	if err != nil || s == nil || len(s.Closes) == 0 {
		if rng == "1D" {
			s, err = fetchSeries(t, "5d", "5m")
		}
		if err != nil || s == nil || len(s.Closes) == 0 {
			http.Error(w, "no intraday data", 404)
			return
		}
	}
	writeJSON(w, s)
}
