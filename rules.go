package main

import "regexp"

type EventRule struct {
	ID          string
	Label       string
	Re          *regexp.Regexp
	Dir         int
	Mag         float64
	Base        string
	Flight      int
	PriceAction bool
	Effect      string
	Not         *regexp.Regexp
	Family      string
}

type MacroRule struct {
	ID      string
	Label   string
	Re      *regexp.Regexp
	Sectors map[string]float64
	Base    string
	Effect  string
}

func er(id, label, pat string, dir int, mag float64, base string, flight int, effect string) *EventRule {
	return &EventRule{ID: id, Label: label, Re: regexp.MustCompile(`(?i)\b(?:` + pat + `)`), Dir: dir, Mag: mag, Base: base, Flight: flight, Effect: effect}
}

var eventRules = []*EventRule{
	er("fraud", "Fraud / accounting probe", `fraud|accounting (?:irregularit|scandal|probe)|restat(?:e|es|ed|ement)s? (?:earnings|results|financials)|cooked the books|ponzi|embezzl|short[- ]seller (?:report|attack)|hindenburg|muddy waters`, -1, 20, "permanent", 3,
		"Credibility shock. If the books are wrong, every valuation model built on them is wrong too, so institutions often sell first and ask questions later. These rarely fully recover; delisting or bankruptcy is possible in bad cases."),
	er("auditor", "Auditor / filing red flag", `auditor (?:resigns|quits|exits)|material weakness|delays? (?:its )?(?:annual report|10-k|10-q|filing)|late filing|going concern`, -1, 12, "permanent", 3,
		"Auditor exits, late filings and going-concern warnings are classic early signals of deeper trouble. Funds with governance mandates often have to sell."),
	er("bankruptcy", "Bankruptcy / default", `bankruptcy|chapter 11|chapter 7|insolven|default(?:s|ed)? on (?:its )?(?:debt|bonds|loan|payment)|(?:into|enters?|faces?) liquidation|liquidator|receivership|restructuring support agreement`, -1, 40, "permanent", 3,
		"Common shareholders sit last in line in bankruptcy and are often wiped out. This is about as permanent as stock damage gets; the stock can stay depressed or go to near zero."),
	er("delist", "Delisting / trading halt", `delist|trading halt|halts trading|suspended from trading|removed from (?:the )?(?:s&p|index)`, -1, 15, "permanent", 3,
		"Delisting or index removal forces index funds to sell and cuts liquidity, which pushes the price down mechanically on top of whatever caused it."),
	er("probe", "Government investigation", `sec (?:probe|investigation|charges|subpoena|sues)|doj (?:probe|investigation|sues)|justice department (?:probe|sues|investigat)|under (?:federal |criminal )?investigation|criminal (?:probe|charges|investigation)|raided|indicted|subpoena`, -1, 7, "permanent", 2,
		"Investigations hang over a stock for months or years: legal costs, possible fines, management distraction, and headline risk every time there's an update. Outcome uncertainty is what scares investors."),
	er("ceo_exit", "CEO / top exec exit", `(?:ceo|chief executive|cfo|chief financial officer|founder|chairman) (?:resigns|steps down|is stepping down|to step down|ousted|fired|departs|exits|quits|leaves)|abrupt(?:ly)? (?:resign|depart|exit)|ousts? (?:its )?(?:ceo|chief)`, -1, 5, "permanent", 2,
		"Sudden leadership exits create strategy uncertainty and often hint at problems not yet public. Planned, orderly successions hurt much less than abrupt ones."),
	er("guidance_cut", "Guidance cut / profit warning", `(?:cuts|lowers|slashes|trims|reduces|withdraws|pulls) (?:its )?(?:full[- ]year |annual |sales |profit |revenue |earnings )?(?:guidance|forecast|outlook)|profit warning|warns (?:on|of) (?:profit|sales|revenue|earnings|weak)|weak (?:guidance|outlook|forecast)`, -1, 8, "permanent", 1,
		"A guidance cut changes the forward earnings everyone values the stock on, so it tends to cause a lasting re-rating rather than a one-day dip. Often followed by analyst downgrades."),
	er("guidance_raise", "Guidance raised", `(?:raises|lifts|boosts|ups|hikes) (?:its )?(?:full[- ]year |annual |sales |profit |revenue |earnings )?(?:guidance|forecast|outlook)|strong (?:guidance|outlook|forecast)`, 1, 5, "permanent", 0,
		"Higher guidance raises the earnings base the stock is valued on, so it usually sticks, especially when paired with an earnings beat. Analysts tend to follow with price-target hikes."),
	er("earnings_beat", "Earnings beat", `beats? (?:wall street |analysts'? |profit |revenue |earnings )?(?:estimates|expectations|forecasts|views)|tops? (?:wall street |analysts'? )?(?:estimates|expectations|forecasts|views)|better[- ]than[- ]expected|record (?:profit|revenue|quarter|sales|earnings)|(?:profit|earnings|revenue|sales) (?:jumps|soars|surges|rises|climbs|beats)`, 1, 4, "scheduled", 0,
		"Earnings dates are known in advance, so the market has already placed its bets. The move is usually fast (1–3 days) and depends on the size of the beat and especially on guidance."),
	er("earnings_miss", "Earnings miss", `miss(?:es|ed)? (?:wall street |analysts'? |profit |revenue |earnings )?(?:estimates|expectations|forecasts|views)|falls? short of|worse[- ]than[- ]expected|(?:profit|earnings|revenue|sales) (?:falls|drops|plunges|slumps|declines|sinks|misses)|disappointing (?:results|quarter|earnings|sales)|(?:quarterly|annual) loss`, -1, 5, "scheduled", 1,
		"A scheduled event the market was waiting on. Misses tend to get punished harder than beats are rewarded. One miss is usually temporary; repeated misses are not."),
	er("dividend_cut", "Dividend cut / suspended", `(?:cuts|slashes|suspends|eliminates|scraps|halts) (?:its )?(?:quarterly )?dividend|dividend (?:cut|suspension)`, -1, 6, "permanent", 2,
		"Income investors own these stocks for the dividend; cutting it pushes them out and signals cash trouble. Typically a lasting hit."),
	er("dividend_up", "Dividend raised", `(?:raises|hikes|boosts|increases) (?:its )?(?:quarterly )?dividend|dividend (?:hike|increase)|special dividend`, 1, 1.5, "temporary", 0,
		"Signals management confidence in cash flow. Modest, steady support rather than a big jump."),
	er("buyback", "Share buyback", `buyback|share repurchase|repurchase (?:program|plan)|buy back \$`, 1, 2, "temporary", 0,
		"Buybacks reduce share count and signal management thinks the stock is cheap. Supportive, usually a mild short-term lift."),
	er("antitrust", "Antitrust / regulatory action", `antitrust|monopol|break(?:ing)? ?up|competition (?:regulator|authority|watchdog|probe)|ftc (?:sues|lawsuit|challenge|probe)|european commission (?:fines|probe|charges)|digital markets act|cma (?:probe|blocks)`, -1, 4, "permanent", 1,
		"Regulatory fights drag on for years. Fines are usually affordable for giants; forced business changes or breakups are the real long-term risk."),
	er("fine", "Fine / lost lawsuit", `ordered to pay|verdict against|jury (?:orders|awards)|loses? (?:lawsuit|case|appeal|patent)|(?:billion|million) (?:fine|penalty|settlement)|fined|record fine|agrees to pay`, -1, 3, "temporary", 1,
		"A known, one-off cost. Once the number is public the uncertainty is gone, so stocks often recover within days unless the ruling changes how the business operates."),
	er("lawsuit", "Lawsuit filed", `lawsuit|sued|sues|class action|litigation`, -1, 1.5, "temporary", 0,
		"Most lawsuits are noise to the stock price. Matters only if the potential damages are large relative to the company or it's part of a pattern."),
	er("ban", "Ban / sanctions / export curbs", `banned|bans? (?:on|its)|sanction(?:s|ed)|blacklist|entity list|export (?:controls|restrictions|curbs|ban)|import ban|forced to divest`, -1, 6, "permanent", 2,
		"Losing access to a market or supplier cuts revenue directly and often for years. Investors worry about further escalation."),
	er("recall", "Recall / safety probe", `recall(?:s|ed|ing)?|safety (?:probe|investigation|warning)|nhtsa|defect`, -1, 3, "temporary", 1,
		"Recalls cost money and brand trust but are usually absorbed within weeks. Becomes serious if injuries/deaths are involved or it reveals a systemic quality problem."),
	er("breach", "Hack / breach / outage", `data breach|hack(?:ed|ers)|cyber ?attack|ransomware|outage|went down|service disruption|systems? (?:failure|crash)`, -1, 2.5, "temporary", 1,
		"Typically a short-lived dip. Lasting damage only if customer data loss triggers regulatory fines, lawsuits or customer churn."),
	er("accident", "Accident / disaster at company", `explosion|fire at|blaze at|fatal|killed in|deaths? (?:linked|at)|oil spill|derail|plant collapse|crash kills|crash of`, -1, 4, "temporary", 1,
		"Operational disasters hit production, liability and reputation. The headline drop is often overdone unless liability is huge (think major spills)."),
	er("fda_win", "Regulatory approval / trial success", `fda (?:approv|clears|grants|accepts)|wins? (?:fda |regulatory )?approval|approval for|breakthrough (?:therapy )?designation|positive (?:trial|results|data|phase)|trial (?:succeeds|meets|met)|met (?:its )?primary endpoint`, 1, 6, "permanent", 0,
		"Approvals turn a pipeline bet into actual sales potential. For smaller drugmakers this can be huge; for giants it's incremental."),
	er("fda_fail", "Rejection / trial failure", `fda (?:rejects|declines|denies|delays)|complete response letter|trial (?:fails|failed|halted|missed)|fail(?:s|ed)? (?:to meet|trial|study)|clinical hold|misses primary endpoint`, -1, 12, "permanent", 2,
		"Failed trials erase the value investors were paying for in the future product. Biotech stocks can lose most of their value on one readout."),
	er("contract", "Big contract / partnership", `wins? (?:a )?(?:\$[\d.]+ ?(?:billion|million|bn|m) )?(?:contract|deal|order)|awarded|landmark deal|partnership with|partners with|strategic alliance|teams up|signs? (?:a )?(?:deal|agreement) with|multi-?year deal`, 1, 2.5, "permanent", 0,
		"New revenue that wasn't in forecasts. Size relative to the company's sales decides whether it moves the needle."),
	er("upgrade", "Analyst upgrade", `upgrade[sd]?|raises? (?:its )?price target|lifts? price target|initiates? (?:with )?(?:a )?buy|outperform rating|overweight rating`, 1, 2, "temporary", 0,
		"Analyst calls move stocks for a day or two; the effect fades unless fundamentals back it up."),
	er("downgrade", "Analyst downgrade", `downgrade[sd]?|cuts? (?:its )?price target|lowers? (?:its )?price target|underperform|underweight|sell rating`, -1, 2.5, "temporary", 0,
		"Short-term pressure as funds that follow that analyst trim positions. Usually fades within days."),
	er("activist", "Big investor buys in", `activist (?:investor|stake|hedge fund)|takes? (?:a )?stake|raises? (?:its )?stake|buys? (?:a )?stake|berkshire (?:buys|stake|takes)|buffett (?:buys|bets)`, 1, 3, "temporary", 0,
		"Big names buying in act as a vote of confidence and can push for changes that unlock value."),
	er("insider_sell", "Insider / major holder selling", `insider (?:selling|sells|sale)|sells? \$?[\d.]+ ?(?:million|billion|bn|m) (?:in|of|worth of) (?:stock|shares)|dumps? (?:shares|stake)|stake sale|cuts? (?:its )?stake|exits? (?:its )?stake`, -1, 1.5, "temporary", 0,
		"Insiders sell for many reasons (taxes, diversification), so it's a weak signal alone. Clusters of selling, or a major fund exiting, matter more."),
	er("supply", "Production / supply problems", `delays? (?:the )?(?:launch|release|production|deliveries)|production (?:halt|halted|problems|issues|cut)|supply (?:shortage|disruption|chain (?:issues|problems))|shortage of|halts production`, -1, 3, "temporary", 0,
		"Lost or delayed sales this quarter. Usually temporary unless it's a sign the product can't be made at scale."),
	er("strike", "Strike / labor action", `strike(?:s)? (?:at|by|against)|workers strike|walkout|work stoppage|union (?:strike|walks out)`, -1, 2, "temporary", 0,
		"Strikes cost output while they last and raise long-term labor costs once settled. Markets usually look through short strikes."),
	er("layoffs", "Layoffs / restructuring", `layoffs?|lay off|job cuts|cut(?:s|ting)? (?:\d[\d,]* )?jobs|slash(?:es)? (?:\d[\d,]* )?jobs|restructuring plan|cost[- ]cutting`, 1, 1.2, "temporary", 0,
		"Wall Street often cheers cost cuts (higher margins), but large cuts can also signal slowing demand. Mild, mixed effect."),
	er("launch", "Product launch / expansion", `unveils|launches|debuts|rolls out|introduces|expands into|opens (?:new )?(?:plant|factory)|new (?:ai )?(?:model|chip|product)`, 1, 1, "temporary", 0,
		"Launches are mostly expected and priced in. Real effect shows up later in sales numbers."),
	er("ai", "AI deal / data-center win", `ai (?:deal|partnership|contract|order|agreement|supply deal)|data[- ]cent(?:er|re) (?:deal|contract|order|agreement)|(?:wins?|lands?|signs?) .{0,30}\bai\b .{0,15}(?:deal|contract|order)`, 1, 2, "temporary", 0,
		"AI-linked headlines have been strong price catalysts. Watch whether it translates into reported revenue; hype-driven moves reverse fast."),
	er("split", "Stock split", `stock split|share split`, 1, 2.5, "temporary", 0,
		"A split doesn't change company value, but it often brings in retail buying. Short-lived boost."),
	er("spinoff", "Spin-off / IPO of unit", `spin[- ]?off|spins off|to separate (?:its )?|ipo of (?:its )?unit|carve[- ]out`, 1, 2, "permanent", 0,
		"Splitting a company can unlock value the market wasn't pricing. Usually positive, sometimes messy."),
	er("credit_down", "Credit rating cut", `credit (?:rating )?(?:downgrade|cut)|cut to junk|junk (?:status|rating)|(?:moody's|fitch|s&p) (?:cuts|downgrades|lowers)`, -1, 4, "permanent", 2,
		"Higher borrowing costs and, at junk level, forced selling by bond funds. A signal of real financial strain."),
	er("record_high", "Stock rallying", `(?:record high|all[- ]time high|soar(?:s|ed|ing)?|surg(?:e|es|ed|ing)|skyrockets?|jump(?:s|ed)?|rall(?:y|ies|ied|ying)|rockets?|climb(?:s|ed)?|gain(?:s|ed)?|ris(?:e|es|ing)|rose|gaps? up|higher|rebound(?:s|ed)?|pops?|spikes?|(?:up|higher) \d+(?:\.\d+)?%)\b`, 1, 1.5, "temporary", 0,
		"Price-action headline. Momentum often continues short term but there's no new information by itself."),
	er("plunge", "Stock selling off", `(?:plung(?:es|ed|e)|plummet(?:s|ed)?|tumbl(?:es|ed|e)|sink(?:s)?|sank|crash(?:es|ed)?|slump(?:s|ed)?|tank(?:s|ed)?|sell-?off|selloff|fall(?:s|ing)?|fell|drop(?:s|ped)?|slid(?:es|e)?|dive(?:s|d)?|declin(?:es|ed)|retreat(?:s|ed)?|dip(?:s|ped)?|sheds?|loses? ground|(?:down|lower) \d+(?:\.\d+)?%|wipes? out \$|rally (?:fades|faded|cools|cooled|stalls|stalled|fizzles|reverses|reversed)|erasing|erases)\b`, -1, 2, "temporary", 0,
		"Price-action headline. Sharp drops sometimes overshoot and bounce; check the underlying cause."),
}

