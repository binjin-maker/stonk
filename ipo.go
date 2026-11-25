package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type IPO struct {
	ID         string
	Ticker     string
	Name       string
	Exchange   string
	Status     string
	Price      float64
	PriceRange string
	Shares     float64
	Value      float64
	Date       time.Time
	Filed      time.Time
	Withdrawn  time.Time
	Lockup     time.Time
	HasQuote   bool
	Last       float64
	DayPct     float64
	FirstOpen  float64
	FirstClose float64
	PopPct     float64
	SinceIPO   float64
	Mega       bool
	Spark      []float64
}

type IPOData struct {
	Upcoming  []*IPO
	Recent    []*IPO
	Filed     []*IPO
	Withdrawn []*IPO
	Updated   time.Time
	Err       string
}

var ipoMu sync.Mutex

func httpGetNasdaq(u string) ([]byte, error) {
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://www.nasdaq.com")
	req.Header.Set("Referer", "https://www.nasdaq.com/")
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return b, err
}

var numClean = regexp.MustCompile(`[^0-9.]`)

func pnum(s string) float64 {
	v, _ := strconv.ParseFloat(numClean.ReplaceAllString(s, ""), 64)
	return v
}

func pdate(s string) time.Time {
	for _, f := range []string{"1/2/2006", "01/02/2006", "2006-01-02"} {
		if t, err := time.Parse(f, strings.TrimSpace(s)); err == nil {
			return t.Add(17 * time.Hour)
		}
	}
	return time.Time{}
}

