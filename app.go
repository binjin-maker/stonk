package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Holding struct {
	Ticker   string
	Invested float64
	Shares   float64
	BuyPrice float64
	Added    time.Time
}

type Settings struct {
	RefreshMin   int
	ScanTimes    []string
	Theories     []string
	NotifyOn     bool
	NotifyWatch  bool
	Sensitivity  string
	NoAutoUpdate bool
}

type NotifRec struct {
	Impact float64
	At     time.Time
	Class  string
}

type Alert struct {
	At     time.Time
	Ticker string
	Title  string
	Body   string
	Level  string
	Key    string
}

type Store struct {
	Holdings  []*Holding
	Watch     []string
	Extra     map[string]*Company
	Settings  Settings
	LastScan  time.Time
	Notified  map[string]NotifRec
	Alerts    []Alert
	Overrides map[string]Override
	Learned   Learned
}

type App struct {
	mu        sync.RWMutex
	articles  map[string]*Article
	quotes    map[string]*Quote
	store     Store
	an        *Analysis
	stats     map[string]*SourceStat
	scanning  bool
	scanMsg   string
	lastScan  time.Time
	nextScan  time.Time
	trigger   chan struct{}
	nav       string
	saveMu    sync.Mutex
	updStatus string
}

func NewApp() *App {
	return &App{
		articles: map[string]*Article{},
		quotes:   map[string]*Quote{},
		stats:    map[string]*SourceStat{},
		trigger:  make(chan struct{}, 1),
		store: Store{Extra: map[string]*Company{}, Notified: map[string]NotifRec{},
			Settings: Settings{RefreshMin: 15, ScanTimes: defaultScanTimes(), NotifyOn: true, NotifyWatch: true, Sensitivity: "normal", Theories: []string{"news", "box", "dow", "ma", "rsi", "macd", "contra"}}},
	}
}

func (a *App) Load() {
	sp := filepath.Join(appDir, "store.json")
	if b, err := os.ReadFile(sp); err == nil {
		if json.Unmarshal(b, &a.store) != nil {
			os.WriteFile(filepath.Join(appDir, fmt.Sprintf("store.corrupt-%d.json", time.Now().Unix())), b, 0644)
			for _, bk := range []string{"store.backup.json", "store.prev.json"} {
				if bb, err := os.ReadFile(filepath.Join(appDir, bk)); err == nil && json.Unmarshal(bb, &a.store) == nil {
					log.Printf("store.json was unreadable, restored from %s", bk)
					break
				}
			}
		}
	} else if bb, err := os.ReadFile(filepath.Join(appDir, "store.prev.json")); err == nil {
		json.Unmarshal(bb, &a.store)
	}
	if a.store.Extra == nil {
		a.store.Extra = map[string]*Company{}
	}
	if a.store.Notified == nil {
		a.store.Notified = map[string]NotifRec{}
	}
	if a.store.Settings.RefreshMin < 5 {
		a.store.Settings.RefreshMin = 15
	}
	if len(a.store.Settings.ScanTimes) == 0 {
		a.store.Settings.ScanTimes = defaultScanTimes()
	}
	a.lastScan = a.store.LastScan
	var keep []*Holding
	for _, h := range a.store.Holdings {
		if h.Invested <= 1e10 && h.Shares <= 1e9 && h.Invested >= 0 && h.Shares >= 0 {
			keep = append(keep, h)
		}
	}
	a.store.Holdings = keep
	if b, err := os.ReadFile(filepath.Join(appDir, "cache.json")); err == nil {
		var arts []*Article
		if json.Unmarshal(b, &arts) == nil {
			for _, x := range arts {
				a.articles[x.ID] = x
			}
		}
	}
}

func (a *App) saveStore() {
	a.mu.RLock()
	b, _ := json.MarshalIndent(a.store, "", " ")
	a.mu.RUnlock()
	if len(b) < 10 {
		return
	}
	a.saveMu.Lock()
	sp := filepath.Join(appDir, "store.json")
	if old, err := os.ReadFile(sp); err == nil && json.Valid(old) {
		os.WriteFile(filepath.Join(appDir, "store.prev.json"), old, 0644)
	}
	writeAtomic(sp, b)
	a.saveMu.Unlock()
}

