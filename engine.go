package main

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

type ArtRef struct {
	ID        string
	Title     string
	URL       string
	Source    string
	Published time.Time
	Hedged    bool
}

type Cluster struct {
	Key        string
	Kind       string
	Ticker     string
	Name       string
	Sector     string
	EventID    string
	Label      string
	Dir        int
	Mag        float64
	Base       string
	Class      string
	Flight     int
	Effect     string
	Note       string
	Sectors    map[string]float64 `json:",omitempty"`
	Articles   []ArtRef
	ArticleIDs []string
	NArticles  int
	NSources   int
	Sources    []string
	FirstSeen  time.Time
	LastSeen   time.Time
	Weight     float64
	Coverage   float64
	Decay      float64
	Impact     float64
	srcSet     map[string]bool
	urlSet     map[string]bool
}

type Driver struct {
	Label  string
	Kind   string
	Class  string
	Impact float64
	Key    string
}

type Signal struct {
	Ticker, Name, Sector string
	HasQuote             bool
	Price, DayPct        float64
	R5, R20, Vol         float64
	Expected, Low, High  float64
	NewsMove, MacroMove  float64
	Momentum             float64
	Outlook              string
	OutlookLvl           int
	Confidence           int
	Flight               int
	FlightLabel          string
	Horizon              string
	Narrative            string
	PricedNote           string
	Drivers              []Driver
	EventKeys            []string
	NSources             int
	NArticles            int
	Held, Watched        bool
	Theories             []Theory
}

type Analysis struct {
	Generated  time.Time
	Signals    []*Signal
	Events     []*Cluster
	Macro      []*Cluster
	SectorHeat map[string]float64
	NewsTone   float64
	NArticles  int
	NCompanies int
	bySig      map[string]*Signal
	byKey      map[string]*Cluster
}

type Override struct {
	NotCo   bool
	Drop    []string
	Relabel string
}

type Learned struct {
	CoNeg map[string][]string
	EvNeg map[string][]string
}

type span struct {
	pos, end int
	indirect bool
}

type coMatcher struct {
	co     *Company
	strong *regexp.Regexp
	weak   *regexp.Regexp
}

type Matcher struct {
	list    []coMatcher
	byTick  map[string]*Company
	learned *Learned
}

var weakBeforeRe = regexp.MustCompile(`(?i)(?:price|stock|share)\s*$`)
var weakAfterRe = regexp.MustCompile(`^\s*(?i:price|date|of\b|for\b|audience|market\b|zone|rate|practice|range|levels?\b)`)

func buildMatcher(cos []*Company, l *Learned) *Matcher {
	m := &Matcher{byTick: map[string]*Company{}, learned: l}
	for _, c := range cos {
		var s, w []string
		for _, a := range c.Aliases {
			if strings.HasPrefix(a, "~") {
				w = append(w, regexp.QuoteMeta(a[1:]))
			} else if a != "" {
				s = append(s, regexp.QuoteMeta(a))
			}
		}
		cm := coMatcher{co: c}
		if len(s) > 0 {
			cm.strong = regexp.MustCompile(`\b(?:` + strings.Join(s, "|") + `)\b`)
		}
		if len(w) > 0 {
			cm.weak = regexp.MustCompile(`\b(?:` + strings.Join(w, "|") + `)\b`)
		}
		m.list = append(m.list, cm)
		m.byTick[c.Ticker] = c
		m.byTick[strings.ReplaceAll(c.Ticker, "-", ".")] = c
	}
	return m
}

func wordsAround(t string, pos, end int) (string, string) {
	before := strings.Fields(strings.ToLower(t[:pos]))
	after := strings.Fields(strings.ToLower(t[end:]))
	b, a := "", ""
	if len(before) > 0 {
		b = strings.Trim(before[len(before)-1], ".,:;!?\"'’()")
	}
	if len(after) > 0 {
		a = strings.Trim(after[0], ".,:;!?\"'’()")
	}
	return b, a
}