func init() {
	for _, r := range eventRules {
		if r.ID == "record_high" || r.ID == "plunge" {
			r.PriceAction = true
		}
		if r.ID == "upgrade" || r.ID == "downgrade" || r.ID == "record_high" || r.ID == "plunge" {
			r.Not = regexp.MustCompile(`(?i)(?:under|out)performs? (?:the )?(?:market|competitors|monday|tuesday|wednesday|thursday|friday|s&p|dow|nasdaq)|when compared to competitors`)
		}
		if r.ID == "upgrade" {
			r.Not = regexp.MustCompile(`(?i)(?:under|out)performs? (?:the )?(?:market|competitors)|when compared to competitors|an? (?:\S+[- ]){0,2}upgrade(?:\s*$|[,.;:])|upgrades? (?:its|their|your|the) (?:fleet|network|software|app|stores?|plant|factory|systems?|cabin|cabins)|upgrade (?:path|cycle|program|kit|offer)|(?:software|hardware|cabin|seat|free|gas-saving|camera|engine|interior) upgrade`)
		}
		if r.ID == "record_high" {
			r.Not = regexp.MustCompile(`(?i)(?:under|out)performs? (?:the )?(?:market|competitors|monday|tuesday|wednesday|thursday|friday)|when compared to competitors|rally (?:fades|faded|cools|cooled|stalls|stalled|fizzles|reverses|reversed)`)
		}
		switch r.ID {
		case "earnings_beat", "earnings_miss":
			r.Family = "earnings"
		case "guidance_raise", "guidance_cut":
			r.Family = "guidance"
		case "upgrade", "downgrade":
			r.Family = "rating"
		case "record_high", "plunge":
			r.Family = "price"
		case "dividend_up", "dividend_cut":
			r.Family = "dividend"
		case "fda_win", "fda_fail":
			r.Family = "fda"
		}
		if r.ID == "delist" {
			r.Not = regexp.MustCompile(`(?i)\b(?:notes|bonds|debentures|preferred|warrants|etf|etn)\b`)
		}
		if r.ID == "bankruptcy" || r.ID == "fraud" {
			r.Not = regexp.MustCompile(`(?i)\b(?:like enron|not enron|as enron|bitcoin|crypto)\b`)
		}
	}
}

