package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Quote struct {
	Symbol   string
	Name     string
	Price    float64
	Prev     float64
	DayPct   float64
	R5, R20  float64
	Vol      float64
	Spark    []float64
	Closes   []float64 `json:"-"`
	Highs    []float64 `json:"-"`
	Lows     []float64 `json:"-"`
	Volumes  []float64 `json:"-"`
	Times    []int64   `json:"-"`
	Currency string
	Fetched  time.Time
	OK       bool
	Err      string
	Src      string
}

var marketSyms = []struct{ Sym, Name string }{
	{"^GSPC", "S&P 500"}, {"^IXIC", "Nasdaq"}, {"^DJI", "Dow"}, {"^RUT", "Russell 2k"}, {"^VIX", "VIX"},
	{"^TNX", "US 10Y"}, {"DX-Y.NYB", "Dollar"}, {"CL=F", "Crude"}, {"GC=F", "Gold"}, {"BTC-USD", "Bitcoin"},
	{"^FTSE", "FTSE"}, {"^GDAXI", "DAX"}, {"^N225", "Nikkei"}, {"^HSI", "Hang Seng"}, {"000001.SS", "Shanghai"},
}

func fetchQuote(sym string) *Quote {
	q := fetchYahoo(sym, "query1")
	if !q.OK {
		q2 := fetchYahoo(sym, "query2")
		if q2.OK {
			return q2
		}
		q3 := fetchStooq(sym)
		if q3.OK {
			return q3
		}
	}
	return q
}

func fetchYahoo(sym, host string) *Quote {
	q := &Quote{Symbol: sym, Fetched: time.Now(), Src: "Yahoo"}
	b, err := httpGet("https://" + host + ".finance.yahoo.com/v8/finance/chart/" + url.PathEscape(sym) + "?range=1y&interval=1d")
	if err != nil {
		q.Err = err.Error()
		return q
	}
	var r struct {
		Chart struct {
			Result []struct {
				Meta struct {
					Currency           string
					RegularMarketPrice float64
					RegularMarketTime  int64
					Gmtoffset          int64
					LongName           string
					ShortName          string
				}
				Timestamp  []int64
				Indicators struct {
					Quote []struct {
						Close  []*float64
						High   []*float64
						Low    []*float64
						Volume []*float64
					}
				}
			}
		}
	}
	if err := json.Unmarshal(b, &r); err != nil || len(r.Chart.Result) == 0 {
		q.Err = "bad response"
		return q
	}
	res := r.Chart.Result[0]
	q.Currency = res.Meta.Currency
	q.Name = firstNonEmpty(res.Meta.LongName, res.Meta.ShortName)
	var closes, highs, lows, vols []float64
	var ts []int64
	at := func(a []*float64, i int, d float64) float64 {
		if i < len(a) && a[i] != nil && *a[i] > 0 {
			return *a[i]
		}
		return d
	}
	if len(res.Indicators.Quote) > 0 {
		qq := res.Indicators.Quote[0]
		for i, c := range qq.Close {
			if c != nil && i < len(res.Timestamp) && *c > 0 {
				closes = append(closes, *c)
				highs = append(highs, at(qq.High, i, *c))
				lows = append(lows, at(qq.Low, i, *c))
				vols = append(vols, at(qq.Volume, i, 0))
				ts = append(ts, res.Timestamp[i])
			}
		}
	}
	price := res.Meta.RegularMarketPrice
	if price <= 0 && len(closes) > 0 {
		price = closes[len(closes)-1]
	}
	if price <= 0 {
		q.Err = "no price"
		return q
	}
	day := func(t int64) int64 { return (t + res.Meta.Gmtoffset) / 86400 }
	if n := len(closes); n > 0 && day(ts[n-1]) == day(res.Meta.RegularMarketTime) {
		closes[n-1] = price
		highs[n-1] = math.Max(highs[n-1], price)
		lows[n-1] = math.Min(lows[n-1], price)
	} else {
		closes = append(closes, price)
		highs = append(highs, price)
		lows = append(lows, price)
		vols = append(vols, 0)
		ts = append(ts, res.Meta.RegularMarketTime)
	}
	q.Highs, q.Lows, q.Volumes, q.Times = highs, lows, vols, ts
	fill(q, closes)
	return q
}

var stooqMap = map[string]string{"^GSPC": "^spx", "^DJI": "^dji", "^IXIC": "^ndq", "^N225": "^nkx", "^FTSE": "^ukx", "^GDAXI": "^dax", "^HSI": "^hsi", "BTC-USD": "btcusd", "GC=F": "gc.f", "CL=F": "cl.f"}
var plainTicker = regexp.MustCompile(`^[A-Z]{1,5}(-[A-Z])?$`)

func fetchStooq(sym string) *Quote {
	q := &Quote{Symbol: sym, Fetched: time.Now(), Src: "Stooq"}
	s, ok := stooqMap[sym]
	if !ok {
		if !plainTicker.MatchString(sym) {
			q.Err = "unsupported"
			return q
		}
		s = strings.ToLower(strings.ReplaceAll(sym, "-", ".")) + ".us"
	}
	b, err := httpGet("https://stooq.com/q/d/l/?s=" + url.QueryEscape(s) + "&i=d&d1=" + time.Now().AddDate(-1, 0, 0).Format("20060102"))
	if err != nil {
		q.Err = err.Error()
		return q
	}
	rows, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if err != nil || len(rows) < 3 {
		q.Err = "no data"
		return q
	}
	var closes []float64
	pf := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
	for _, r := range rows[1:] {
		if len(r) >= 5 {
			if v := pf(r[4]); v > 0 {
				closes = append(closes, v)
				q.Highs = append(q.Highs, math.Max(pf(r[2]), v))
				lo := pf(r[3])
				if lo <= 0 {
					lo = v
				}
				q.Lows = append(q.Lows, lo)
				vv := 0.0
				if len(r) >= 6 {
					vv = pf(r[5])
				}
				q.Volumes = append(q.Volumes, vv)
				tt, _ := time.Parse("2006-01-02", r[0])
				q.Times = append(q.Times, tt.Unix())
			}
		}
	}
	if len(closes) < 2 {
		q.Err = "no data"
		return q
	}
	fill(q, closes)
	return q
}