func (a *App) saveCache() {
	a.mu.RLock()
	arts := make([]*Article, 0, len(a.articles))
	for _, x := range a.articles {
		arts = append(arts, x)
	}
	a.mu.RUnlock()
	b, _ := json.Marshal(arts)
	a.saveMu.Lock()
	writeAtomic(filepath.Join(appDir, "cache.json"), b)
	a.saveMu.Unlock()
}

func writeAtomic(p string, b []byte) {
	tmp := p + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return
	}
	_, err = f.Write(b)
	f.Sync()
	f.Close()
	if err == nil {
		os.Rename(tmp, p)
	}
}

func (a *App) companies() []*Company {
	cos := builtinCompanies()
	have := map[string]bool{}
	for _, c := range cos {
		have[c.Ticker] = true
	}
	a.mu.RLock()
	for _, c := range a.store.Extra {
		if !have[c.Ticker] {
			cos = append(cos, c)
		}
	}
	a.mu.RUnlock()
	return cos
}

func (a *App) company(t string) *Company {
	for _, c := range a.companies() {
		if c.Ticker == t {
			return c
		}
	}
	return nil
}

func (a *App) tracked() (held, watch map[string]bool) {
	held, watch = map[string]bool{}, map[string]bool{}
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, h := range a.store.Holdings {
		held[h.Ticker] = true
	}
	for _, w := range a.store.Watch {
		watch[w] = true
	}
	return
}

func (a *App) Run() {
	held, watch := a.tracked()
	syms := marketList()
	for t := range mergeSets(held, watch) {
		syms = append(syms, t)
	}
	a.refreshQuotes(syms, 0)
	a.analyze(false)
	go a.quoteLoop()
	a.mu.RLock()
	last := a.store.LastScan
	a.mu.RUnlock()
	a.mu.Lock()
	a.nextScan = a.nextSlotLocked()
	a.mu.Unlock()
	if last.IsZero() || last.Before(a.prevSlot(time.Now())) {
		a.scan()
	}
	for {
		next := a.nextSlot(time.Now())
		a.mu.Lock()
		a.nextScan = next
		a.mu.Unlock()
		tick := time.NewTicker(30 * time.Second)
	wait:
		for {
			select {
			case <-a.trigger:
				break wait
			case <-tick.C:
				if !time.Now().Before(next) {
					break wait
				}
			}
		}
		tick.Stop()
		a.scan()
	}
}

func (a *App) nextSlotLocked() time.Time {
	a.mu.Unlock()
	defer a.mu.Lock()
	return a.nextSlot(time.Now())
}

func defaultScanTimes() []string { return []string{"06:00", "09:30", "12:30", "17:00"} }

func (a *App) slots(day time.Time) []time.Time {
	a.mu.RLock()
	ts := append([]string(nil), a.store.Settings.ScanTimes...)
	a.mu.RUnlock()
	if len(ts) == 0 {
		ts = defaultScanTimes()
	}
	var out []time.Time
	for _, s := range ts {
		t, err := time.ParseInLocation("15:04", s, time.Local)
		if err != nil {
			continue
		}
		out = append(out, time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), 0, 0, time.Local))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

func (a *App) nextSlot(now time.Time) time.Time {
	for d := 0; d < 3; d++ {
		for _, s := range a.slots(now.AddDate(0, 0, d)) {
			if s.After(now) {
				return s
			}
		}
	}
	return now.Add(6 * time.Hour)
}

func (a *App) prevSlot(now time.Time) time.Time {
	for d := 0; d < 3; d++ {
		ss := a.slots(now.AddDate(0, 0, -d))
		for i := len(ss) - 1; i >= 0; i-- {
			if !ss[i].After(now) {
				return ss[i]
			}
		}
	}
	return now.Add(-24 * time.Hour)
}

func (a *App) quoteLoop() {
	for {
		time.Sleep(90 * time.Second)
		a.mu.RLock()
		busy := a.scanning
		a.mu.RUnlock()
		if busy {
			continue
		}
		held, watch := a.tracked()
		syms := marketList()
		for t := range held {
			syms = append(syms, t)
		}
		for t := range watch {
			syms = append(syms, t)
		}
		a.refreshQuotes(syms, 80*time.Second)
		a.analyze(false)
	}
}

func marketList() []string {
	var s []string
	for _, m := range marketSyms {
		s = append(s, m.Sym)
	}
	return s
}