var (
	lawFirmRe   = regexp.MustCompile(`(?i)class action|securities (?:fraud )?(?:lawsuit|litigation)|investors? (?:who|with) (?:lost|losses)|law firm|shareholder alert|investor alert|lead plaintiff|deadline|stock drop notice|rosen|pomerantz|bragar|levi & korsinsky|faruqi|glancy|bronstein|kessler topaz|reminds investors|encourages investors|investigation on behalf`)
	filingRe    = regexp.MustCompile(`(?i)\b(?:acquires? (?:a )?(?:new )?(?:stake|position|shares|[\d,]+ shares)|(?:stock|shares|stake|position|holdings?) (?:acquired|bought|purchased|sold|trimmed|lessened|lowered|raised|boosted|increased|decreased|reduced|cut|grown|lifted|upped) by|(?:buys|sells|purchases|trims|raises|boosts|lowers|cuts|grows|lifts|reduces) (?:its )?(?:new )?(?:stake|position|holdings|shares) in|(?:stake|position|holdings) in .{0,40}(?:inc|corp|ltd|co)\b|[\d,]+ shares of)`)
	filingNegRe = regexp.MustCompile(`(?i)\b(?:sold|sells|trims|trimmed|lessened|lowered|lowers|decreased|reduced|reduces|cuts?)\b`)
	analystCtx  = regexp.MustCompile(`(?i)price target|:\s*(?:bofa|goldman(?: sachs)?|morgan stanley|jpmorgan|citi|ubs|wells fargo|hsbc|bnp(?: paribas)?)\s*$|\btarget\b|rating|upgrade|downgrade|initiates|reiterates|\bsees\b|analyst|\bsays\b|overweight|underweight|outperform|neutral`)
	orderQtyRe  = regexp.MustCompile(`^\s*(?:up to |another |more than |nearly |about )?[\d,.]+\s`)
	instBuy     = &EventRule{ID: "inst_buy", Label: "Funds adding shares (13F filings)", Dir: 1, Mag: 0.4, Base: "temporary", Effect: "Routine institutional filings showing funds adding to positions. Mildly supportive, mostly background noise."}
	instSell    = &EventRule{ID: "inst_sell", Label: "Funds trimming shares (13F filings)", Dir: -1, Mag: 0.4, Base: "temporary", Effect: "Routine institutional filings showing funds reducing positions. Mild, mostly background noise."}
	shSuit      = &EventRule{ID: "sh_suit", Label: "Shareholder class-action ads", Dir: -1, Mag: 0.8, Base: "temporary", Effect: "Law firms file and advertise these after almost any big drop. They follow the price, not lead it, and rarely matter to the stock on their own."}
	orderWin    = &EventRule{ID: "order", Label: "Big customer order", Dir: 1, Mag: 2.5, Base: "permanent", Effect: "A firm order adds to backlog and future revenue. Size relative to annual sales decides how much it matters."}
	opinionRe   = regexp.MustCompile(`(?i)^(?:how|why (?:i|you|we)|what (?:to|the|\S+ means?)|should you|is (?:it|now|this)|is \S+(?: \S+)? (?:stock )?a (?:buy|sell|bargain)|\d+ (?:reasons?|stocks?|things|ways)|better (?:buy|stock)|i |my |here'?s|could|can )|\bprediction\b|\bforecast\b|weekly (?:analysis|outlook)|\bvs\.?\s|millionaire|\bmust[- ]buy|\bto buy (?:now|before|in)|(?:top|best) \d+ |\bbuy the dip\b|technical (?:analysis|structure)|what .{0,40} mean`)
	speculRe    = regexp.MustCompile(`(?i)\b(?:will|could|would|might|may|likely|should|can|or)\b`)
	assetRe     = regexp.MustCompile(`(?i)^.{0,45}\b(?:lot|campus|building|property|properties|land|headquarters|hq|assets?|plant|factory|factories|site|stake|portfolio|stores?|aircraft|jets|planes|ships|bitcoin|btc|shares|tower|office|studio lot|license|rights)\b`)
	sayRe       = regexp.MustCompile(`(?i)\b(?:says?|said|warns?|sees|expects?|predicts?|thinks|believes|according|reports? that|tells|told)\b`)
	segAllowRe  = regexp.MustCompile(`(?i)^(?:stock|stocks|shares?|price|prices|inc\.?|corp\.?|co\.?|corporation|class|cl|a|b|c|the|ltd|plc|adrs?|nyse|nasdaq|nasdaqgs|otc|tsx|lse|its|s|today|premarket|pre-market|pre|market|after-hours|hours|trading|in|early|late|monday|tuesday|wednesday|thursday|friday|holdings|group|stock's|\d[\d.,]*%?|[$€£¥][\d.,]+[bmk]?|(?-i:[A-Z]{1,5}))$`)
	subjHintRe  = regexp.MustCompile(`(?i)^\W{0,4}(?:['’]s\s)?(?:\([A-Z:\s.]+\)\s*)?(?:inc\.?\s|corp\.?\s|group\s)?(?:stock|shares|earnings|profit|revenue|ceo|reports|posts|quarter|q[1-4]|results|is |to (?:pay|cut|lay)|fined|sued|faces|hit with|agrees)`)
	indirectRe  = regexp.MustCompile(`^(?:-(?:owned|backed|linked|related|affiliated|based|style|like)|['’]s? (?:\S+ ){0,3}(?:suppliers?|rivals?|customers?|partners?|bottlers?|biggest \S+ (?:buyer|customer|supplier)|competitors?|peers?))`)
	companyExcl = map[string]*regexp.Regexp{
		"KO":    regexp.MustCompile(`(?i)europacific|bottler|coca-cola consolidated|femsa|coca-cola hbc|coca-cola icecek`),
		"TGT":   regexp.MustCompile(`(?i)price target|target price|targets? (?:of|for)|on target`),
		"MS":    regexp.MustCompile(`(?i)multiple sclerosis`),
		"BRK-B": regexp.MustCompile(`(?i)berkshire hills|berkshire grey`),
		"C":     regexp.MustCompile(`(?i)citizens`),
		"F":     regexp.MustCompile(`(?i)ford foundation|harrison ford|doug ford|tom ford|gerald ford|ford's theatre|fiji`),
		"V":     regexp.MustCompile(`(?i)visa (?:rules|applications?|holders?|policy|program|bans?|waiver|fees?|overstay)|golden visa|student visa|h-1b`),
	}
	victimRe   = regexp.MustCompile(`(?i)(?:spying on|spied on|attacks? on|attacked|hack(?:ed|ing)? (?:of|into)|stole from|stolen from|theft from|scammed|targeting|impersonat\w*|defrauded|on behalf of)\s+(?:\S+\s+)?$`)
	clauseRe   = regexp.MustCompile(`;|\s\|\s|(?i)top (?:midday |morning |evening )?stories:|\bplus\b:?|\balso\b:`)
	analystCos = map[string]bool{"GS": true, "MS": true, "JPM": true, "BAC": true, "C": true, "WFC": true, "UBS": true, "HSBC": true}
)

