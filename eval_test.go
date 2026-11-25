package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestEval(t *testing.T) {
	b, err := os.ReadFile(os.Getenv("SEISMO_CACHE"))
	if err != nil {
		t.Skip("set SEISMO_CACHE to a cache.json to run the detection eval")
	}
	var arts []*Article
	json.Unmarshal(b, &arts)
	an := Analyze(AnalysisInput{Articles: arts, Companies: builtinCompanies(), Quotes: map[string]*Quote{}, Now: time.Now()})
	idx := map[string][]string{}
	for _, c := range append(an.Events, an.Macro...) {
		lbl := c.Ticker + ":" + c.EventID
		if c.Kind == "macro" {
			lbl = "MACRO:" + c.EventID
		}
		for _, id := range c.ArticleIDs {
			idx[id] = append(idx[id], lbl)
		}
	}
	byID := map[string]*Article{}
	for _, a := range arts {
		byID[a.ID] = a
	}
	var ids []string
	for id, l := range idx {
		hasCo := false
		for _, x := range l {
			if !strings.HasPrefix(x, "MACRO") && !strings.HasSuffix(x, "tone_pos") && !strings.HasSuffix(x, "tone_neg") {
				hasCo = true
			}
		}
		if hasCo {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	seed, _ := strconv.Atoi(os.Getenv("SEED"))
	rand.New(rand.NewSource(int64(seed))).Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	fmt.Println("articles with company events:", len(ids))
	for _, id := range ids[:min(110, len(ids))] {
		fmt.Printf("%s\n    -> %v\n", byID[id].Title, idx[id])
	}
	fmt.Println("=== top events")
	for _, c := range an.Events[:25] {
		fmt.Printf("%-6s %-28s %+.2f s%d | %s\n", c.Ticker, c.EventID, c.Impact, c.NSources, c.Articles[0].Title)
	}
}