func (m *Matcher) learnedReject(t, text string, pos, end int) bool {
	if m.learned == nil || len(m.learned.CoNeg[t]) == 0 {
		return false
	}
	b, a := wordsAround(text, pos, end)
	al := strings.ToLower(text[pos:end])
	for _, p := range m.learned.CoNeg[t] {
		if p == b+" "+al || p == al+" "+a {
			return true
		}
	}
	return false
}

func (m *Matcher) firstValid(re *regexp.Regexp, t, text string, weak bool) (int, int) {
	if re == nil {
		return -1, -1
	}
	for _, loc := range re.FindAllStringIndex(text, -1) {
		if weak && (weakBeforeRe.MatchString(text[:loc[0]]) || weakAfterRe.MatchString(text[loc[1]:])) {
			continue
		}
		if m.learnedReject(t, text, loc[0], loc[1]) {
			continue
		}
		return loc[0], loc[1]
	}
	return -1, -1
}

func (m *Matcher) Find(title, summary string) map[string]span {
	found := map[string]span{}
	ctx := weakCtxRe.MatchString(title + " " + summary)
	for _, cm := range m.list {
		t := cm.co.Ticker
		if ex := companyExcl[t]; ex != nil && ex.MatchString(title) {
			continue
		}
		p, e := m.firstValid(cm.strong, t, title, false)
		if p < 0 && ctx {
			p, e = m.firstValid(cm.weak, t, title, true)
		}
		if p >= 0 {
			found[t] = span{p, e, indirectRe.MatchString(title[e:])}
			continue
		}
		if summary != "" {
			if ps, _ := m.firstValid(cm.strong, t, summary, false); ps >= 0 {
				found[t] = span{10000, 10000, false}
			} else if ctx {
				if pw, _ := m.firstValid(cm.weak, t, summary, true); pw >= 0 {
					found[t] = span{10000, 10000, false}
				}
			}
		}
	}
	for _, re := range tickerRes {
		for _, sm := range re.FindAllStringSubmatchIndex(title, -1) {
			tk := title[sm[2]:sm[3]]
			if c, ok := m.byTick[tk]; ok {
				if _, have := found[c.Ticker]; !have {
					found[c.Ticker] = span{sm[0], sm[1], false}
				}
			}
		}
	}
	return found
}

type evMatch struct {
	r   *EventRule
	pos int
	end int
}

func evLearnedReject(l *Learned, id, text string, pos, end int) bool {
	if l == nil || len(l.EvNeg[id]) == 0 {
		return false
	}
	low := strings.ToLower(text)
	for _, p := range l.EvNeg[id] {
		if strings.Contains(low, p) {
			return true
		}
	}
	return false
}