var (
	mergerTargetRe = regexp.MustCompile(`(?i)\b(?:to be (?:acquired|bought)|acquired by|bought by|takeover (?:bid|offer|approach) for|buyout (?:offer|bid) for|agrees to be (?:bought|acquired)|bid for)\b`)
	mergerRe       = regexp.MustCompile(`(?i)\b(?:to acquire|acquires|acquiring|agrees to (?:buy|acquire)|(?:deal|offer|bid|plans?) to (?:buy|acquire)|acquisition of|takeover of|merger (?:with|of)|merge with|to merge|buyout of|takeover bid|buys [\w .&'-]{2,40} for \$|in talks to (?:buy|acquire))\b`)
	mergerTarget   = &EventRule{ID: "merger_target", Label: "Being acquired (takeover)", Dir: 1, Mag: 12, Base: "permanent", Effect: "Takeover targets jump toward the offer price and stay there unless the deal collapses (regulators, financing, shareholder vote). Biggest risk is the deal breaking."}
	mergerBuyer    = &EventRule{ID: "merger_acquirer", Label: "Acquiring another company", Dir: -1, Mag: 1.5, Base: "temporary", Effect: "Buyers often dip on deal news: they pay a premium, take on debt or dilute shares, and integration is risky. If the deal is seen as strategic it can flip positive."}
	hedgeRe        = regexp.MustCompile(`(?i)\?|\b(?:could|may|might|reportedly|rumou?r|considering|weighs|mulls|explores?|plans? to|said to|in talks|eyes|seeks|would|urges?|calls? for|push(?:es)? for|demands?|pressures?|probably|wants?)\b`)
	negRe          = regexp.MustCompile(`(?i)(?:\bno longer|\bnot|\bno|n't|\bdenies|\bdeny|\brejects|\bdismiss(?:es|ed)?|\bwithout|\bavoids?|\bdrops?|\bends?|\bsettles?|\bcloses|\bconcludes|\bclears|\bwraps up|\bresolves?|\bescapes?|\bavoids?)\s+(?:\S+\s+){0,2}$`)
	posLex         = regexp.MustCompile(`(?i)\b(?:gain|gains|rise|rises|rising|growth|grow|strong|stronger|beat|beats|surge|record|profit|upgrade|bullish|soar|jump|rally|win|wins|expand|boost|optimism|upbeat|robust|outperform|breakthrough|approval|recover|rebound)\b`)
	negLex         = regexp.MustCompile(`(?i)\b(?:fall|falls|drop|drops|decline|declines|loss|losses|weak|weaker|miss|cut|cuts|plunge|bearish|slump|concern|concerns|risk|risks|fear|fears|warn|warns|probe|lawsuit|layoff|crisis|downgrade|struggle|struggles|slowdown|scrutiny|backlash|delay|halt|tumble|threat)\b`)
	weakCtxRe      = regexp.MustCompile(`(?i)\b(?:stock|stocks|shares|company|companies|ceo|earnings|investors?|inc|corp|retailer|firm|revenue|profit|sales|analysts?|nyse|nasdaq|quarter|brand|chain|maker|group|market)\b`)
	tickerRes      = []*regexp.Regexp{
		regexp.MustCompile(`\$([A-Z]{2,5}(?:[.-][A-Z])?)\b`),
		regexp.MustCompile(`\((?:NYSE|NASDAQ|Nasdaq|NYSEARCA|OTC|TSX|LSE)\s?:\s?([A-Z]{1,5}(?:[.-][A-Z])?)\)`),
		regexp.MustCompile(`\(([A-Z]{2,5})\)`),
		regexp.MustCompile(`(?:NYSE|NASDAQ|Nasdaq)\s?:\s?([A-Z]{1,5})\b`),
	}
)

