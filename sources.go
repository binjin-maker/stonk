package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Article struct {
	ID        string
	Title     string
	Summary   string
	URL       string
	Source    string
	Feed      string
	Published time.Time
	Seen      time.Time
	Cred      float64
	Tickers   []string
}

type Feed struct {
	Name   string
	Group  string
	URL    string
	Kind   string
	Ticker string
}

type SourceStat struct {
	Name  string
	Group string
	Count int
	OK    bool
	Err   string
	Ms    int64
	Last  time.Time
}

var httpc = &http.Client{Timeout: 20 * time.Second}

const ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"

func httpGet(u string) ([]byte, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "*/*")
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 12<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return b, nil
}

func httpPost(u, body string) {
	req, err := http.NewRequest("POST", u, strings.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req); err == nil {
		resp.Body.Close()
	}
}

func gnSearch(q string) string {
	return "https://news.google.com/rss/search?q=" + url.QueryEscape(q) + "&hl=en-US&gl=US&ceid=US:en"
}

func gnTopic(topic, hl, gl string) string {
	return fmt.Sprintf("https://news.google.com/rss/headlines/section/topic/%s?hl=%s&gl=%s&ceid=%s:en", topic, hl, gl, gl)
}

func baseFeeds(cos []*Company) []Feed {
	var f []Feed
	add := func(name, group, u string) { f = append(f, Feed{Name: name, Group: group, URL: u, Kind: "rss"}) }

	for _, ed := range [][2]string{{"en-US", "US"}, {"en-GB", "GB"}, {"en-IN", "IN"}, {"en-AU", "AU"}, {"en-CA", "CA"}, {"en-SG", "SG"}, {"en-ZA", "ZA"}, {"en-NG", "NG"}, {"en-IE", "IE"}} {
		add("Google Business "+ed[1], "Google News (world editions)", gnTopic("BUSINESS", ed[0], ed[1]))
	}
	add("Google World US", "Google News (world editions)", gnTopic("WORLD", "en-US", "US"))
	add("Google World GB", "Google News (world editions)", gnTopic("WORLD", "en-GB", "GB"))
	add("Google World IN", "Google News (world editions)", gnTopic("WORLD", "en-IN", "IN"))
	add("Google Tech US", "Google News (world editions)", gnTopic("TECHNOLOGY", "en-US", "US"))

	for _, q := range []string{
		"shares plunge", "shares soar", "stock tumbles", "stock surges", "profit warning",
		"SEC investigation company", "DOJ probe company", "class action lawsuit shares", "product recall shares",
		"antitrust regulators company", "files for bankruptcy", "CEO steps down", "FDA approval shares",
		"acquisition deal billion", "takeover bid", "tariffs stocks", "sanctions companies", "export controls chips",
		"data breach company shares", "analyst downgrade", "analyst upgrade", "guidance cut", "raises guidance",
		"earnings beat estimates", "earnings miss estimates", "dividend cut", "accounting irregularities",
		"short seller report", "credit rating downgrade company", "factory fire explosion company",
		"Federal Reserve interest rates", "inflation report markets", "OPEC oil prices", "stock market today",
		"European stocks", "Asian markets stocks", "China economy stocks", "emerging markets stocks",
		"IPO priced", "IPO debut shares", "files for IPO", "IPO lock-up expiration", "upcoming IPOs",
	} {
		add("Google: "+q, "Google News (event searches)", gnSearch(q+" when:2d"))
	}

	var batch []string
	flush := func() {
		if len(batch) == 0 {
			return
		}
		add("Google: "+strings.Join(batch, ", "), "Google News (company sweeps)", gnSearch("("+strings.Join(quoteAll(batch), " OR ")+") stock when:2d"))
		batch = nil
	}
	for _, c := range cos {
		batch = append(batch, primaryAlias(c))
		if len(batch) == 6 {
			flush()
		}
	}
	flush()

	for _, s := range [][3]string{
		{"Yahoo Finance", "Major financial press", "https://finance.yahoo.com/news/rssindex"},
		{"CNBC Top News", "Major financial press", "https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=100003114"},
		{"CNBC World", "Major financial press", "https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=100727362"},
		{"CNBC Earnings", "Major financial press", "https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=15839135"},
		{"CNBC Tech", "Major financial press", "https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=19854910"},
		{"CNBC Finance", "Major financial press", "https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=10000664"},
		{"MarketWatch", "Major financial press", "https://feeds.content.dowjones.io/public/rss/mw_topstories"},
		{"MarketWatch Pulse", "Major financial press", "https://feeds.content.dowjones.io/public/rss/mw_marketpulse"},
		{"WSJ Markets", "Major financial press", "https://feeds.a.dj.com/rss/RSSMarketsMain.xml"},
		{"WSJ Business", "Major financial press", "https://feeds.a.dj.com/rss/WSJcomUSBusiness.xml"},
		{"NYT Business", "Major financial press", "https://rss.nytimes.com/services/xml/rss/nyt/Business.xml"},
		{"Seeking Alpha Currents", "Major financial press", "https://seekingalpha.com/market_currents.xml"},
		{"Investing.com", "Major financial press", "https://www.investing.com/rss/news.rss"},
		{"Business Insider Markets", "Major financial press", "https://markets.businessinsider.com/rss/news"},
		{"Benzinga", "Major financial press", "https://www.benzinga.com/feed"},
		{"BBC Business", "International", "https://feeds.bbci.co.uk/news/business/rss.xml"},
		{"BBC World", "International", "https://feeds.bbci.co.uk/news/world/rss.xml"},
		{"Guardian Business", "International", "https://www.theguardian.com/uk/business/rss"},
		{"Al Jazeera", "International", "https://www.aljazeera.com/xml/rss/all.xml"},
		{"DW Business", "International", "https://rss.dw.com/rdf/rss-en-bus"},
		{"Nikkei Asia", "International", "https://asia.nikkei.com/rss/feed/nar"},
		{"SCMP Business", "International", "https://www.scmp.com/rss/92/feed"},
		{"Economic Times Markets", "International", "https://economictimes.indiatimes.com/markets/rssfeeds/1977021501.cms"},
		{"TechCrunch", "Sector press", "https://techcrunch.com/feed/"},
		{"Fierce Pharma", "Sector press", "https://www.fiercepharma.com/rss/xml"},
		{"OilPrice", "Sector press", "https://oilprice.com/rss/main"},
		{"CoinDesk", "Sector press", "https://www.coindesk.com/arc/outboundfeeds/rss/"},
		{"Defense News", "Sector press", "https://www.defensenews.com/arc/outboundfeeds/rss/"},
		{"r/stocks", "Retail investors (Reddit)", "https://www.reddit.com/r/stocks/.rss"},
		{"r/investing", "Retail investors (Reddit)", "https://www.reddit.com/r/investing/.rss"},
		{"r/wallstreetbets", "Retail investors (Reddit)", "https://www.reddit.com/r/wallstreetbets/.rss"},
		{"r/StockMarket", "Retail investors (Reddit)", "https://www.reddit.com/r/StockMarket/.rss"},
		{"r/worldnews", "Retail investors (Reddit)", "https://www.reddit.com/r/worldnews/.rss"},
		{"r/economics", "Retail investors (Reddit)", "https://www.reddit.com/r/economics/.rss"},
	} {
		add(s[0], s[1], s[2])
	}
	return f
}

