package main

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

type MarketItem struct {
	Sym, Name string
	Price     float64
	DayPct    float64
	OK        bool
	Spark     []float64
}

func (a *App) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/state", a.hState)
	mux.HandleFunc("/api/company", a.hCompany)
	mux.HandleFunc("/api/search", a.hSearch)
	mux.HandleFunc("/api/peers", a.hPeers)
	mux.HandleFunc("/api/track", a.hTrack)
	mux.HandleFunc("/api/untrack", a.hUntrack)
	mux.HandleFunc("/api/holding", a.hHolding)
	mux.HandleFunc("/api/holding/remove", a.hHoldingRemove)
	mux.HandleFunc("/api/holding/clear", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		a.store.Holdings = nil
		a.mu.Unlock()
		a.saveStore()
		go a.analyze(false)
		writeJSON(w, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/settings", a.hSettings)
	mux.HandleFunc("/api/feedback", a.hFeedback)
	mux.HandleFunc("/api/nav", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Hash string }
		readJSON(r, &req)
		a.mu.Lock()
		a.nav = req.Hash
		a.mu.Unlock()
		writeJSON(w, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/update", func(w http.ResponseWriter, r *http.Request) {
		go a.checkUpdate(true)
		writeJSON(w, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/startup", a.hStartup)
	mux.HandleFunc("/api/scan", func(w http.ResponseWriter, r *http.Request) {
		select {
		case a.trigger <- struct{}{}:
		default:
		}
		writeJSON(w, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/testnotify", func(w http.ResponseWriter, r *http.Request) {
		go toast("Seismo notifications work", "You'll get alerts like this when a big event hits a stock you hold or track.", "seismo://open")
		writeJSON(w, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/alerts/clear", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		a.store.Alerts = nil
		a.mu.Unlock()
		a.saveStore()
		writeJSON(w, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/quit", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]bool{"ok": true})
		go func() {
			time.Sleep(300 * time.Millisecond)
			a.shutdown()
		}()
	})
}

func (a *App) hState(w http.ResponseWriter, r *http.Request) {
	pv := a.portfolioView()
	a.mu.Lock()
	nav := a.nav
	a.nav = ""
	a.mu.Unlock()
	a.mu.RLock()
	defer a.mu.RUnlock()
	var market []MarketItem
	for _, m := range marketSyms {
		it := MarketItem{Sym: m.Sym, Name: m.Name}
		if q := a.quotes[m.Sym]; q != nil && q.OK {
			it.Price, it.DayPct, it.OK = q.Price, q.DayPct, true
			if len(q.Spark) > 30 {
				it.Spark = q.Spark[len(q.Spark)-30:]
			} else {
				it.Spark = q.Spark
			}
		}
		market = append(market, it)
	}
	regime, regimeNote := "—", ""
	if v, s := a.quotes["^VIX"], a.quotes["^GSPC"]; v != nil && v.OK && s != nil && s.OK {
		switch {
		case v.Price >= 30:
			regime, regimeNote = "FEAR", fmt.Sprintf("VIX %.1f: panic-level volatility. Big swings both ways.", v.Price)
		case v.Price >= 22 || s.DayPct <= -1:
			regime, regimeNote = "RISK-OFF", fmt.Sprintf("VIX %.1f, S&P %+.2f%%: investors are cautious.", v.Price, s.DayPct)
		case v.Price < 15 && s.DayPct >= 0:
			regime, regimeNote = "CALM / RISK-ON", fmt.Sprintf("VIX %.1f: low fear, buyers in control.", v.Price)
		default:
			regime, regimeNote = "NORMAL", fmt.Sprintf("VIX %.1f, S&P %+.2f%%.", v.Price, s.DayPct)
		}
	}
	var stats []*SourceStat
	for _, s := range a.stats {
		stats = append(stats, s)
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Group != stats[j].Group {
			return stats[i].Group < stats[j].Group
		}
		return stats[i].Name < stats[j].Name
	})
	out := map[string]any{
		"Portfolio": pv, "Market": market, "MarketStatus": marketStatus(), "Regime": regime, "RegimeNote": regimeNote,
		"Sources": stats, "Scanning": a.scanning, "ScanMsg": a.scanMsg, "LastScan": a.lastScan, "NextScan": a.nextScan,
		"Alerts": a.store.Alerts, "Settings": a.store.Settings, "TheoryCatalog": theoryCatalog, "Watch": a.store.Watch,
		"NArticles": len(a.articles), "Startup": startupEnabled(), "Now": time.Now(),
		"Nav": nav, "Rules": ruleCatalog(), "NCorrections": len(a.store.Overrides), "Version": version, "Update": a.updStatus,
	}
	if an := a.an; an != nil {
		ev := an.Events
		if len(ev) > 300 {
			ev = ev[:300]
		}
		out["Signals"], out["Events"], out["Macro"], out["SectorHeat"], out["NewsTone"], out["NCompanies"], out["Generated"] = an.Signals, ev, an.Macro, an.SectorHeat, an.NewsTone, an.NCompanies, an.Generated
	}
	writeJSON(w, out)
}

func (a *App) hCompany(w http.ResponseWriter, r *http.Request) {
	t := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("t")))
	if t == "" {
		http.Error(w, "missing t", 400)
		return
	}
	a.mu.RLock()
	q := a.quotes[t]
	a.mu.RUnlock()
	if q == nil || !q.OK || time.Since(q.Fetched) > 5*time.Minute {
		a.refreshQuotes([]string{t}, 0)
	}
	a.mu.RLock()
	q = a.quotes[t]
	an := a.an
	a.mu.RUnlock()
	var sig *Signal
	var events []*Cluster
	if an != nil {
		sig = an.bySig[t]
		for _, c := range an.Events {
			if c.Ticker == t {
				events = append(events, c)
			}
		}
	}
	co := a.company(t)
	if co == nil {
		co = &Company{Ticker: t, Name: t, Sector: "Other"}
		if hits, err := yahooSearch(t); err == nil {
			for _, h := range hits {
				if h.Symbol == t {
					co = companyFromHit(h)
				}
			}
		}
	}
	if sig == nil {
		if an == nil {
			an = &Analysis{bySig: map[string]*Signal{}, byKey: map[string]*Cluster{}}
		}
		sig = buildSignal(co, nil, an, q)
	}
	held, watch := a.tracked()
	resp := map[string]any{"Signal": sig, "Events": events, "Company": co, "Held": held[t], "Watched": watch[t]}
	if q != nil && q.OK {
		resp["Opens"], resp["Closes"], resp["Highs"], resp["Lows"], resp["Volumes"], resp["Times"] = q.Opens, q.Closes, q.Highs, q.Lows, q.Volumes, q.Times
		resp["Quote"] = q
	}
	writeJSON(w, resp)
}

func (a *App) hSearch(w http.ResponseWriter, r *http.Request) {
	qs := strings.TrimSpace(r.URL.Query().Get("q"))
	if qs == "" {
		writeJSON(w, []SearchHit{})
		return
	}
	var out []SearchHit
	seen := map[string]bool{}
	lq := strings.ToLower(qs)
	for _, c := range a.companies() {
		hit := strings.EqualFold(c.Ticker, qs) || strings.Contains(strings.ToLower(c.Name), lq)
		for _, al := range c.Aliases {
			if strings.Contains(strings.ToLower(strings.TrimPrefix(al, "~")), lq) && len(lq) >= 3 {
				hit = true
			}
		}
		if hit && !seen[c.Ticker] {
			seen[c.Ticker] = true
			out = append(out, SearchHit{Symbol: c.Ticker, Name: c.Name, Sector: c.Sector, Builtin: true})
		}
	}
	if hits, err := yahooSearch(qs); err == nil {
		for _, h := range hits {
			if !seen[h.Symbol] {
				seen[h.Symbol] = true
				out = append(out, h)
			}
		}
	}
	if len(out) > 15 {
		out = out[:15]
	}
	writeJSON(w, out)
}

type Peer struct {
	Symbol, Name, Sector, Reason string
	Price, DayPct, Expected      float64
	Outlook                      string
	OutlookLvl                   int
	HasQuote, Tracked            bool
}

func (a *App) hPeers(w http.ResponseWriter, r *http.Request) {
	t := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("t")))
	co := a.company(t)
	sector := ""
	if co != nil {
		sector = co.Sector
	} else if hits, err := yahooSearch(t); err == nil {
		for _, h := range hits {
			if h.Symbol == t {
				sector = mapYahooSector(h.Sector, h.Industry)
			}
		}
	}
	type pc struct{ sym, reason string }
	var list []pc
	seen := map[string]bool{t: true}
	for _, s := range yahooPeers(t) {
		if !seen[s] {
			seen[s] = true
			list = append(list, pc{s, "Investors who follow " + t + " also follow this"})
		}
	}
	if sector != "" && sector != "Other" {
		n := 0
		for _, c := range builtinCompanies() {
			if c.Sector == sector && !seen[c.Ticker] && n < 8 {
				seen[c.Ticker] = true
				list = append(list, pc{c.Ticker, "Same sector (" + sector + ")"})
				n++
			}
		}
	}
	if len(list) > 14 {
		list = list[:14]
	}
	syms := make([]string, len(list))
	for i, p := range list {
		syms[i] = p.sym
	}
	a.refreshQuotes(syms, 10*time.Minute)
	held, watch := a.tracked()
	a.mu.RLock()
	an := a.an
	var out []Peer
	for _, p := range list {
		pe := Peer{Symbol: p.sym, Name: p.sym, Reason: p.reason, Outlook: "—", Tracked: held[p.sym] || watch[p.sym]}
		if c := a.companyLocked(p.sym); c != nil {
			pe.Name, pe.Sector = c.Name, c.Sector
		}
		if q := a.quotes[p.sym]; q != nil && q.OK {
			pe.HasQuote, pe.Price, pe.DayPct = true, q.Price, q.DayPct
			if pe.Name == p.sym && q.Name != "" {
				pe.Name = q.Name
			}
		}
		if an != nil {
			if s := an.bySig[p.sym]; s != nil {
				pe.Outlook, pe.OutlookLvl, pe.Expected = s.Outlook, s.OutlookLvl, s.Expected
			}
		}
		out = append(out, pe)
	}
	a.mu.RUnlock()
	writeJSON(w, out)
}

func (a *App) ensureCompany(t string) (*Company, error) {
	if c := a.company(t); c != nil {
		return c, nil
	}
	hits, err := yahooSearch(t)
	var c *Company
	if err == nil {
		for _, h := range hits {
			if strings.EqualFold(h.Symbol, t) {
				c = companyFromHit(h)
				break
			}
		}
	}
	if c == nil {
		q := fetchQuote(t)
		if !q.OK {
			return nil, fmt.Errorf("couldn't find ticker %s", t)
		}
		c = &Company{Ticker: t, Name: firstNonEmpty(q.Name, t), Sector: "Other"}
		c = companyFromHit(SearchHit{Symbol: t, Name: c.Name})
	}
	a.mu.Lock()
	a.store.Extra[t] = c
	a.mu.Unlock()
	return c, nil
}

func (a *App) quickFetch(t string) {
	c := a.company(t)
	if c == nil {
		return
	}
	a.runFeeds(trackedFeeds(c))
	a.refreshQuotes([]string{t}, 0)
	a.analyze(false)
	a.saveCache()
}

func (a *App) hTrack(w http.ResponseWriter, r *http.Request) {
	var req struct{ Ticker string }
	if readJSON(r, &req) != nil || req.Ticker == "" {
		http.Error(w, "bad request", 400)
		return
	}
	t := strings.ToUpper(strings.TrimSpace(req.Ticker))
	if _, err := a.ensureCompany(t); err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	a.mu.Lock()
	if !contains(a.store.Watch, t) {
		a.store.Watch = append(a.store.Watch, t)
	}
	a.mu.Unlock()
	a.saveStore()
	go a.quickFetch(t)
	writeJSON(w, map[string]bool{"ok": true})
}

func (a *App) hUntrack(w http.ResponseWriter, r *http.Request) {
	var req struct{ Ticker string }
	readJSON(r, &req)
	t := strings.ToUpper(req.Ticker)
	a.mu.Lock()
	var nw []string
	for _, x := range a.store.Watch {
		if x != t {
			nw = append(nw, x)
		}
	}
	a.store.Watch = nw
	a.mu.Unlock()
	a.saveStore()
	go a.analyze(false)
	writeJSON(w, map[string]bool{"ok": true})
}

func (a *App) hHolding(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Ticker                     string
		Invested, Shares, BuyPrice float64
		Replace                    bool
	}
	if readJSON(r, &req) != nil || req.Ticker == "" {
		http.Error(w, "bad request", 400)
		return
	}
	t := strings.ToUpper(strings.TrimSpace(req.Ticker))
	if req.Invested < 0 || req.Shares < 0 || req.BuyPrice < 0 || req.Invested > 1e10 || req.Shares > 1e9 || req.BuyPrice > 1e7 {
		http.Error(w, "that amount doesn't look right (max $10 billion invested, 1 billion shares, $10M per share)", 400)
		return
	}
	if _, err := a.ensureCompany(t); err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	bp := req.BuyPrice
	if bp <= 0 && req.Invested > 0 && req.Shares > 0 {
		bp = req.Invested / req.Shares
	}
	if bp <= 0 {
		q := fetchQuote(t)
		if !q.OK {
			http.Error(w, "couldn't get a current price for "+t+"; enter your buy price", 400)
			return
		}
		bp = q.Price
		a.mu.Lock()
		a.quotes[t] = q
		a.mu.Unlock()
	}
	inv, sh := req.Invested, req.Shares
	switch {
	case inv > 0 && sh <= 0:
		sh = inv / bp
	case sh > 0 && inv <= 0:
		inv = sh * bp
	case inv <= 0 && sh <= 0:
		http.Error(w, "enter an amount invested or a number of shares", 400)
		return
	}
	a.mu.Lock()
	var found *Holding
	for _, h := range a.store.Holdings {
		if h.Ticker == t {
			found = h
		}
	}
	if found == nil {
		a.store.Holdings = append(a.store.Holdings, &Holding{Ticker: t, Invested: inv, Shares: sh, BuyPrice: bp, Added: time.Now()})
	} else if req.Replace {
		found.Invested, found.Shares, found.BuyPrice = inv, sh, bp
	} else {
		found.Invested += inv
		found.Shares += sh
		found.BuyPrice = found.Invested / math.Max(found.Shares, 1e-12)
	}
	a.mu.Unlock()
	a.saveStore()
	go a.quickFetch(t)
	writeJSON(w, map[string]bool{"ok": true})
}