func (a *App) refreshQuotes(syms []string, maxAge time.Duration) {
	var need []string
	seen := map[string]bool{}
	a.mu.RLock()
	for _, s := range syms {
		if seen[s] {
			continue
		}
		seen[s] = true
		if q := a.quotes[s]; q == nil || time.Since(q.Fetched) > maxAge {
			need = append(need, s)
		}
	}
	a.mu.RUnlock()
	if len(need) == 0 {
		return
	}
	got := fetchQuotes(need)
	a.mu.Lock()
	for s, q := range got {
		if old := a.quotes[s]; q.OK || old == nil || !old.OK {
			a.quotes[s] = q
		}
	}
	a.mu.Unlock()
}

func (a *App) setMsg(scanning bool, msg string) {
	a.mu.Lock()
	a.scanning, a.scanMsg = scanning, msg
	a.mu.Unlock()
}

func (a *App) runFeeds(feeds []Feed) int {
	type res struct {
		f    Feed
		arts []*Article
		err  error
		ms   int64
	}
	ch := make(chan res)
	sem := make(chan struct{}, 10)
	for _, f := range feeds {
		go func(f Feed) {
			sem <- struct{}{}
			t0 := time.Now()
			b, err := httpGet(f.URL)
			var arts []*Article
			if err == nil {
				if f.Kind == "yahoo" {
					arts = parseYahooNews(b, f)
				} else {
					arts = parseFeed(b, f)
				}
			}
			<-sem
			ch <- res{f, arts, err, time.Since(t0).Milliseconds()}
		}(f)
	}
	total := 0
	for i := range feeds {
		r := <-ch
		total += a.merge(r.arts)
		st := &SourceStat{Name: r.f.Name, Group: r.f.Group, Count: len(r.arts), OK: r.err == nil, Ms: r.ms, Last: time.Now()}
		if r.err != nil {
			st.Err = r.err.Error()
		}
		a.mu.Lock()
		a.stats[r.f.Name] = st
		a.scanMsg = fmt.Sprintf("Scanning sources… %d/%d", i+1, len(feeds))
		a.mu.Unlock()
	}
	return total
}

func (a *App) merge(arts []*Article) int {
	cut := time.Now().Add(-7 * 24 * time.Hour)
	n := 0
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, x := range arts {
		if x.Title == "" || x.Published.Before(cut) {
			continue
		}
		k := articleKey(x.Title)
		if old, ok := a.articles[k]; ok {
			for _, t := range x.Tickers {
				if !contains(old.Tickers, t) {
					old.Tickers = append(old.Tickers, t)
				}
			}
			if old.Summary == "" {
				old.Summary = x.Summary
			}
			continue
		}
		x.ID = k
		x.Cred = credFor(x.Source, x.Feed)
		a.articles[k] = x
		n++
	}
	return n
}

func (a *App) scan() {
	a.setMsg(true, "Scanning sources…")
	t0 := time.Now()
	feeds := baseFeeds(builtinCompanies())
	held, watch := a.tracked()
	for t := range mergeSets(held, watch) {
		if c := a.company(t); c != nil {
			feeds = append(feeds, trackedFeeds(c)...)
		}
	}
	done := make(chan int)
	go func() {
		n := 0
		for i, q := range gdeltQueries {
			if i > 0 {
				time.Sleep(6 * time.Second)
			}
			name := fmt.Sprintf("GDELT global #%d", i+1)
			t := time.Now()
			b, err := httpGet(gdeltURL(q))
			var arts []*Article
			if err == nil {
				arts = parseGdelt(b, name)
			}
			n += a.merge(arts)
			st := &SourceStat{Name: name, Group: "GDELT (worldwide, 100+ countries)", Count: len(arts), OK: err == nil, Ms: time.Since(t).Milliseconds(), Last: time.Now()}
			if err != nil {
				st.Err = err.Error()
			}
			a.mu.Lock()
			a.stats[name] = st
			a.mu.Unlock()
		}
		done <- n
	}()
	added := a.runFeeds(feeds)
	a.setMsg(true, "Finishing worldwide GDELT sweep…")
	added += <-done
	a.prune()
	a.setMsg(true, "Pulling stock prices…")
	a.analyze(true)
	a.mu.Lock()
	a.scanning = false
	a.scanMsg = fmt.Sprintf("Last scan: %d new articles in %.0fs", added, time.Since(t0).Seconds())
	a.lastScan = time.Now()
	a.store.LastScan = a.lastScan
	a.mu.Unlock()
	a.saveStore()
	a.saveCache()
	log.Printf("scan done: +%d articles, total %d", added, len(a.articles))
}

