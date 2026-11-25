package main

import (
	"net/http"
	"strings"
)

var commonNeighbor = map[string]bool{"stock": true, "stocks": true, "shares": true, "share": true, "s": true, "inc": true, "inc.": true, "corp": true, "ceo": true, "and": true, "the": true, "a": true, "of": true, "to": true, "in": true, "on": true, "for": true, "as": true, "is": true, "says": true, "said": true, "after": true, "with": true, "by": true, "": true, "price": false}

type RuleInfo struct {
	ID, Label string
	Dir       int
}

func ruleCatalog() []RuleInfo {
	var out []RuleInfo
	for _, r := range eventRules {
		out = append(out, RuleInfo{r.ID, r.Label, r.Dir})
	}
	for _, r := range []*EventRule{mergerTarget, mergerBuyer, orderWin, shSuit, instBuy, instSell} {
		out = append(out, RuleInfo{r.ID, r.Label, r.Dir})
	}
	return out
}

func addUnique(xs []string, s string) []string {
	if s == "" || contains(xs, s) {
		return xs
	}
	return append(xs, s)
}

func (a *App) learnFromArticle(title, ticker, eventID, kind, action string) {
	l := &a.store.Learned
	if l.CoNeg == nil {
		l.CoNeg = map[string][]string{}
	}
	if l.EvNeg == nil {
		l.EvNeg = map[string][]string{}
	}
	switch action {
	case "notco":
		m := buildMatcher(a.companiesLocked(), nil)
		sp, ok := m.Find(title, "")[ticker]
		if !ok || sp.pos >= 10000 {
			return
		}
		b, af := wordsAround(title, sp.pos, sp.end)
		al := strings.ToLower(title[sp.pos:sp.end])
		if !commonNeighbor[b] {
			l.CoNeg[ticker] = addUnique(l.CoNeg[ticker], b+" "+al)
		}
		if !commonNeighbor[af] {
			l.CoNeg[ticker] = addUnique(l.CoNeg[ticker], al+" "+af)
		}
	case "drop":
		var loc []int
		key := eventID
		if kind == "macro" {
			key = "macro:" + eventID
			for _, r := range macroRules {
				if r.ID == eventID {
					loc = r.Re.FindStringIndex(title)
				}
			}
		} else if r := ruleByID[eventID]; r != nil && r.Re != nil {
			loc = r.Re.FindStringIndex(title)
		}
		if loc == nil {
			return
		}
		phrase := strings.ToLower(strings.TrimSpace(title[loc[0]:loc[1]]))
		rest := strings.Fields(title[loc[1]:])
		if len(rest) > 0 && strings.ContainsAny(rest[0], "0123456789$%") {
			rest = nil
		}
		if len(rest) > 0 {
			phrase += " " + strings.ToLower(strings.Trim(rest[0], ".,:;!?\"'’()"))
		} else if pre := strings.Fields(title[:loc[0]]); len(pre) > 0 {
			phrase = strings.ToLower(strings.Trim(pre[len(pre)-1], ".,:;!?\"'’()")) + " " + phrase
		}
		if len(phrase) >= 6 {
			l.EvNeg[key] = addUnique(l.EvNeg[key], phrase)
		}
	}
}

func (a *App) companiesLocked() []*Company {
	cos := builtinCompanies()
	have := map[string]bool{}
	for _, c := range cos {
		have[c.Ticker] = true
	}
	for _, c := range a.store.Extra {
		if !have[c.Ticker] {
			cos = append(cos, c)
		}
	}
	return cos
}

func (a *App) hFeedback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Ticker, EventID, Kind, Action, Relabel string
		ArticleIDs                             []string
	}
	if readJSON(r, &req) != nil || req.Action == "" {
		http.Error(w, "bad request", 400)
		return
	}
	a.mu.Lock()
	if req.Action == "reset" {
		a.store.Overrides = map[string]Override{}
		a.store.Learned = Learned{}
		a.mu.Unlock()
		a.saveStore()
		go a.analyze(false)
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
	ids := req.ArticleIDs
	if len(ids) == 0 && a.an != nil {
		key := req.Ticker + "|" + req.EventID
		if req.Kind == "macro" {
			key = "macro|" + req.EventID
		}
		if c := a.an.byKey[key]; c != nil {
			ids = c.ArticleIDs
		}
	}
	if a.store.Overrides == nil {
		a.store.Overrides = map[string]Override{}
	}
	learned := 0
	for i, id := range ids {
		k := id + "|" + req.Ticker
		if req.Kind == "macro" {
			k = id + "|macro"
		}
		ov := a.store.Overrides[k]
		switch req.Action {
		case "notco":
			ov.NotCo = true
		case "drop":
			ov.Drop = addUnique(ov.Drop, req.EventID)
		case "relabel":
			ov.Drop = addUnique(ov.Drop, req.EventID)
			ov.Relabel = req.Relabel
		}
		a.store.Overrides[k] = ov
		if art := a.articles[id]; art != nil && i < 25 && req.Action != "relabel" {
			a.learnFromArticle(art.Title, req.Ticker, req.EventID, req.Kind, req.Action)
			learned++
		}
	}
	a.mu.Unlock()
	a.saveStore()
	a.analyze(false)
	writeJSON(w, map[string]any{"ok": true, "articles": len(ids)})
}