func mr(id, label, pat string, sectors map[string]float64, base, effect string) *MacroRule {
	return &MacroRule{ID: id, Label: label, Re: regexp.MustCompile(`(?i)\b(?:` + pat + `)`), Sectors: sectors, Base: base, Effect: effect}
}

var macroRules = []*MacroRule{
	mr("fed_hike", "Rate hike / hawkish central bank", `rate hike|raises (?:interest )?rates|hikes rates|hawkish|higher for longer`, map[string]float64{"Tech": -1.5, "Semis": -1.5, "RealEstate": -2.5, "Utilities": -1.5, "Consumer": -1, "Finance": 0.8, "broad": -0.6}, "scheduled",
		"Higher rates make future profits worth less today (hurts growth/tech), raise mortgage and borrowing costs (hurts real estate, consumers), but widen bank lending margins."),
	mr("fed_cut", "Rate cut / dovish central bank", `rate cut|cuts (?:interest )?rates|lowers rates|dovish|monetary easing`, map[string]float64{"Tech": 1.5, "Semis": 1.5, "RealEstate": 2.5, "Utilities": 1.2, "Consumer": 1, "Finance": -0.4, "broad": 0.6}, "scheduled",
		"Cheaper money supports growth stocks, housing and consumer spending. Usually telegraphed in advance, so much of the move happens before the announcement."),
	mr("inflation_hot", "Hot inflation", `inflation (?:accelerates|rises|jumps|surges|heats up|hotter|higher than expected|unexpectedly rises)|hot(?:ter)?[- ]than[- ]expected (?:cpi|inflation)|cpi (?:jumps|rises|surges|beats)|prices rise faster`, map[string]float64{"Tech": -1.5, "Semis": -1.2, "RealEstate": -1.5, "Consumer": -1, "Retail": -1, "Energy": 0.5, "broad": -0.8}, "scheduled",
		"Sticky inflation means rates stay high for longer, which pressures valuations and squeezes consumers."),
	mr("inflation_cool", "Cooling inflation", `inflation (?:cools|eases|slows|falls|moderates|lower than expected)|cpi (?:cools|eases|slows|falls)|disinflation`, map[string]float64{"Tech": 1.5, "Semis": 1.2, "RealEstate": 1.5, "Consumer": 1, "Retail": 1, "broad": 0.8}, "scheduled",
		"Cooling inflation opens the door for rate cuts and relieves consumers. Broadly positive, best for rate-sensitive sectors."),
	mr("tariffs", "Tariffs / trade war", `tariffs?|trade war|import (?:duties|taxes)|levies on imports`, map[string]float64{"Retail": -2, "Auto": -2, "Semis": -1.5, "Industrial": -1, "Consumer": -1.5, "Tech": -1, "Materials": -0.5, "broad": -0.5}, "permanent",
		"Tariffs raise costs for importers and companies with overseas supply chains, and invite retaliation against exporters. Policy can change fast, but while in force the costs are real."),
	mr("trade_deal", "Trade deal / tariff relief", `trade (?:deal|truce|agreement|pact)|tariffs? (?:paused|lifted|removed|suspended|rolled back|exemption)|tariff relief`, map[string]float64{"Retail": 1.5, "Auto": 1.5, "Semis": 1.5, "Industrial": 1, "Consumer": 1.2, "Tech": 1, "broad": 0.6}, "temporary",
		"Relief rally for import-dependent and globally exposed companies."),
	mr("war", "War / military escalation", `invasion|invades|airstrikes?|air strikes?|missile (?:strike|attack|barrage)s?|military (?:strike|escalation|offensive)|troops (?:enter|cross|mass)|declares war|war (?:escalates|erupts|breaks out|widens)|drone (?:attack|strike)s?|bombing of`, map[string]float64{"Defense": 3, "Energy": 2, "Airlines": -3, "Travel": -2, "Shipping": 1, "Materials": 0.5, "broad": -0.7}, "temporary",
		"Conflict lifts defense contractors and oil (supply fears) while hitting airlines and travel (fuel costs, demand). Broad markets usually recover from geopolitical shocks within weeks unless the conflict spreads."),
	mr("ceasefire", "Ceasefire / peace talks", `ceasefire|cease-fire|peace (?:deal|talks|agreement|plan)|truce|armistice`, map[string]float64{"Defense": -1.5, "Energy": -1.5, "Airlines": 2, "Travel": 1.5, "broad": 0.5}, "temporary",
		"De-escalation unwinds the war trade: defense and oil give back gains, travel and airlines bounce."),
	mr("oil_up", "Oil price spike", `oil (?:prices? )?(?:surges?|jumps?|spikes?|soars?|rall(?:y|ies)|climbs?)|crude (?:surges|jumps|spikes|soars|rallies)|opec\+? (?:cuts|cut|extends cuts)|output cut|brent (?:tops|jumps|surges)`, map[string]float64{"Energy": 3, "Airlines": -3, "Auto": -1, "Shipping": -1, "Consumer": -0.7, "Travel": -1, "broad": -0.3}, "temporary",
		"Good for oil producers' profits; bad for anyone who burns fuel (airlines, shipping, trucking) and for consumers' wallets."),
	mr("oil_down", "Oil price drop", `oil (?:prices? )?(?:falls?|plunges?|tumbles?|slumps?|drops?|sinks?)|crude (?:falls|plunges|tumbles|slumps|drops)|opec\+? (?:raises|boosts|hikes) (?:output|production)`, map[string]float64{"Energy": -3, "Airlines": 2, "Travel": 1, "Consumer": 0.5}, "temporary",
		"Squeezes oil producers' margins, lowers costs for airlines and transport, and leaves consumers more spending money."),
	mr("chip_curbs", "Chip export controls", `chip (?:export|curbs|ban|restrictions|controls)|semiconductor (?:export|curbs|controls|restrictions|ban)|export (?:curbs|controls) on (?:chips|semiconductors|ai)`, map[string]float64{"Semis": -3, "Tech": -0.8}, "permanent",
		"Cuts chipmakers off from China (a huge market) and can trigger retaliation. Tends to be durable policy."),
	mr("recession", "Recession / slowdown fears", `recession|economic slowdown|economy (?:shrinks|contracts|slows)|gdp (?:contracts|shrinks|falls)|jobless claims (?:jump|rise|surge)|unemployment (?:rises|jumps|climbs)|layoffs surge|hard landing`, map[string]float64{"Consumer": -2, "Retail": -1.5, "Finance": -1.5, "Industrial": -1.5, "Travel": -1.5, "Materials": -1.5, "Auto": -1.5, "Utilities": 0.5, "broad": -1.2}, "permanent",
		"Slowdowns hit cyclical companies (consumer, industrial, banks) hardest. Utilities and staples hold up better."),
	mr("strong_econ", "Strong economic data", `jobs report (?:beats|tops|strong)|(?:strong|robust|blowout) jobs|gdp (?:beats|grows faster|expands|surges)|economy (?:grows|expands|booms|beats)|soft landing`, map[string]float64{"Consumer": 1, "Finance": 1, "Industrial": 1, "broad": 0.6}, "scheduled",
		"Supports earnings across cyclical sectors, though very strong data can also delay rate cuts."),
	mr("pandemic", "Outbreak / pandemic", `pandemic|outbreak|new variant|lockdowns?|epidemic`, map[string]float64{"Airlines": -4, "Travel": -4, "Pharma": 2, "Retail": -1, "Consumer": -1, "broad": -1}, "temporary",
		"Travel and in-person businesses get hit; vaccine/drug makers and delivery/digital names benefit."),
	mr("disaster", "Natural disaster", `hurricane|earthquake|typhoon|cyclone|tsunami|floods?|flooding|wildfires?|heat wave|volcan`, map[string]float64{"Utilities": -1.2, "Travel": -1, "Finance": -0.5, "Materials": 0.5}, "temporary",
		"Insurers and local utilities take losses; rebuilding later helps materials and home-improvement retailers."),
	mr("crypto_up", "Crypto rally", `bitcoin (?:surges|soars|record|rallies|jumps|tops|hits)|crypto (?:rally|surge|boom)|ether (?:surges|soars)`, map[string]float64{"Crypto": 4}, "temporary",
		"Crypto-linked stocks move like leveraged bitcoin."),
	mr("crypto_down", "Crypto selloff", `bitcoin (?:plunges|crashes|tumbles|falls|slumps|sinks|drops)|crypto (?:crash|selloff|sell-off|winter|plunge)`, map[string]float64{"Crypto": -4}, "temporary",
		"Crypto-linked stocks fall harder than bitcoin itself."),
	mr("bank_stress", "Banking stress", `bank (?:run|failure|collapse)|banking crisis|credit crunch|bank (?:fails|collapses)|deposit flight|liquidity crisis`, map[string]float64{"Finance": -4, "RealEstate": -2, "broad": -1.2}, "permanent",
		"Fear spreads across banks quickly; lending tightens, which slows the whole economy."),
	mr("shipping", "Shipping chokepoint disruption", `red sea|suez|strait of hormuz|houthi|shipping (?:disruption|attacks|crisis)|port (?:strike|closure|congestion)|panama canal`, map[string]float64{"Shipping": 2, "Energy": 1.5, "Retail": -1.5, "Industrial": -0.5}, "temporary",
		"Longer routes raise freight rates (good for shippers) and costs/delays for retailers importing goods."),
	mr("ai_bubble", "AI bubble fears", `ai bubble|ai (?:selloff|sell-off|rout)|ai spending (?:fears|concerns|doubts)`, map[string]float64{"Semis": -2.5, "Tech": -1.5}, "temporary",
		"Doubts about AI payback hit the biggest winners of the AI trade hardest."),
	mr("ai_boom", "AI investment boom", `ai (?:boom|spending (?:surge|jumps)|capex (?:surge|jumps|boom)|frenzy)|data cent(?:er|re) boom`, map[string]float64{"Semis": 2, "Tech": 1, "Utilities": 1}, "temporary",
		"More AI spending flows to chipmakers, cloud providers and power producers."),
	mr("shutdown", "Government shutdown / debt fight", `government shutdown|debt ceiling|debt limit|fiscal cliff|sovereign default`, map[string]float64{"Defense": -1, "broad": -0.6}, "temporary",
		"Historically a short-term distraction for markets unless it drags on or threatens default."),
	mr("china_stim", "China stimulus", `china (?:stimulus|eases|cuts rates|unveils measures)|beijing (?:stimulus|support measures)|pboc (?:cuts|eases)`, map[string]float64{"Materials": 2, "Consumer": 0.8, "Auto": 0.8, "Industrial": 0.8}, "temporary",
		"Boosts demand for commodities and benefits companies with big China sales (luxury, autos, chips)."),
	mr("china_slow", "China slowdown", `china (?:slowdown|economy slows|deflation|property crisis)|evergrande|country garden|chinese demand (?:weak|slumps|falls)`, map[string]float64{"Materials": -2, "Consumer": -0.8, "Auto": -1, "Semis": -0.8}, "permanent",
		"Weak Chinese demand hurts miners, luxury brands, automakers and chip suppliers with heavy China exposure."),
}