func (a *App) hHoldingRemove(w http.ResponseWriter, r *http.Request) {
	var req struct{ Ticker string }
	readJSON(r, &req)
	t := strings.ToUpper(req.Ticker)
	a.mu.Lock()
	var nh []*Holding
	for _, h := range a.store.Holdings {
		if h.Ticker != t {
			nh = append(nh, h)
		}
	}
	a.store.Holdings = nh
	a.mu.Unlock()
	a.saveStore()
	go a.analyze(false)
	writeJSON(w, map[string]bool{"ok": true})
}

func (a *App) hSettings(w http.ResponseWriter, r *http.Request) {
	var s Settings
	if readJSON(r, &s) != nil {
		http.Error(w, "bad request", 400)
		return
	}
	if s.RefreshMin < 5 {
		s.RefreshMin = 15
	}
	var ts []string
	for _, t := range s.ScanTimes {
		if _, err := time.Parse("15:04", t); err == nil && !contains(ts, t) {
			ts = append(ts, t)
		}
	}
	if len(ts) == 0 {
		ts = defaultScanTimes()
	}
	s.ScanTimes = ts
	if s.Sensitivity == "" {
		s.Sensitivity = "normal"
	}
	a.mu.Lock()
	a.store.Settings = s
	a.mu.Unlock()
	a.saveStore()
	next := a.nextSlot(time.Now())
	a.mu.Lock()
	a.nextScan = next
	a.mu.Unlock()
	writeJSON(w, map[string]bool{"ok": true})
}

func (a *App) hStartup(w http.ResponseWriter, r *http.Request) {
	var req struct{ On bool }
	readJSON(r, &req)
	err := setStartup(req.On)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