func trackedFeeds(c *Company) []Feed {
	n := primaryAlias(c)
	return []Feed{
		{Name: "Google: " + c.Ticker, Group: "Tracked companies", Kind: "rss", Ticker: c.Ticker, URL: gnSearch(`"` + n + `" when:7d`)},
		{Name: "Google: " + c.Ticker + " stock", Group: "Tracked companies", Kind: "rss", Ticker: c.Ticker, URL: gnSearch(`"` + n + `" stock OR shares when:7d`)},
		{Name: "Yahoo: " + c.Ticker, Group: "Tracked companies", Kind: "rss", Ticker: c.Ticker, URL: "https://feeds.finance.yahoo.com/rss/2.0/headline?s=" + url.QueryEscape(c.Ticker) + "&region=US&lang=en-US"},
		{Name: "Yahoo news: " + c.Ticker, Group: "Tracked companies", Kind: "yahoo", Ticker: c.Ticker, URL: "https://query2.finance.yahoo.com/v1/finance/search?q=" + url.QueryEscape(c.Ticker) + "&quotesCount=0&newsCount=25"},
	}
}

var gdeltQueries = []string{
	`(shares OR stock) (plunge OR plunges OR soar OR soars OR tumble OR surge) sourcelang:english`,
	`(lawsuit OR investigation OR fined OR recall OR bankruptcy) (company OR shares) sourcelang:english`,
	`(sanctions OR tariffs OR invasion OR airstrike OR embargo) (markets OR stocks OR oil) sourcelang:english`,
	`(earnings OR profit OR revenue) (beats OR misses OR warns) sourcelang:english`,
}

