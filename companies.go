package main

type Company struct {
	Ticker   string
	Name     string
	Sector   string
	Industry string
	Aliases  []string
}

// aliases prefixed with "~" are ambiguous words and need finance context to count
var builtinRaw = []struct {
	T, N, S string
	A       []string
}{
	{"AAPL", "Apple", "Tech", []string{"Apple", "iPhone", "Tim Cook"}},
	{"MSFT", "Microsoft", "Tech", []string{"Microsoft", "Satya Nadella", "Azure"}},
	{"GOOGL", "Alphabet (Google)", "Tech", []string{"Alphabet", "Google", "YouTube", "Sundar Pichai", "Waymo"}},
	{"AMZN", "Amazon", "Tech", []string{"~Amazon", "Amazon.com", "AWS", "Andy Jassy"}},
	{"META", "Meta Platforms", "Tech", []string{"Meta Platforms", "~Meta", "Facebook", "Instagram", "WhatsApp", "Zuckerberg"}},
	{"NFLX", "Netflix", "Media", []string{"Netflix"}},
	{"TSLA", "Tesla", "Auto", []string{"Tesla", "Cybertruck"}},
	{"ORCL", "Oracle", "Tech", []string{"~Oracle", "Larry Ellison"}},
	{"CRM", "Salesforce", "Tech", []string{"Salesforce"}},
	{"ADBE", "Adobe", "Tech", []string{"Adobe"}},
	{"IBM", "IBM", "Tech", []string{"IBM"}},
	{"CSCO", "Cisco", "Tech", []string{"Cisco"}},
	{"UBER", "Uber", "Tech", []string{"Uber"}},
	{"PLTR", "Palantir", "Tech", []string{"Palantir"}},
	{"SHOP", "Shopify", "Tech", []string{"Shopify"}},
	{"SNOW", "Snowflake", "Tech", []string{"~Snowflake"}},
	{"BABA", "Alibaba", "Tech", []string{"Alibaba"}},
	{"TCEHY", "Tencent", "Tech", []string{"Tencent"}},
	{"BIDU", "Baidu", "Tech", []string{"Baidu"}},
	{"PDD", "PDD Holdings (Temu)", "Retail", []string{"PDD Holdings", "Temu", "Pinduoduo"}},
	{"JD", "JD.com", "Retail", []string{"JD.com"}},
	{"SONY", "Sony", "Tech", []string{"Sony"}},
	{"005930.KS", "Samsung Electronics", "Semis", []string{"Samsung"}},
	{"SAP", "SAP", "Tech", []string{"SAP"}},
	{"ASML", "ASML", "Semis", []string{"ASML"}},
	{"NVDA", "Nvidia", "Semis", []string{"Nvidia", "NVIDIA", "Jensen Huang"}},
	{"AMD", "AMD", "Semis", []string{"AMD", "Advanced Micro Devices", "Lisa Su"}},
	{"INTC", "Intel", "Semis", []string{"~Intel"}},
	{"TSM", "TSMC", "Semis", []string{"TSMC", "Taiwan Semiconductor"}},
	{"AVGO", "Broadcom", "Semis", []string{"Broadcom"}},
	{"QCOM", "Qualcomm", "Semis", []string{"Qualcomm"}},
	{"MU", "Micron", "Semis", []string{"Micron"}},
	{"ARM", "Arm Holdings", "Semis", []string{"Arm Holdings"}},
	{"TXN", "Texas Instruments", "Semis", []string{"Texas Instruments"}},
	{"AMAT", "Applied Materials", "Semis", []string{"Applied Materials"}},
	{"LRCX", "Lam Research", "Semis", []string{"Lam Research"}},
	{"SMCI", "Super Micro Computer", "Semis", []string{"Super Micro", "Supermicro"}},
	{"F", "Ford", "Auto", []string{"~Ford", "Ford Motor"}},
	{"GM", "General Motors", "Auto", []string{"General Motors", "GM"}},
	{"TM", "Toyota", "Auto", []string{"Toyota"}},
	{"STLA", "Stellantis", "Auto", []string{"Stellantis", "Chrysler", "~Jeep"}},
	{"RIVN", "Rivian", "Auto", []string{"Rivian"}},
	{"LCID", "Lucid Group", "Auto", []string{"Lucid Motors", "Lucid Group"}},
	{"NIO", "NIO", "Auto", []string{"NIO"}},
	{"BYDDY", "BYD", "Auto", []string{"BYD"}},
	{"HMC", "Honda", "Auto", []string{"Honda"}},
	{"VWAGY", "Volkswagen", "Auto", []string{"Volkswagen", "VW"}},
	{"RACE", "Ferrari", "Auto", []string{"~Ferrari"}},
	{"XOM", "ExxonMobil", "Energy", []string{"Exxon", "ExxonMobil", "Exxon Mobil"}},
	{"CVX", "Chevron", "Energy", []string{"Chevron"}},
	{"SHEL", "Shell", "Energy", []string{"~Shell"}},
	{"BP", "BP", "Energy", []string{"BP"}},
	{"COP", "ConocoPhillips", "Energy", []string{"ConocoPhillips"}},
	{"TTE", "TotalEnergies", "Energy", []string{"TotalEnergies"}},
	{"OXY", "Occidental", "Energy", []string{"Occidental Petroleum", "~Occidental"}},
	{"SLB", "SLB (Schlumberger)", "Energy", []string{"SLB", "Schlumberger"}},
	{"2222.SR", "Saudi Aramco", "Energy", []string{"Aramco"}},
	{"JPM", "JPMorgan Chase", "Finance", []string{"JPMorgan", "JP Morgan", "Jamie Dimon"}},
	{"BAC", "Bank of America", "Finance", []string{"Bank of America", "BofA"}},
	{"WFC", "Wells Fargo", "Finance", []string{"Wells Fargo"}},
	{"C", "Citigroup", "Finance", []string{"Citigroup", "Citi"}},
	{"GS", "Goldman Sachs", "Finance", []string{"Goldman Sachs", "~Goldman"}},
	{"MS", "Morgan Stanley", "Finance", []string{"Morgan Stanley"}},
	{"BRK-B", "Berkshire Hathaway", "Finance", []string{"Berkshire", "Warren Buffett", "Buffett"}},
	{"V", "Visa", "Finance", []string{"~Visa"}},
	{"MA", "Mastercard", "Finance", []string{"Mastercard"}},
	{"AXP", "American Express", "Finance", []string{"American Express", "Amex"}},
	{"BLK", "BlackRock", "Finance", []string{"BlackRock"}},
	{"SCHW", "Charles Schwab", "Finance", []string{"Schwab"}},
	{"HSBC", "HSBC", "Finance", []string{"HSBC"}},
	{"UBS", "UBS", "Finance", []string{"UBS"}},
	{"PYPL", "PayPal", "Finance", []string{"PayPal"}},
	{"COIN", "Coinbase", "Crypto", []string{"Coinbase"}},
	{"HOOD", "Robinhood", "Finance", []string{"Robinhood"}},
	{"MSTR", "Strategy (MicroStrategy)", "Crypto", []string{"MicroStrategy", "Michael Saylor"}},
	{"LLY", "Eli Lilly", "Pharma", []string{"Eli Lilly", "~Lilly", "Mounjaro", "Zepbound"}},
	{"NVO", "Novo Nordisk", "Pharma", []string{"Novo Nordisk", "Ozempic", "Wegovy"}},
	{"PFE", "Pfizer", "Pharma", []string{"Pfizer"}},
	{"JNJ", "Johnson & Johnson", "Pharma", []string{"Johnson & Johnson", "J&J"}},
	{"MRK", "Merck", "Pharma", []string{"~Merck"}},
	{"ABBV", "AbbVie", "Pharma", []string{"AbbVie"}},
	{"MRNA", "Moderna", "Pharma", []string{"Moderna"}},
	{"AZN", "AstraZeneca", "Pharma", []string{"AstraZeneca"}},
	{"BMY", "Bristol Myers Squibb", "Pharma", []string{"Bristol Myers", "Bristol-Myers"}},
	{"GILD", "Gilead", "Pharma", []string{"Gilead"}},
	{"AMGN", "Amgen", "Pharma", []string{"Amgen"}},
	{"UNH", "UnitedHealth", "Health", []string{"UnitedHealth", "UnitedHealthcare"}},
	{"CVS", "CVS Health", "Health", []string{"CVS"}},
	{"ISRG", "Intuitive Surgical", "Health", []string{"Intuitive Surgical"}},
	{"WMT", "Walmart", "Retail", []string{"Walmart"}},
	{"COST", "Costco", "Retail", []string{"Costco"}},
	{"TGT", "Target", "Retail", []string{"~Target"}},
	{"HD", "Home Depot", "Retail", []string{"Home Depot"}},
	{"LOW", "Lowe's", "Retail", []string{"Lowe's"}},
	{"NKE", "Nike", "Consumer", []string{"Nike"}},
	{"SBUX", "Starbucks", "Consumer", []string{"Starbucks"}},
	{"MCD", "McDonald's", "Consumer", []string{"McDonald's"}},
	{"KO", "Coca-Cola", "Consumer", []string{"Coca-Cola"}},
	{"PEP", "PepsiCo", "Consumer", []string{"PepsiCo", "Pepsi"}},
	{"PG", "Procter & Gamble", "Consumer", []string{"Procter & Gamble", "P&G"}},
	{"DIS", "Disney", "Media", []string{"Disney"}},
	{"LVMUY", "LVMH", "Consumer", []string{"LVMH", "Louis Vuitton"}},
	{"CMG", "Chipotle", "Consumer", []string{"Chipotle"}},
	{"LULU", "Lululemon", "Consumer", []string{"Lululemon"}},
	{"BUD", "AB InBev", "Consumer", []string{"AB InBev", "Anheuser-Busch", "Budweiser"}},
	{"NSRGY", "Nestle", "Consumer", []string{"Nestle", "Nestlé"}},
	{"BA", "Boeing", "Industrial", []string{"Boeing"}},
	{"CAT", "Caterpillar", "Industrial", []string{"~Caterpillar"}},
	{"DE", "Deere", "Industrial", []string{"John Deere", "Deere"}},
	{"GE", "GE Aerospace", "Industrial", []string{"GE Aerospace", "General Electric"}},
	{"HON", "Honeywell", "Industrial", []string{"Honeywell"}},
	{"UPS", "UPS", "Industrial", []string{"UPS"}},
	{"FDX", "FedEx", "Industrial", []string{"FedEx"}},
	{"UNP", "Union Pacific", "Industrial", []string{"Union Pacific"}},
	{"MMM", "3M", "Industrial", []string{"3M"}},
	{"EADSY", "Airbus", "Industrial", []string{"Airbus"}},
	{"LMT", "Lockheed Martin", "Defense", []string{"Lockheed"}},
	{"RTX", "RTX (Raytheon)", "Defense", []string{"RTX", "Raytheon"}},
	{"NOC", "Northrop Grumman", "Defense", []string{"Northrop"}},
	{"GD", "General Dynamics", "Defense", []string{"General Dynamics"}},
	{"DAL", "Delta Air Lines", "Airlines", []string{"Delta Air Lines", "~Delta"}},
	{"UAL", "United Airlines", "Airlines", []string{"United Airlines"}},
	{"AAL", "American Airlines", "Airlines", []string{"American Airlines"}},
	{"LUV", "Southwest Airlines", "Airlines", []string{"Southwest Airlines"}},
	{"BKNG", "Booking Holdings", "Travel", []string{"Booking Holdings", "Booking.com"}},
	{"ABNB", "Airbnb", "Travel", []string{"Airbnb"}},
	{"MAR", "Marriott", "Travel", []string{"Marriott"}},
	{"CCL", "Carnival", "Travel", []string{"~Carnival"}},
	{"RCL", "Royal Caribbean", "Travel", []string{"Royal Caribbean"}},
	{"T", "AT&T", "Telecom", []string{"AT&T"}},
	{"VZ", "Verizon", "Telecom", []string{"Verizon"}},
	{"TMUS", "T-Mobile", "Telecom", []string{"T-Mobile"}},
	{"CMCSA", "Comcast", "Media", []string{"Comcast", "NBCUniversal"}},
	{"WBD", "Warner Bros. Discovery", "Media", []string{"Warner Bros"}},
	{"SPOT", "Spotify", "Media", []string{"Spotify"}},
	{"FCX", "Freeport-McMoRan", "Materials", []string{"Freeport-McMoRan"}},
	{"NEM", "Newmont", "Materials", []string{"Newmont"}},
	{"BHP", "BHP", "Materials", []string{"BHP"}},
	{"RIO", "Rio Tinto", "Materials", []string{"Rio Tinto"}},
	{"NUE", "Nucor", "Materials", []string{"Nucor"}},
	{"ALB", "Albemarle", "Materials", []string{"Albemarle"}},
	{"LIN", "Linde", "Materials", []string{"Linde"}},
	{"NEE", "NextEra Energy", "Utilities", []string{"NextEra"}},
	{"DUK", "Duke Energy", "Utilities", []string{"Duke Energy"}},
	{"CEG", "Constellation Energy", "Utilities", []string{"Constellation Energy"}},
	{"VST", "Vistra", "Utilities", []string{"Vistra"}},
	{"PCG", "PG&E", "Utilities", []string{"PG&E"}},
	{"PLD", "Prologis", "RealEstate", []string{"Prologis"}},
	{"AMT", "American Tower", "RealEstate", []string{"American Tower"}},
	{"ZIM", "ZIM Shipping", "Shipping", []string{"ZIM Integrated"}},
	{"AMKBY", "Maersk", "Shipping", []string{"Maersk"}},
}

