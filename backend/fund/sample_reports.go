package fund

// Sample publications from the design mockups, for local development only (`seed`).
// They show every block the article page supports; see README "Writing a report".
var sampleReports = []map[string]any{
	{
		"slug": "letter-first-quarter-2026-27", "title": "Letter to Investors, the first quarter of 2026–27", "type": "Quarterly Letter", "category": "Letters",
		"date": "2026-10-12", "access": "investors", "author": "Fund Manager", "pages": "14 pp", "published": true,
		"kicker": "2026–27", "kickerSub": "13 JULY TO 30 SEPTEMBER 2026",
		"summary": "Our first quarter of the 2026–27 cycle: how we deployed the July raise, what drove returns, the positions we exited, and where we are finding value.",
		"dek":     "A strong start for industrials and private banks, two exits we should have made sooner, and why we are adding to capital goods while the market worries about capex.",
		"facts":   []map[string]any{{"k": "FUND · Q1 OF THE CYCLE", "v": "+7.4%", "up": true}, {"k": "NIFTY 50", "v": "+5.1%"}, {"k": "POSITIONS", "v": "18"}},
		"body": `Dear investors, the fund returned 7.4% in the first quarter of the 2026–27 cycle, against 5.1% for the Nifty 50. This is sample text: replace it with the real letter.

The quarter rewarded patience more than activity. We deployed the July raise over the first two weeks, and most of the return came from a handful of high-conviction positions.

## The quarter in brief

Private banks led the market as credit costs stayed benign and deposit growth finally caught up with loans. Industrials followed, helped by order books that now cover more than three years of revenue.

::chart Growth of ₹100 this cycle. Auxesis Capital (gold) against the Nifty 50 (dashed).

## What worked

Our best performer added 1.1 percentage points on its own. The thesis we presented was simple: domestic electronics manufacturing would scale faster than consensus expected. Both halves of that view have held so far.

::table TOP CONTRIBUTORS · FIRST QUARTER
| Company | Avg. weight | Contribution |
| --- | --- | --- |
| Dixon Technologies | 3.8% | +1.12 pts |
| ICICI Bank | 8.4% | +1.06 pts |
| Larsen & Toubro | 6.9% | +0.94 pts |
| Infosys | 6.6% | −0.38 pts |

> We would rather be early and patient than late and certain.

## What we sold

We exited two positions. The first had re-rated well past what its earnings could support; we had argued about trimming it for two quarters and waited too long. The second was a small specialty chemicals holding where our thesis on pricing recovery did not play out.

## Where we are looking

We think the more durable story is private capex in power equipment, cables and grid infrastructure, where order inflows have broadened well beyond the public sector. With the fund due to be liquidated in February or March, we size these positions for the months we have, not for the years the thesis may take.

::sign With conviction, | Name Surname | FUND MANAGER · AUXESIS CAPITAL`,
	},
	{
		"slug": "rbi-last-mile", "title": "The RBI’s last mile: rates into FY27", "type": "Macro Note", "category": "Macro",
		"date": "2026-08-12", "access": "public", "author": "Macro Desk", "pages": "9 pp", "published": true,
		"kicker": "Repo", "kickerSub": "MACRO NOTE · MONETARY POLICY",
		"summary": "Inflation is back inside the band. What remains of the easing cycle, and what it means for banks.",
		"dek":     "Inflation is back inside the band. What remains of the easing cycle, and what it means for banks and rate-sensitive sectors.",
		"facts":   []map[string]any{{"k": "CPI · JULY", "v": "3.9%"}, {"k": "REPO RATE", "v": "5.50%"}, {"k": "10Y G-SEC", "v": "6.31%"}},
		"body": `For most of the past two years the Reserve Bank has been guarding the last mile of disinflation. With headline CPI now below four per cent for three consecutive prints, the question has shifted from whether it will ease to how far. (Sample text.)

Our base case is one further cut of twenty-five basis points before the end of the fiscal year, followed by a long pause.

## What the market is pricing

Overnight index swaps imply roughly forty basis points of easing over the next twelve months, more than we expect.

> Rate cuts help banks less than the market assumes, and help borrowers more.

## Implications for the portfolio

Lower policy rates compress net interest margins before they lift loan growth. We prefer lenders with a high share of current and savings deposits.

::sign The Macro Desk | Auxesis Capital | BETA · IIM AHMEDABAD`,
	},
	{
		"slug": "factsheet-aug-2026", "title": "Factsheet, August 2026", "type": "Monthly Factsheet", "category": "Factsheets",
		"date": "2026-09-05", "access": "investors", "author": "Risk & Portfolio Analytics", "pages": "4 pp", "published": true,
		"kicker": "August", "kickerSub": "MONTHLY FACTSHEET · 2026",
		"dek":  "Monthly NAV, attribution and risk figures for August 2026.",
		"body": "The fund's August figures go here. (Sample text.)\n\nBeta and volatility over the month, and the positions that moved NAV.\n\n::chart Growth of ₹100 this cycle.\n\nFull attribution by sector and position follows.",
	},
}