func gdeltURL(q string) string {
	return "https://api.gdeltproject.org/api/v2/doc/doc?query=" + url.QueryEscape(q) + "&mode=artlist&maxrecords=250&timespan=1d&format=json&sort=datedesc"
}

func quoteAll(xs []string) []string {
	o := make([]string, len(xs))
	for i, x := range xs {
		o[i] = `"` + x + `"`
	}
	return o
}

func primaryAlias(c *Company) string {
	for _, a := range c.Aliases {
		a = strings.TrimPrefix(a, "~")
		if len(a) > 2 {
			return a
		}
	}
	return c.Name
}

type xLink struct {
	Href string `xml:"href,attr"`
	Text string `xml:",chardata"`
}
type xItem struct {
	Title       string  `xml:"title"`
	Links       []xLink `xml:"link"`
	GUID        string  `xml:"guid"`
	PubDate     string  `xml:"pubDate"`
	Date        string  `xml:"date"`
	Updated     string  `xml:"updated"`
	Published   string  `xml:"published"`
	Description string  `xml:"description"`
	Summary     string  `xml:"summary"`
	Source      string  `xml:"source"`
}

var tagRe = regexp.MustCompile(`<[^>]*>`)
var wsRe = regexp.MustCompile(`\s+`)

func cleanText(s string, max int) string {
	s = html.UnescapeString(tagRe.ReplaceAllString(html.UnescapeString(s), " "))
	s = strings.TrimSpace(wsRe.ReplaceAllString(s, " "))
	if len(s) > max {
		s = s[:max]
	}
	return s
}

func parseFeed(b []byte, feed Feed) []*Article {
	dec := xml.NewDecoder(strings.NewReader(string(b)))
	dec.Strict = false
	dec.Entity = xml.HTMLEntity
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	var out []*Article
	now := time.Now()
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || (se.Name.Local != "item" && se.Name.Local != "entry") {
			continue
		}
		var it xItem
		if dec.DecodeElement(&it, &se) != nil {
			continue
		}
		title := cleanText(it.Title, 300)
		if title == "" {
			continue
		}
		link := ""
		for _, l := range it.Links {
			if l.Href != "" {
				link = l.Href
				break
			}
			if strings.TrimSpace(l.Text) != "" {
				link = strings.TrimSpace(l.Text)
				break
			}
		}
		if link == "" && strings.HasPrefix(it.GUID, "http") {
			link = it.GUID
		}
		src := cleanText(it.Source, 80)
		if strings.Contains(feed.URL, "news.google.com") {
			if i := strings.LastIndex(title, " - "); i > 0 {
				if src == "" {
					src = strings.TrimSpace(title[i+3:])
				}
				title = strings.TrimSpace(title[:i])
			}
		}
		if src == "" {
			src = feed.Name
		}
		sum := it.Description
		if sum == "" {
			sum = it.Summary
		}
		sum = cleanText(sum, 400)
		if strings.HasPrefix(sum, title) || strings.Contains(feed.URL, "news.google.com") {
			sum = ""
		}
		ds := firstNonEmpty(it.PubDate, it.Published, it.Updated, it.Date)
		a := &Article{Title: title, Summary: sum, URL: link, Source: src, Feed: feed.Name, Published: parseDate(ds, now), Seen: now}
		if feed.Ticker != "" {
			a.Tickers = []string{feed.Ticker}
		}
		out = append(out, a)
	}
	return out
}