func fill(q *Quote, closes []float64) {
	n := len(closes)
	q.Price = closes[n-1]
	if n >= 2 {
		q.Prev = closes[n-2]
		q.DayPct = (q.Price/q.Prev - 1) * 100
	}
	if n > 5 {
		q.R5 = (q.Price/closes[n-6] - 1) * 100
	}
	if n > 21 {
		q.R20 = (q.Price/closes[n-22] - 1) * 100
	} else if n > 1 {
		q.R20 = (q.Price/closes[0] - 1) * 100
	}
	var rets []float64
	for i := max(1, n-21); i < n; i++ {
		rets = append(rets, math.Log(closes[i]/closes[i-1]))
	}
	if len(rets) > 2 {
		var mu, v float64
		for _, r := range rets {
			mu += r
		}
		mu /= float64(len(rets))
		for _, r := range rets {
			v += (r - mu) * (r - mu)
		}
		q.Vol = math.Sqrt(v/float64(len(rets)-1)) * 100
	}
	q.Closes = closes
	if len(q.Highs) != n {
		q.Highs, q.Lows, q.Volumes = closes, closes, make([]float64, n)
	}
	if len(q.Times) != n {
		q.Times = make([]int64, n)
		for i := range q.Times {
			q.Times[i] = time.Now().AddDate(0, 0, -(n-1-i)*7/5).Unix()
		}
	}
	q.Spark = closes
	if n > 90 {
		q.Spark = closes[n-90:]
	}
	q.OK = true
}

func fetchQuotes(syms []string) map[string]*Quote {
	out := map[string]*Quote{}
	var mu sync.Mutex
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, s := range syms {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			sem <- struct{}{}
			q := fetchQuote(s)
			<-sem
			mu.Lock()
			out[s] = q
			mu.Unlock()
		}(s)
	}
	wg.Wait()
	return out
}

type SearchHit struct {
	Symbol   string
	Name     string
	Exchange string
	Type     string
	Sector   string
	Industry string
	Builtin  bool
}

func yahooSearch(q string) ([]SearchHit, error) {
	b, err := httpGet("https://query2.finance.yahoo.com/v1/finance/search?q=" + url.QueryEscape(q) + "&quotesCount=12&newsCount=0")
	if err != nil {
		return nil, err
	}
	var r struct {
		Quotes []struct {
			Symbol    string
			Shortname string
			Longname  string
			ExchDisp  string
			QuoteType string
			Sector    string
			Industry  string
		}
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	var out []SearchHit
	for _, x := range r.Quotes {
		if x.Symbol == "" || (x.QuoteType != "EQUITY" && x.QuoteType != "ETF") {
			continue
		}
		out = append(out, SearchHit{Symbol: x.Symbol, Name: firstNonEmpty(x.Longname, x.Shortname, x.Symbol), Exchange: x.ExchDisp, Type: x.QuoteType, Sector: x.Sector, Industry: x.Industry})
	}
	return out, nil
}

func yahooPeers(sym string) []string {
	b, err := httpGet("https://query2.finance.yahoo.com/v6/finance/recommendationsbysymbol/" + url.PathEscape(sym))
	if err != nil {
		return nil
	}
	var r struct {
		Finance struct {
			Result []struct {
				RecommendedSymbols []struct{ Symbol string }
			}
		}
	}
	if json.Unmarshal(b, &r) != nil || len(r.Finance.Result) == 0 {
		return nil
	}
	var out []string
	for _, s := range r.Finance.Result[0].RecommendedSymbols {
		out = append(out, s.Symbol)
	}
	return out
}

var nameSuffix = regexp.MustCompile(`(?i)[,.]?\s+(inc\.?|incorporated|corp\.?|corporation|co\.?|company|holdings?|group|plc|ltd\.?|limited|s\.?a\.?|n\.?v\.?|ag|se|class [a-c]|the)$`)

func companyFromHit(h SearchHit) *Company {
	n := strings.TrimSpace(h.Name)
	for i := 0; i < 3; i++ {
		n = strings.TrimSpace(nameSuffix.ReplaceAllString(n, ""))
	}
	n = strings.TrimPrefix(n, "The ")
	c := &Company{Ticker: h.Symbol, Name: h.Name, Industry: h.Industry, Sector: mapYahooSector(h.Sector, h.Industry)}
	if n != "" {
		if len(n) >= 5 && strings.Contains(n, " ") || len(n) >= 7 {
			c.Aliases = append(c.Aliases, n)
		} else {
			c.Aliases = append(c.Aliases, "~"+n)
		}
		if f := strings.Fields(n); len(f) > 1 && len(f[0]) >= 5 {
			c.Aliases = append(c.Aliases, "~"+f[0])
		}
	}
	if len(c.Aliases) == 0 {
		c.Aliases = []string{fmt.Sprintf("~%s", h.Symbol)}
	}
	return c
}