func builtinCompanies() []*Company {
	out := make([]*Company, 0, len(builtinRaw))
	for _, r := range builtinRaw {
		out = append(out, &Company{Ticker: r.T, Name: r.N, Sector: r.S, Aliases: r.A})
	}
	return out
}

func mapYahooSector(s, industry string) string {
	switch s {
	case "Technology":
		if containsFold(industry, "semiconductor") {
			return "Semis"
		}
		return "Tech"
	case "Financial Services":
		return "Finance"
	case "Healthcare":
		if containsFold(industry, "drug") || containsFold(industry, "biotech") {
			return "Pharma"
		}
		return "Health"
	case "Consumer Cyclical":
		if containsFold(industry, "auto") {
			return "Auto"
		}
		if containsFold(industry, "travel") || containsFold(industry, "lodging") || containsFold(industry, "resort") {
			return "Travel"
		}
		if containsFold(industry, "retail") {
			return "Retail"
		}
		return "Consumer"
	case "Consumer Defensive":
		if containsFold(industry, "discount") || containsFold(industry, "grocery") {
			return "Retail"
		}
		return "Consumer"
	case "Energy":
		return "Energy"
	case "Industrials":
		if containsFold(industry, "aerospace") || containsFold(industry, "defense") {
			return "Defense"
		}
		if containsFold(industry, "airline") {
			return "Airlines"
		}
		if containsFold(industry, "marine") {
			return "Shipping"
		}
		return "Industrial"
	case "Communication Services":
		if containsFold(industry, "telecom") {
			return "Telecom"
		}
		if containsFold(industry, "internet") {
			return "Tech"
		}
		return "Media"
	case "Basic Materials":
		return "Materials"
	case "Real Estate":
		return "RealEstate"
	case "Utilities":
		return "Utilities"
	}
	return "Other"
}