func parseYahooNews(b []byte, feed Feed) []*Article {
	var r struct {
		News []struct {
			Title               string
			Publisher           string
			Link                string
			ProviderPublishTime int64
			RelatedTickers      []string
		}
	}
	if json.Unmarshal(b, &r) != nil {
		return nil
	}
	now := time.Now()
	var out []*Article
	for _, n := range r.News {
		if n.Title == "" {
			continue
		}
		a := &Article{Title: cleanText(n.Title, 300), URL: n.Link, Source: n.Publisher, Feed: feed.Name, Published: time.Unix(n.ProviderPublishTime, 0), Seen: now}
		if a.Published.IsZero() || n.ProviderPublishTime == 0 {
			a.Published = now
		}
		for _, t := range n.RelatedTickers {
			if strings.EqualFold(t, feed.Ticker) {
				a.Tickers = []string{feed.Ticker}
			}
		}
		out = append(out, a)
	}
	return out
}

func parseGdelt(b []byte, name string) []*Article {
	var r struct {
		Articles []struct {
			URL           string
			Title         string
			Seendate      string
			Domain        string
			Sourcecountry string
		}
	}
	if json.Unmarshal(b, &r) != nil {
		return nil
	}
	now := time.Now()
	var out []*Article
	for _, a := range r.Articles {
		t, err := time.Parse("20060102T150405Z", a.Seendate)
		if err != nil {
			t = now
		}
		src := a.Domain
		if a.Sourcecountry != "" {
			src += " (" + a.Sourcecountry + ")"
		}
		out = append(out, &Article{Title: cleanText(a.Title, 300), URL: a.URL, Source: src, Feed: name, Published: t, Seen: now})
	}
	return out
}

var dateFormats = []string{time.RFC1123Z, time.RFC1123, time.RFC3339, time.RFC3339Nano, "Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST", "Mon, 02 Jan 2006 15:04:05 Z", "2 Jan 2006 15:04:05 -0700", "2006-01-02T15:04:05-0700", "2006-01-02 15:04:05", "Mon, 2 Jan 2006 15:04 -0700", "Mon, 02 Jan 06 15:04:05 -0700", "2006-01-02T15:04:05"}

func parseDate(s string, now time.Time) time.Time {
	s = strings.TrimSpace(s)
	for _, f := range dateFormats {
		if t, err := time.Parse(f, s); err == nil {
			if t.After(now.Add(time.Hour)) {
				return now
			}
			return t
		}
	}
	return now
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func articleKey(title string) string {
	k := nonAlnum.ReplaceAllString(strings.ToLower(title), "")
	if len(k) > 90 {
		k = k[:90]
	}
	h := sha1.Sum([]byte(k))
	return hex.EncodeToString(h[:8])
}

var credTable = []struct {
	k string
	v float64
}{
	{"reuters", 1}, {"bloomberg", 1}, {"wall street journal", 1}, {"wsj", 1}, {"financial times", 1}, {"ft.com", 1},
	{"associated press", .95}, {"ap news", .95}, {"cnbc", .95}, {"nikkei", .95}, {"economist", .95}, {"new york times", .95}, {"nyt", .95},
	{"barron", .9}, {"marketwatch", .9}, {"bbc", .9}, {"yahoo", .85}, {"cnn", .85}, {"guardian", .85}, {"south china morning", .85}, {"scmp", .85},
	{"al jazeera", .8}, {"economic times", .8}, {"fierce", .8}, {"defense news", .8}, {"washington post", .9}, {"axios", .85}, {"politico", .8},
	{"forbes", .75}, {"business insider", .75}, {"techcrunch", .75}, {"fortune", .8}, {"the verge", .7}, {"coindesk", .7}, {"investing.com", .7},
	{"seeking alpha", .65}, {"benzinga", .65}, {"oilprice", .65}, {"thestreet", .6}, {"nasdaq", .6}, {"investopedia", .6},
	{"motley fool", .45}, {"fool.com", .45}, {"zacks", .5}, {"insidermonkey", .4}, {"r/", .35}, {"reddit", .35},
}

func credFor(src, feed string) float64 {
	s := strings.ToLower(src + " " + feed)
	for _, c := range credTable {
		if strings.Contains(s, c.k) {
			return c.v
		}
	}
	if strings.Contains(s, "gdelt") {
		return .55
	}
	return .6
}