func fetchIPOMonth(ym string) (map[string][]map[string]any, error) {
	b, err := httpGetNasdaq("https://api.nasdaq.com/api/ipo/calendar?date=" + ym)
	if err != nil {
		return nil, err
	}
	var r struct {
		Data map[string]json.RawMessage
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	out := map[string][]map[string]any{}
	for _, k := range []string{"priced", "filed", "withdrawn", "upcoming"} {
		raw, ok := r.Data[k]
		if !ok {
			continue
		}
		var t struct {
			Rows          []map[string]any
			UpcomingTable struct{ Rows []map[string]any }
		}
		json.Unmarshal(raw, &t)
		rows := t.Rows
		if len(rows) == 0 {
			rows = t.UpcomingTable.Rows
		}
		out[k] = rows
	}
	return out, nil
}

func str(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func ipoFromRow(m map[string]any, status string) *IPO {
	p := &IPO{ID: str(m, "dealID"), Ticker: strings.ToUpper(str(m, "proposedTickerSymbol")), Name: str(m, "companyName"), Exchange: str(m, "proposedExchange"), Status: status}
	pr := str(m, "proposedSharePrice")
	if strings.Contains(pr, "-") {
		p.PriceRange = pr
		parts := strings.Split(pr, "-")
		p.Price = (pnum(parts[0]) + pnum(parts[len(parts)-1])) / 2
	} else {
		p.Price = pnum(pr)
	}
	p.Shares = pnum(str(m, "sharesOffered"))
	p.Value = pnum(str(m, "dollarValueOfSharesOffered"))
	if p.Value == 0 && p.Price > 0 {
		p.Value = p.Price * p.Shares
	}
	p.Date = pdate(firstNonEmpty(str(m, "pricedDate"), str(m, "expectedPriceDate")))
	p.Filed = pdate(str(m, "filedDate"))
	p.Withdrawn = pdate(str(m, "withdrawDate"))
	if !p.Date.IsZero() && status == "priced" {
		p.Lockup = p.Date.AddDate(0, 0, 180)
	}
	p.Mega = p.Value >= 1e9
	if p.ID == "" {
		p.ID = p.Name + "|" + status
	}
	return p
}

func (a *App) refreshIPOs() {
	now := time.Now()
	months := map[string]bool{}
	for i := -6; i <= 1; i++ {
		months[now.AddDate(0, i, 0).Format("2006-01")] = true
	}
	d := &IPOData{Updated: now}
	seen := map[string]bool{}
	var errs []string
	for ym := range months {
		rows, err := fetchIPOMonth(ym)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		for k, rs := range rows {
			for _, m := range rs {
				p := ipoFromRow(m, k)
				if seen[k+p.ID] {
					continue
				}
				seen[k+p.ID] = true
				switch k {
				case "priced":
					if now.Sub(p.Date) <= 120*24*time.Hour {
						d.Recent = append(d.Recent, p)
					}
				case "upcoming":
					d.Upcoming = append(d.Upcoming, p)
				case "filed":
					if now.Sub(p.Filed) <= 60*24*time.Hour {
						d.Filed = append(d.Filed, p)
					}
				case "withdrawn":
					if now.Sub(p.Withdrawn) <= 45*24*time.Hour {
						d.Withdrawn = append(d.Withdrawn, p)
					}
				}
			}
		}
	}
	if len(errs) == len(months) {
		d.Err = "Couldn't reach the IPO calendar: " + errs[0]
	}
	var syms []string
	for _, p := range d.Recent {
		if p.Ticker != "" {
			syms = append(syms, p.Ticker)
		}
	}
	a.refreshQuotes(syms, 10*time.Minute)
	a.mu.RLock()
	for _, p := range d.Recent {
		q := a.quotes[p.Ticker]
		if q == nil || !q.OK || len(q.Closes) == 0 {
			continue
		}
		p.HasQuote, p.Last, p.DayPct = true, q.Price, q.DayPct
		first := 0
		for i, t := range q.Times {
			if !time.Unix(t, 0).Before(p.Date.Add(-12 * time.Hour)) {
				first = i
				break
			}
		}
		if first < len(q.Opens) {
			p.FirstOpen, p.FirstClose = q.Opens[first], q.Closes[first]
		}
		if p.Price > 0 {
			p.PopPct = (p.FirstClose/p.Price - 1) * 100
			p.SinceIPO = (p.Last/p.Price - 1) * 100
		}
		p.Spark = q.Closes[first:]
	}
	a.mu.RUnlock()
	sort.Slice(d.Recent, func(i, j int) bool { return d.Recent[i].Date.After(d.Recent[j].Date) })
	sort.Slice(d.Upcoming, func(i, j int) bool { return d.Upcoming[i].Date.Before(d.Upcoming[j].Date) })
	sort.Slice(d.Filed, func(i, j int) bool { return d.Filed[i].Filed.After(d.Filed[j].Filed) })
	sort.Slice(d.Withdrawn, func(i, j int) bool { return d.Withdrawn[i].Withdrawn.After(d.Withdrawn[j].Withdrawn) })
	ipoMu.Lock()
	a.ipo = d
	ipoMu.Unlock()
	log.Printf("ipo: %d upcoming, %d recent, %d filed", len(d.Upcoming), len(d.Recent), len(d.Filed))
	a.ipoAlerts(d)
}

func (a *App) ipoAlerts(d *IPOData) {
	a.mu.RLock()
	on := a.store.Settings.NotifyOn && !a.store.Settings.NoIPONotify
	first := len(a.store.Notified) == 0
	a.mu.RUnlock()
	if !on {
		return
	}
	held, watch := a.tracked()
	now := time.Now()
	type note struct{ key, title, body, link string }
	var notes []note
	for _, p := range d.Upcoming {
		if p.Value >= 3e8 || p.Mega {
			when := "soon"
			if !p.Date.IsZero() {
				when = "expected " + p.Date.Format("Mon Jan 2")
			}
			notes = append(notes, note{"ipo|up|" + p.ID, fmt.Sprintf("Upcoming IPO: %s (%s)", p.Name, p.Ticker), fmt.Sprintf("%s, raising about $%s at %s/share.", when, bigMoney(p.Value), firstNonEmpty(p.PriceRange, fmt.Sprintf("$%.2f", p.Price))), "seismo://ipo"})
		}
	}
	for _, p := range d.Recent {
		if p.Mega && now.Sub(p.Date) < 36*time.Hour {
			notes = append(notes, note{"ipo|debut|" + p.ID, fmt.Sprintf("Big IPO debut: %s (%s)", p.Name, p.Ticker), fmt.Sprintf("Priced at $%.2f, raising $%s. Now trading.", p.Price, bigMoney(p.Value)), "seismo://t/" + p.Ticker})
		}
		if (held[p.Ticker] || watch[p.Ticker]) && !p.Lockup.IsZero() {
			days := int(math.Ceil(p.Lockup.Sub(now).Hours() / 24))
			if days >= 0 && days <= 10 {
				notes = append(notes, note{"ipo|lock|" + p.ID, fmt.Sprintf("%s lock-up expires in about %d days", p.Ticker, days), "Early investors and employees can start selling around " + p.Lockup.Format("Jan 2") + ". New stocks often dip near lock-up expiry.", "seismo://t/" + p.Ticker})
			}
		}
	}
	var fire []note
	a.mu.Lock()
	for _, n := range notes {
		if _, ok := a.store.Notified[n.key]; ok {
			continue
		}
		a.store.Notified[n.key] = NotifRec{At: now, Class: "IPO"}
		if first && !strings.HasPrefix(n.key, "ipo|lock") {
			continue
		}
		fire = append(fire, n)
		a.store.Alerts = append([]Alert{{At: now, Ticker: "", Title: n.title, Body: n.body, Level: "info", Key: n.key}}, a.store.Alerts...)
	}
	a.mu.Unlock()
	for i, n := range fire {
		if i >= 3 {
			break
		}
		go toast(n.title, n.body, n.link)
	}
	if len(notes) > 0 {
		a.saveStore()
	}
}

func bigMoney(v float64) string {
	switch {
	case v >= 1e12:
		return fmt.Sprintf("%.2fT", v/1e12)
	case v >= 1e9:
		return fmt.Sprintf("%.1fB", v/1e9)
	case v >= 1e6:
		return fmt.Sprintf("%.0fM", v/1e6)
	}
	return money(v)
}

func (a *App) ipoLoop() {
	time.Sleep(15 * time.Second)
	for {
		a.refreshIPOs()
		time.Sleep(3 * time.Hour)
	}
}

func (a *App) hIPO(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("refresh") == "1" {
		a.refreshIPOs()
	}
	ipoMu.Lock()
	d := a.ipo
	ipoMu.Unlock()
	if d == nil {
		writeJSON(w, map[string]any{"Loading": true})
		return
	}
	a.mu.RLock()
	var news []ArtRef
	for _, x := range a.articles {
		if ipoNewsRe.MatchString(x.Title) {
			news = append(news, ArtRef{x.ID, x.Title, x.URL, x.Source, x.Published, false})
		}
	}
	a.mu.RUnlock()
	sort.Slice(news, func(i, j int) bool { return news[i].Published.After(news[j].Published) })
	seen := map[string]bool{}
	var nn []ArtRef
	for _, x := range news {
		k := articleKey(x.Title)
		if !seen[k] && len(nn) < 25 {
			seen[k] = true
			nn = append(nn, x)
		}
	}
	writeJSON(w, map[string]any{"Data": d, "News": nn})
}

var ipoNewsRe = regexp.MustCompile(`\bIPOs?\b|(?i)\bgoes public\b|\bgoing public\b|\bpublic debut\b|\bfiles to go public\b|\bstock market debut\b|\block-?up expir`)