func (a *App) prune() {
	cut := time.Now().Add(-7 * 24 * time.Hour)
	a.mu.Lock()
	defer a.mu.Unlock()
	for k, x := range a.articles {
		if x.Published.Before(cut) {
			delete(a.articles, k)
		}
	}
	if len(a.articles) > 9000 {
		arts := make([]*Article, 0, len(a.articles))
		for _, x := range a.articles {
			arts = append(arts, x)
		}
		sort.Slice(arts, func(i, j int) bool { return arts[i].Published.After(arts[j].Published) })
		for _, x := range arts[9000:] {
			delete(a.articles, x.ID)
		}
	}
}

func (a *App) analyze(fetch bool) {
	held, watch := a.tracked()
	cos := a.companies()
	run := func() *Analysis {
		a.mu.RLock()
		arts := make([]*Article, 0, len(a.articles))
		for _, x := range a.articles {
			cp := *x
			cp.Tickers = append([]string(nil), x.Tickers...)
			arts = append(arts, &cp)
		}
		qs := make(map[string]*Quote, len(a.quotes))
		for k, v := range a.quotes {
			qs[k] = v
		}
		ovs := make(map[string]Override, len(a.store.Overrides))
		for k, v := range a.store.Overrides {
			ovs[k] = v
		}
		lr := &Learned{CoNeg: map[string][]string{}, EvNeg: map[string][]string{}}
		for k, v := range a.store.Learned.CoNeg {
			lr.CoNeg[k] = append([]string(nil), v...)
		}
		for k, v := range a.store.Learned.EvNeg {
			lr.EvNeg[k] = append([]string(nil), v...)
		}
		a.mu.RUnlock()
		return Analyze(AnalysisInput{Articles: arts, Companies: cos, Quotes: qs, Held: held, Watch: watch, Overrides: ovs, Learned: lr, Now: time.Now()})
	}
	an := run()
	if fetch {
		syms := marketList()
		var pri []string
		for t := range mergeSets(held, watch) {
			pri = append(pri, t)
		}
		var other []string
		for i, s := range an.Signals {
			if i >= 45 {
				break
			}
			other = append(other, s.Ticker)
		}
		a.refreshQuotes(append(syms, pri...), 80*time.Second)
		a.refreshQuotes(other, 10*time.Minute)
		an = run()
	}
	a.mu.Lock()
	a.an = an
	a.mu.Unlock()
	a.checkAlerts(an)
}