func matchEvents(text string, l *Learned) []evMatch {
	if filingRe.MatchString(text) {
		r := instBuy
		if filingNegRe.MatchString(text) {
			r = instSell
		}
		if !evLearnedReject(l, r.ID, text, 0, 0) {
			return []evMatch{{r, 0, 0}}
		}
	}
	if lawFirmRe.MatchString(text) && !evLearnedReject(l, shSuit.ID, text, 0, 0) {
		return []evMatch{{shSuit, 0, 0}}
	}
	var out []evMatch
	for _, r := range eventRules {
		var loc []int
		for _, c := range r.Re.FindAllStringIndex(text, -1) {
			if !negRe.MatchString(text[:c[0]]) {
				loc = c
				break
			}
		}
		if loc == nil || (r.Not != nil && r.Not.MatchString(text)) {
			continue
		}
		rr := r
		if r.ID == "fraud" && lawFirmRe.MatchString(text) {
			rr = shSuit
		}
		if evLearnedReject(l, rr.ID, text, loc[0], loc[1]) {
			continue
		}
		out = append(out, evMatch{rr, loc[0], loc[1]})
	}
	if loc := mergerTargetRe.FindStringIndex(text); loc != nil {
		out = append(out, evMatch{mergerTarget, loc[0], loc[1]})
	} else if loc := mergerRe.FindStringIndex(text); loc != nil {
		out = append(out, evMatch{nil, loc[0], loc[1]})
	}
	fam := map[string]int{}
	for i, e := range out {
		if e.r == nil || e.r.Family == "" {
			continue
		}
		if j, ok := fam[e.r.Family]; ok {
			if e.pos < out[j].pos {
				out[j].r = nil
				out[j].pos = -1
				fam[e.r.Family] = i
			} else {
				out[i].pos = -1
			}
		} else {
			fam[e.r.Family] = i
		}
	}
	f := out[:0]
	for _, e := range out {
		if e.pos >= 0 {
			f = append(f, e)
		}
	}
	out = f
	sort.SliceStable(out, func(i, j int) bool { return evMag(out[i]) > evMag(out[j]) })
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

func evMag(e evMatch) float64 {
	if e.r == nil {
		return 5
	}
	return e.r.Mag
}

func lexScore(t string) int {
	return len(posLex.FindAllString(t, -1)) - len(negLex.FindAllString(t, -1))
}

var sepRe = regexp.MustCompile(`[,&]|\band\b`)
var specStartRe = regexp.MustCompile(`(?i)^(?:is|are|will|could|can|should|would|has|have)\b`)

func sameClause(title string, a, b int) bool {
	if a > b {
		a, b = b, a
	}
	if a < 0 || b > len(title) {
		return true
	}
	return !clauseRe.MatchString(title[a:b])
}

func subjectSegOK(title string, from, to int, others []span) bool {
	if to < from || to-from > 48 {
		return false
	}
	seg := []byte(title[from:to])
	for _, o := range others {
		if o.pos >= from && o.end <= to {
			for i := o.pos; i < o.end; i++ {
				seg[i-from] = ' '
			}
		}
	}
	str := sepRe.ReplaceAllString(string(seg), " ")
	for _, w := range strings.FieldsFunc(str, func(r rune) bool {
		return r == ' ' || r == '(' || r == ')' || r == ':' || r == '-' || r == '—' || r == '\'' || r == '’'
	}) {
		if !segAllowRe.MatchString(strings.Trim(w, ".,")) {
			return false
		}
	}
	return true
}

type assign struct {
	t string
	r *EventRule
	w float64
}

func attribute(e evMatch, title string, spans map[string]span, titleCos []string, forced []string) []assign {
	var out []assign
	if len(titleCos) == 0 {
		if e.r == nil || e.r == mergerTarget || e.r.PriceAction {
			return nil
		}
		for _, t := range forced {
			out = append(out, assign{t, e.r, 0.75})
		}
		return out
	}
	if e.r == nil || e.r == mergerTarget {
		if e.r == nil && assetRe.MatchString(title[e.end:]) {
			return nil
		}
		for _, t := range titleCos {
			sp := spans[t]
			if !sameClause(title, sp.pos, e.pos) {
				continue
			}
			switch {
			case e.r == mergerTarget && sp.pos < e.pos:
				out = append(out, assign{t, mergerTarget, 1})
			case e.r == mergerTarget:
				out = append(out, assign{t, mergerBuyer, 1})
			case sp.pos < e.pos:
				out = append(out, assign{t, mergerBuyer, 1})
			case orderQtyRe.MatchString(title[e.end:]):
				out = append(out, assign{t, orderWin, 1})
			default:
				out = append(out, assign{t, mergerTarget, 1})
			}
		}
		return out
	}
	var all []span
	for _, t := range titleCos {
		all = append(all, spans[t])
	}
	if e.r.PriceAction {
		if strings.Contains(strings.ToLower(title), "prediction") {
			return nil
		}
		best, bestEnd := "", -1
		for _, t := range titleCos {
			sp := spans[t]
			if sp.indirect || sp.end > e.pos || !sameClause(title, sp.pos, e.pos) {
				continue
			}
			if subjectSegOK(title, sp.end, e.pos, all) && sp.end > bestEnd {
				best, bestEnd = t, sp.end
			}
		}
		if best != "" {
			out = append(out, assign{best, e.r, 1})
		}
		return out
	}
	var cands []string
	for _, t := range titleCos {
		sp := spans[t]
		if analystCos[t] && (e.r.Family == "rating" || analystCtx.MatchString(title)) && !subjHintRe.MatchString(title[sp.end:]) {
			continue
		}
		if !sameClause(title, sp.pos, e.pos) {
			continue
		}
		if (e.r.ID == "probe" || e.r.ID == "fraud" || e.r.ID == "lawsuit" || e.r.ID == "fine") && victimRe.MatchString(title[:sp.pos]) {
			continue
		}
		cands = append(cands, t)
	}
	if len(cands) == 0 {
		return nil
	}
	primary := ""
	bestEnd := -1
	for _, t := range cands {
		sp := spans[t]
		if sp.end <= e.pos && sp.end > bestEnd {
			primary, bestEnd = t, sp.end
		}
	}
	if primary == "" {
		bestPos := 1 << 30
		for _, t := range cands {
			sp := spans[t]
			if sp.pos >= e.end && sp.pos-e.end < 70 && sp.pos < bestPos {
				primary, bestPos = t, sp.pos
			}
		}
	}
	for _, t := range cands {
		sp := spans[t]
		w := 0.3
		if t == primary {
			w = 1
			if sp.end <= e.pos && sayRe.MatchString(title[sp.end:e.pos]) {
				w = 0.6
			}
		}
		if sp.indirect {
			w *= 0.5
		}
		if len(cands) > 3 && t != primary {
			continue
		}
		out = append(out, assign{t, e.r, w})
	}
	return out
}

var ruleByID = func() map[string]*EventRule {
	m := map[string]*EventRule{}
	for _, r := range eventRules {
		m[r.ID] = r
	}
	for _, r := range []*EventRule{mergerTarget, mergerBuyer, instBuy, instSell, shSuit, orderWin} {
		m[r.ID] = r
	}
	return m
}()

type AnalysisInput struct {
	Articles  []*Article
	Companies []*Company
	Quotes    map[string]*Quote
	Held      map[string]bool
	Watch     map[string]bool
	Overrides map[string]Override
	Learned   *Learned
	Now       time.Time
}

func Analyze(in AnalysisInput) *Analysis {
	now := in.Now
	m := buildMatcher(in.Companies, in.Learned)
	clusters := map[string]*Cluster{}

	addTo := func(key string, mk func() *Cluster, a *Article, w float64, hedged bool) {
		c := clusters[key]
		if c == nil {
			c = mk()
			c.Key = key
			c.srcSet = map[string]bool{}
			c.urlSet = map[string]bool{}
			c.FirstSeen = a.Published
			c.LastSeen = a.Published
			clusters[key] = c
		}
		if c.urlSet[a.ID] {
			return
		}
		c.urlSet[a.ID] = true
		c.Weight += w
		c.NArticles++
		c.ArticleIDs = append(c.ArticleIDs, a.ID)
		c.srcSet[strings.ToLower(a.Source)] = true
		if a.Published.Before(c.FirstSeen) {
			c.FirstSeen = a.Published
		}
		if a.Published.After(c.LastSeen) {
			c.LastSeen = a.Published
		}
		c.Articles = append(c.Articles, ArtRef{a.ID, a.Title, a.URL, a.Source, a.Published, hedged})
	}
	coCluster := func(t string, co *Company, r *EventRule) (string, func() *Cluster) {
		return t + "|" + r.ID, func() *Cluster {
			return &Cluster{Kind: "company", Ticker: t, Name: co.Name, Sector: co.Sector, EventID: r.ID, Label: r.Label, Dir: r.Dir, Mag: r.Mag, Base: r.Base, Flight: r.Flight, Effect: r.Effect}
		}
	}

	for _, a := range in.Articles {
		title := a.Title
		opinion := opinionRe.MatchString(title)
		specQ := strings.Contains(title, "?") && (speculRe.MatchString(title) || specStartRe.MatchString(title))
		hedged := hedgeRe.MatchString(title) || opinion
		w := a.Cred
		if hedged {
			w *= 0.6
		}
		if opinion {
			w *= 0.6
		}
		for _, r := range macroRules {
			loc := r.Re.FindStringIndex(title)
			if loc == nil || negRe.MatchString(title[:loc[0]]) || specQ {
				continue
			}
			if ov, ok := in.Overrides[a.ID+"|macro"]; ok && contains(ov.Drop, r.ID) {
				continue
			}
			if evLearnedReject(in.Learned, "macro:"+r.ID, title, loc[0], loc[1]) {
				continue
			}
			rr := r
			addTo("macro|"+r.ID, func() *Cluster {
				return &Cluster{Kind: "macro", EventID: rr.ID, Label: rr.Label, Base: rr.Base, Effect: rr.Effect, Sectors: rr.Sectors, Mag: 1}
			}, a, w, hedged)
		}

		spans := m.Find(title, a.Summary)
		for _, t := range a.Tickers {
			if _, ok := spans[t]; !ok {
				spans[t] = span{10001, 10001, false}
			}
		}
		for t := range spans {
			if ov, ok := in.Overrides[a.ID+"|"+t]; ok && ov.NotCo {
				delete(spans, t)
			}
		}
		if len(spans) == 0 {
			continue
		}
		var titleCos, forced []string
		for t, sp := range spans {
			if sp.pos < 10000 {
				titleCos = append(titleCos, t)
			} else {
				forced = append(forced, t)
			}
		}
		sort.Strings(titleCos)
		sort.Strings(forced)
		if len(titleCos) > 6 {
			continue
		}
		evs := matchEvents(title, in.Learned)
		evW := 1.0
		if len(evs) == 0 && a.Summary != "" && len(titleCos) > 0 {
			evs = matchEvents(a.Summary, in.Learned)
			evW = 0.5
			for i := range evs {
				evs[i].pos, evs[i].end = 0, 0
			}
		}
		got := map[string]bool{}
		for _, e := range evs {
			if specQ && (e.r == nil || e.r.PriceAction || e.r.Mag >= 8) {
				continue
			}
			if opinion && e.r != nil && e.r.Mag >= 8 {
				continue
			}
			var as []assign
			if evW < 1 {
				if e.r == nil || e.r == mergerTarget || e.r.PriceAction {
					continue
				}
				for _, t := range titleCos {
					as = append(as, assign{t, e.r, 0.6})
				}
			} else {
				as = attribute(e, title, spans, titleCos, forced)
			}
			for _, x := range as {
				ov := in.Overrides[a.ID+"|"+x.t]
				if contains(ov.Drop, x.r.ID) || ov.Relabel != "" {
					continue
				}
				co := m.byTick[x.t]
				if co == nil {
					continue
				}
				k, mk := coCluster(x.t, co, x.r)
				addTo(k, mk, a, w*evW*x.w, hedged)
				got[x.t] = true
			}
		}
		for t := range spans {
			ov, ok := in.Overrides[a.ID+"|"+t]
			if !ok || ov.Relabel == "" {
				continue
			}
			if r := ruleByID[ov.Relabel]; r != nil {
				if co := m.byTick[t]; co != nil {
					k, mk := coCluster(t, co, r)
					addTo(k, mk, a, a.Cred, false)
				}
			}
			got[t] = true
		}
		if len(evs) > 0 {
			continue
		}
		cands := titleCos
		if len(cands) == 0 {
			cands = forced
		}
		for _, t := range cands {
			if got[t] || len(cands) > 3 {
				continue
			}
			if ov := in.Overrides[a.ID+"|"+t]; contains(ov.Drop, "tone_pos") || contains(ov.Drop, "tone_neg") {
				continue
			}
			co := m.byTick[t]
			if co == nil {
				continue
			}
			s := lexScore(title + " " + a.Summary)
			if s == 0 {
				continue
			}
			dir, id, lbl := 1, "tone_pos", "Positive coverage"
			if s < 0 {
				dir, id, lbl = -1, "tone_neg", "Negative coverage"
			}
			mag := 0.6 + 0.3*math.Min(math.Abs(float64(s)), 3)
			tw := w * 0.6 / math.Sqrt(float64(len(cands)))
			addTo(t+"|"+id, func() *Cluster {
				return &Cluster{Kind: "company", Ticker: t, Name: co.Name, Sector: co.Sector, EventID: id, Label: lbl, Dir: dir, Mag: mag, Base: "temporary",
					Effect: "General tone of coverage without a specific identifiable event. Weak signal on its own; matters when it piles up."}
			}, a, tw, hedged)
		}
	}

	an := &Analysis{Generated: now, SectorHeat: map[string]float64{}, bySig: map[string]*Signal{}, byKey: map[string]*Cluster{}, NArticles: len(in.Articles)}
	for _, c := range clusters {
		finalizeCluster(c, now)
		an.byKey[c.Key] = c
		if c.Kind == "macro" {
			if c.Coverage > 0.12 {
				an.Macro = append(an.Macro, c)
			}
		} else {
			an.Events = append(an.Events, c)
		}
	}
	sort.Slice(an.Macro, func(i, j int) bool {
		return an.Macro[i].Coverage*an.Macro[i].Decay > an.Macro[j].Coverage*an.Macro[j].Decay
	})
	sort.Slice(an.Events, func(i, j int) bool { return math.Abs(an.Events[i].Impact) > math.Abs(an.Events[j].Impact) })

	for _, mc := range an.Macro {
		for s, v := range mc.Sectors {
			an.SectorHeat[s] += v * mc.Coverage * mc.Decay * macroPriced(mc)
		}
	}

	byTicker := map[string][]*Cluster{}
	for _, c := range an.Events {
		byTicker[c.Ticker] = append(byTicker[c.Ticker], c)
	}
	tickers := map[string]bool{}
	for t := range byTicker {
		tickers[t] = true
	}
	for t := range in.Held {
		tickers[t] = true
	}
	for t := range in.Watch {
		tickers[t] = true
	}
	var tone, toneW float64
	for t := range tickers {
		co := m.byTick[t]
		if co == nil {
			co = &Company{Ticker: t, Name: t, Sector: "Other"}
		}
		s := buildSignal(co, byTicker[t], an, in.Quotes[t])
		s.Held, s.Watched = in.Held[t], in.Watch[t]
		an.Signals = append(an.Signals, s)
		an.bySig[t] = s
		tone += s.NewsMove
		toneW++
	}
	if toneW > 0 {
		an.NewsTone = tone / toneW
	}
	an.NCompanies = len(byTicker)
	sort.Slice(an.Signals, func(i, j int) bool { return math.Abs(an.Signals[i].Expected) > math.Abs(an.Signals[j].Expected) })
	for _, c := range an.Events {
		trimCluster(c)
	}
	for _, c := range an.Macro {
		trimCluster(c)
	}
	return an
}

func trimCluster(c *Cluster) {
	sort.Slice(c.Articles, func(i, j int) bool { return c.Articles[i].Published.After(c.Articles[j].Published) })
	if len(c.Articles) > 14 {
		c.Articles = c.Articles[:14]
	}
}

func finalizeCluster(c *Cluster, now time.Time) {
	c.NSources = len(c.srcSet)
	for s := range c.srcSet {
		c.Sources = append(c.Sources, s)
		if len(c.Sources) >= 12 {
			break
		}
	}
	age := now.Sub(c.FirstSeen).Hours()
	since := math.Max(0, now.Sub(c.LastSeen).Hours())
	switch c.Base {
	case "permanent":
		c.Class = "PERMANENT"
		if c.NSources >= 6 && age >= 72 {
			c.Note = "Widely known for a few days now. The initial drop/jump has likely happened, but a lasting overhang or re-rating can continue."
		}
	case "scheduled":
		c.Class = "WELL-KNOWN"
		c.Note = "A scheduled, anticipated event. The market had positioned for it, so the reaction is quick and mostly done within days."
	default:
		if c.NSources >= 4 && age >= 24 {
			c.Class = "WELL-KNOWN"
			c.Note = fmt.Sprintf("Covered by %d outlets over %.0f hours. Most of the price reaction has likely already happened.", c.NSources, age)
		} else {
			c.Class = "TEMPORARY"
		}
	}
	if c.Kind == "macro" && c.Class == "TEMPORARY" && c.NSources >= 8 {
		c.Class = "WELL-KNOWN"
	}
	hl := 2.0
	if c.Class == "WELL-KNOWN" {
		hl = 3
	} else if c.Class == "PERMANENT" {
		hl = 12
	}
	c.Decay = math.Pow(0.5, since/(hl*24))
	c.Coverage = 1 - math.Exp(-c.Weight/(1.2*(1+c.Mag/12)))
	if c.Mag >= 8 && c.NSources < 3 {
		c.Coverage *= 0.5
	}
	pf := 1.0
	if c.Class == "WELL-KNOWN" {
		pf = 0.45
	} else if c.Note != "" {
		pf = 0.7
	}
	c.Impact = float64(c.Dir) * c.Mag * c.Coverage * c.Decay * pf
}

func macroPriced(c *Cluster) float64 {
	if c.Class == "WELL-KNOWN" {
		return 0.45
	}
	return 1
}

func buildSignal(co *Company, cs []*Cluster, an *Analysis, q *Quote) *Signal {
	s := &Signal{Ticker: co.Ticker, Name: co.Name, Sector: co.Sector, Vol: 2}
	if q != nil && q.OK {
		s.HasQuote, s.Price, s.DayPct, s.R5, s.R20 = true, q.Price, q.DayPct, q.R5, q.R20
		if q.Vol > 0 {
			s.Vol = q.Vol
		}
		if q.Name != "" && s.Name == s.Ticker {
			s.Name = q.Name
		}
	}
	var pos, neg float64
	srcs := map[string]bool{}
	var ups, dns []float64
	for _, c := range cs {
		if c.Impact > 0 {
			ups = append(ups, c.Impact)
		} else {
			dns = append(dns, c.Impact)
		}
		if c.Impact > 0 {
			pos += c.Impact
		} else {
			neg -= c.Impact
		}
		for k := range c.srcSet {
			srcs[k] = true
		}
		s.NArticles += c.NArticles
		s.EventKeys = append(s.EventKeys, c.Key)
		if c.Dir < 0 && c.Decay > 0.25 && c.Coverage > 0.3 && c.Flight > s.Flight {
			s.Flight = c.Flight
		}
		if math.Abs(c.Impact) >= 0.1 {
			s.Drivers = append(s.Drivers, Driver{Label: c.Label, Kind: "news", Class: c.Class, Impact: c.Impact, Key: c.Key})
		}
	}
	s.NSources = len(srcs)
	s.NewsMove = stackImpacts(ups) + stackImpacts(dns)
	s.NewsMove = 30 * math.Tanh(s.NewsMove/30)

	for _, mc := range an.Macro {
		v := mc.Sectors[co.Sector] + mc.Sectors["broad"]
		if v == 0 {
			continue
		}
		imp := v * mc.Coverage * mc.Decay * macroPriced(mc) * 0.6
		s.MacroMove += imp
		if math.Abs(imp) >= 0.12 {
			s.Drivers = append(s.Drivers, Driver{Label: mc.Label, Kind: "macro", Class: mc.Class, Impact: imp, Key: mc.Key})
		}
	}
	s.MacroMove = 6 * math.Tanh(s.MacroMove/6)

	if s.HasQuote {
		s.Momentum = clamp(0.1*s.R5+0.04*s.R20, -2.5, 2.5)
		if math.Abs(s.Momentum) >= 0.2 {
			s.Drivers = append(s.Drivers, Driver{Label: fmt.Sprintf("Price trend (5d %+.1f%%, 1mo %+.1f%%)", s.R5, s.R20), Kind: "trend", Class: "", Impact: s.Momentum})
		}
		if s.NewsMove < -1 && s.R5 < 0 && -s.R5 >= -s.NewsMove*0.8 {
			s.NewsMove *= 0.6
			s.PricedNote = fmt.Sprintf("The stock already fell %.1f%% this week, so a good part of the bad news is probably priced in.", -s.R5)
		} else if s.NewsMove > 1 && s.R5 > 0 && s.R5 >= s.NewsMove*0.8 {
			s.NewsMove *= 0.6
			s.PricedNote = fmt.Sprintf("The stock already rose %.1f%% this week, so a good part of the good news is probably priced in.", s.R5)
		}
	}
	sort.Slice(s.Drivers, func(i, j int) bool { return math.Abs(s.Drivers[i].Impact) > math.Abs(s.Drivers[j].Impact) })
	if len(s.Drivers) > 8 {
		s.Drivers = s.Drivers[:8]
	}

	s.Expected = s.NewsMove + s.MacroMove + s.Momentum
	band := math.Max(0.8, s.Vol*math.Sqrt(5)*0.6) + math.Abs(s.Expected)*0.3
	s.Low, s.High = s.Expected-band, s.Expected+band

	switch e := s.Expected; {
	case e >= 4:
		s.Outlook, s.OutlookLvl = "STRONG UP", 2
	case e >= 1.2:
		s.Outlook, s.OutlookLvl = "UP", 1
	case e <= -4:
		s.Outlook, s.OutlookLvl = "STRONG DOWN", -2
	case e <= -1.2:
		s.Outlook, s.OutlookLvl = "DOWN", -1
	default:
		s.Outlook, s.OutlookLvl = "FLAT / MIXED", 0
	}
	if s.Expected <= -8 && s.Flight < 2 {
		s.Flight = 2
	}
	s.FlightLabel = []string{"Low", "Elevated", "High", "Severe"}[s.Flight]

	agree := 0.3
	if pos+neg > 0 {
		agree = math.Abs(pos-neg) / (pos + neg)
	}
	conf := 100 * (0.25 + 0.75*agree) * (1 - math.Exp(-float64(s.NSources)/5))
	if s.HasQuote && s.Momentum*s.NewsMove > 0 {
		conf += 8
	}
	if len(cs) == 0 {
		conf = 15
	}
	s.Confidence = int(clamp(conf, 5, 95))

	s.Horizon = "next few days to 2 weeks"
	if len(s.Drivers) > 0 && s.Drivers[0].Class == "PERMANENT" {
		s.Horizon = "next several weeks to months"
	}
	s.Narrative = narrative(s, an)
	s.Theories = computeTheories(q, s)
	return s
}

func narrative(s *Signal, an *Analysis) string {
	var b strings.Builder
	verb := map[int]string{2: "is likely to rise sharply", 1: "is leaning up", 0: "has no strong push either way", -1: "is leaning down", -2: "is likely to fall sharply"}[s.OutlookLvl]
	fmt.Fprintf(&b, "%s %s. Model expects %+.1f%% (range %+.1f%% to %+.1f%%) over the %s, confidence %d%%.", s.Name, verb, s.Expected, s.Low, s.High, s.Horizon, s.Confidence)
	if len(s.Drivers) > 0 {
		d := s.Drivers[0]
		fmt.Fprintf(&b, " Biggest driver: %s", d.Label)
		if d.Class != "" {
			fmt.Fprintf(&b, " [%s]", d.Class)
		}
		fmt.Fprintf(&b, " (%+.1f%%).", d.Impact)
		if c := an.byKey[d.Key]; c != nil && c.Effect != "" {
			b.WriteString(" " + c.Effect)
		}
		if len(s.Drivers) > 1 {
			var o []string
			for _, d := range s.Drivers[1:min(4, len(s.Drivers))] {
				o = append(o, fmt.Sprintf("%s %+.1f%%", d.Label, d.Impact))
			}
			b.WriteString(" Also in play: " + strings.Join(o, "; ") + ".")
		}
	} else {
		b.WriteString(" No significant news found for it right now.")
	}
	if s.PricedNote != "" {
		b.WriteString(" " + s.PricedNote)
	}
	switch s.Flight {
	case 3:
		b.WriteString(" Investor-flight risk is SEVERE: this is the kind of news that makes funds dump the stock.")
	case 2:
		b.WriteString(" Investor-flight risk is HIGH: some investors are likely to bail.")
	case 1:
		b.WriteString(" Investor-flight risk is elevated but not alarming.")
	}
	return b.String()
}

func stackImpacts(x []float64) float64 {
	sort.Slice(x, func(i, j int) bool { return math.Abs(x[i]) > math.Abs(x[j]) })
	w, t := 1.0, 0.0
	for _, v := range x {
		t += v * w
		w *= 0.6
		if w < 0.2 {
			w = 0.2
		}
	}
	return t
}

func clamp(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }
func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