func mergeSets(xs ...map[string]bool) map[string]bool {
	o := map[string]bool{}
	for _, x := range xs {
		for k := range x {
			o[k] = true
		}
	}
	return o
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

type HoldingView struct {
	Ticker, Name               string
	Invested, Shares, BuyPrice float64
	Price, Value, PL, PLPct    float64
	DayPct, DayPL, Share       float64
	ExpPct, ExpUSD, LowUSD     float64
	HighUSD                    float64
	Outlook                    string
	OutlookLvl, Flight         int
	FlightLabel, Forecast      string
	HasQuote                   bool
	Theories                   []Theory
	Spark                      []float64
}

type PortfolioView struct {
	Holdings                          []HoldingView
	Invested, Value, PL, PLPct, DayPL float64
	ExpUSD, ExpPct, DayPct            float64
}

func (a *App) portfolioView() PortfolioView {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var pv PortfolioView
	for _, h := range a.store.Holdings {
		v := HoldingView{Ticker: h.Ticker, Name: h.Ticker, Invested: h.Invested, Shares: h.Shares, BuyPrice: h.BuyPrice, Price: h.BuyPrice, Outlook: "—"}
		if c := a.companyLocked(h.Ticker); c != nil {
			v.Name = c.Name
		}
		if q := a.quotes[h.Ticker]; q != nil && q.OK {
			v.HasQuote, v.Price, v.DayPct, v.Spark = true, q.Price, q.DayPct, q.Spark
			if v.Name == h.Ticker && q.Name != "" {
				v.Name = q.Name
			}
		}
		v.Value = v.Shares * v.Price
		v.PL = v.Value - v.Invested
		if v.Invested > 0 {
			v.PLPct = v.PL / v.Invested * 100
		}
		if v.HasQuote {
			v.DayPL = v.Value - v.Value/(1+v.DayPct/100)
			if y, m, d := h.Added.Date(); y == time.Now().Year() && m == time.Now().Month() && d == time.Now().Day() {
				v.DayPL = v.PL
			}
		}
		if a.an != nil {
			if s := a.an.bySig[h.Ticker]; s != nil {
				v.ExpPct, v.Outlook, v.OutlookLvl, v.Flight, v.FlightLabel, v.Theories = s.Expected, s.Outlook, s.OutlookLvl, s.Flight, s.FlightLabel, s.Theories
				v.ExpUSD = v.Value * s.Expected / 100
				v.LowUSD = v.Value * s.Low / 100
				v.HighUSD = v.Value * s.High / 100
				v.Forecast = s.Narrative + fmt.Sprintf(" On your $%s position that's roughly %s (range %s to %s).", money(v.Value), smoney(v.ExpUSD), smoney(v.LowUSD), smoney(v.HighUSD))
			}
		}
		pv.Invested += v.Invested
		pv.Value += v.Value
		pv.DayPL += v.DayPL
		pv.ExpUSD += v.ExpUSD
		pv.Holdings = append(pv.Holdings, v)
	}
	pv.PL = pv.Value - pv.Invested
	if pv.Invested > 0 {
		pv.PLPct = pv.PL / pv.Invested * 100
	}
	if pv.Value > 0 {
		pv.ExpPct = pv.ExpUSD / pv.Value * 100
		pv.DayPct = pv.DayPL / (pv.Value - pv.DayPL) * 100
		for i := range pv.Holdings {
			pv.Holdings[i].Share = pv.Holdings[i].Value / pv.Value
		}
	}
	sort.Slice(pv.Holdings, func(i, j int) bool { return pv.Holdings[i].Value > pv.Holdings[j].Value })
	return pv
}

func (a *App) companyLocked(t string) *Company {
	if c, ok := a.store.Extra[t]; ok {
		return c
	}
	for _, r := range builtinRaw {
		if r.T == t {
			return &Company{Ticker: r.T, Name: r.N, Sector: r.S}
		}
	}
	return nil
}

func money(x float64) string {
	neg := x < 0
	x = math.Abs(x)
	s := strconv.FormatFloat(x, 'f', 0, 64)
	if x < 100 {
		s = strconv.FormatFloat(x, 'f', 2, 64)
	}
	ip := s
	dp := ""
	if i := strings.Index(s, "."); i >= 0 {
		ip, dp = s[:i], s[i:]
	}
	var b strings.Builder
	for i, ch := range ip {
		if i > 0 && (len(ip)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	r := b.String() + dp
	if neg {
		return "-" + r
	}
	return r
}

func smoney(x float64) string {
	if x >= 0 {
		return "+$" + money(x)
	}
	return "-$" + money(-x)
}

func (a *App) checkAlerts(an *Analysis) {
	a.mu.RLock()
	set := a.store.Settings
	a.mu.RUnlock()
	if !set.NotifyOn {
		return
	}
	pv := a.portfolioView()
	sens := map[string]float64{"low": 1.6, "normal": 1, "high": 0.6}[set.Sensitivity]
	if sens == 0 {
		sens = 1
	}
	held, watch := a.tracked()
	type cand struct {
		key, ticker, title, body, level, class string
		impact, score                          float64
	}
	var cands []cand
	pos := map[string]HoldingView{}
	for _, h := range pv.Holdings {
		pos[h.Ticker] = h
	}
	now := time.Now()
	for t := range mergeSets(held, watch) {
		s := an.bySig[t]
		if s == nil {
			continue
		}
		hv, isHeld := pos[t]
		if !isHeld && !set.NotifyWatch {
			continue
		}
		share := hv.Share
		for _, k := range s.EventKeys {
			c := an.byKey[k]
			if c == nil || c.Coverage < 0.3 || now.Sub(c.LastSeen) > 36*time.Hour {
				continue
			}
			imp := c.Impact
			usd := hv.Value * imp / 100
			big := math.Abs(imp) >= 5*sens ||
				(c.Dir < 0 && c.Flight >= 2 && math.Abs(imp) >= 1.5*sens) ||
				(isHeld && math.Abs(imp) >= 2.5*sens && share >= 0.25) ||
				(isHeld && math.Abs(imp) >= 2*sens && math.Abs(usd) >= 0.015*pv.Value*sens && pv.Value > 0)
			if !isHeld {
				big = math.Abs(imp) >= 5*sens || (c.Dir < 0 && c.Flight >= 2 && math.Abs(imp) >= 3*sens)
			}
			if !big {
				continue
			}
			lvl := "good"
			icon := "▲"
			if imp < 0 {
				lvl, icon = "bad", "▼"
			}
			title := fmt.Sprintf("%s %s: %s [%s]", icon, t, c.Label, c.Class)
			body := fmt.Sprintf("Est. %+.1f%%", imp)
			if isHeld {
				body += fmt.Sprintf(" ≈ %s on your $%s.", smoney(usd), money(hv.Value))
			} else {
				body += " (watchlist)."
			}
			if c.Dir < 0 && c.Flight >= 2 {
				body += " Investor-flight risk " + []string{"low", "elevated", "HIGH", "SEVERE"}[c.Flight] + "."
			}
			if len(c.Articles) > 0 {
				body += " " + c.Articles[0].Title
			}
			cands = append(cands, cand{k, t, title, body, lvl, c.Class, imp, math.Abs(imp) * (1 + 3*share)})
		}
		if isHeld && hv.HasQuote && (hv.DayPct <= -6*sens || hv.DayPct >= 7*sens) {
			dir := "up"
			lvl := "good"
			if hv.DayPct < 0 {
				dir, lvl = "down", "bad"
			}
			cands = append(cands, cand{fmt.Sprintf("px|%s|%s|%s", t, now.Format("2006-01-02"), dir), t,
				fmt.Sprintf("%s is %s %.1f%% today", t, dir, math.Abs(hv.DayPct)),
				fmt.Sprintf("Your position moved %s today. Now worth $%s.", smoney(hv.DayPL), money(hv.Value)), lvl, "PRICE", hv.DayPct, math.Abs(hv.DayPct) * (1 + 3*share)})
		}
		if isHeld && share >= 0.2 {
			for _, d := range s.Drivers {
				if d.Kind == "macro" && math.Abs(d.Impact) >= 2*sens {
					lvl := "good"
					if d.Impact < 0 {
						lvl = "bad"
					}
					cands = append(cands, cand{"m|" + t + "|" + d.Key, t, fmt.Sprintf("World event hits %s: %s", t, d.Label),
						fmt.Sprintf("Est. %+.1f%% ≈ %s on your $%s.", d.Impact, smoney(hv.Value*d.Impact/100), money(hv.Value)), lvl, d.Class, d.Impact, math.Abs(d.Impact) * (1 + 3*share)})
				}
			}
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	var fire []cand
	a.mu.Lock()
	for _, c := range cands {
		rec, ok := a.store.Notified[c.key]
		if ok && math.Abs(c.impact) < math.Abs(rec.Impact)*1.8 && rec.Class == c.class {
			continue
		}
		if ok && now.Sub(rec.At) < 3*time.Hour {
			continue
		}
		a.store.Notified[c.key] = NotifRec{Impact: c.impact, At: now, Class: c.class}
		fire = append(fire, c)
		al := Alert{At: now, Ticker: c.ticker, Title: c.title, Body: c.body, Level: c.level, Key: c.key}
		a.store.Alerts = append([]Alert{al}, a.store.Alerts...)
	}
	if len(a.store.Alerts) > 150 {
		a.store.Alerts = a.store.Alerts[:150]
	}
	for k, r := range a.store.Notified {
		if now.Sub(r.At) > 10*24*time.Hour {
			delete(a.store.Notified, k)
		}
	}
	a.mu.Unlock()
	if len(fire) == 0 {
		return
	}
	for i, c := range fire {
		if i == 3 {
			go toast(fmt.Sprintf("%d more alerts", len(fire)-3), "Open Seismo to see all of them.", "seismo://alerts")
			break
		}
		go toast(c.title, c.body, "seismo://t/"+c.ticker)
	}
	a.saveStore()
}

func appURL() string { return fmt.Sprintf("http://127.0.0.1:%d/", port) }

func marketStatus() string {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return "UNKNOWN"
	}
	t := time.Now().In(loc)
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return "CLOSED"
	}
	m := t.Hour()*60 + t.Minute()
	switch {
	case m >= 570 && m < 960:
		return "OPEN"
	case m >= 240 && m < 570:
		return "PRE-MARKET"
	case m >= 960 && m < 1200:
		return "AFTER-HOURS"
	}
	return "CLOSED"
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func (a *App) shutdown() {
	a.saveStore()
	a.saveCache()
	stopTray()
	os.Exit(0)
}
