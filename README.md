# Auxesis Capital · IIM Ahmedabad

The public website and investor portal of Auxesis Capital, the student-run fund of Beta, the Finance & Investments Club of IIM Ahmedabad. Built from the Claude Design mockups in `design/` (v4), and hosted like garba2026: one binary behind Apache at `students.iima.ac.in/auxesiscapital`.

- **Public site:** the fund story, the growth of ₹100 against the Nifty 50, the latest publications and the team.
- **Investor portal** (Google sign-in, registered emails only):
  - NAV per unit, your holding value, return and XIRR.
  - NAV against the Nifty 50 over 1M, 3M, 6M, 1Y and since inception.
  - Alpha, beta, Sharpe, Sortino, drawdown and up/down capture.
  - Every holding with its weight, today's move and return since entry.
  - Sector allocation, P&L by pod, monthly returns, your unit ledger and the fund manager's note.
- **Publications as articles:** letters, factsheets and research are written in Markdown and read on the site. Investors-only pieces show a preview to everyone else; the server never sends the rest. An optional PDF is downloadable by those allowed to read the piece.
- **Admin page** (`/admin`, for `ADMIN_EMAILS`): sync or upload the tracker, fetch prices, see warnings, and reconcile units against investors.

## Where the numbers come from

| Data | Source |
| --- | --- |
| **Trades** | The tracker workbook: `Overall Fund` sheet, one row per trade from the `Trade ID` header down (sells have negative quantity), plus the small options P&L table above it (e.g. *Sensex 0dte*). Replaced on every sync. |
| Instrument | The *Instrument* cells are Excel "Stocks" linked data types. The server decodes them to the NSE ticker, name and the last price Excel saw. |
| Sector | The first *Industry* value for that stock in the trade log, else the workbook's *Company → Industry* sheet, else Refinitiv's industry. Override per stock in `instruments` (tick `sectorLocked`). |
| Daily closes | Yahoo Finance (`TICKER.NS`, Nifty 50 `^NSEI`, Nifty 500 `^CRSLDX`), downloaded after each sync, topped up with the prices saved in the workbook. Fix a price by editing `prices` (source `manual` is never overwritten). |
| Capital and units | `capital_flows` in our database: money into or out of the fund. |
| Investors | `investors` (email, name, folio) and `investor_txns` (subscriptions, redemptions, transfers). |
| Reports, team, manager's note | `reports`, `team`, `fund_settings` in the database. |

**NAV** = (cash + Σ open quantity × that day's close) ÷ units in issue, every trading day since inception. Cash is the capital in, less the cost of the trades, plus the options P&L. New money buys units at the previous day's NAV. Brokerage on equity trades isn't in the trade log, so NAV is before those charges.

Checked against the tracker from 25 Sep 2026: marked at the workbook's own prices, NAV comes to ₹1017.92, against the sheet's ₹1018.02. Pod A's P&L matches `Overall Positions` to the rupee. Pods B and C differ only by ABREL: the workbook holds two refreshes of its price (₹1232.00 and ₹1230.90), and we use the newer one.

## First-time setup (after deploying)

1. **Capital:** in the dashboard (`/auxesiscapital/_/` → `capital_flows`), add the fund's corpus: date `2026-07-13`, amount `1238342.20`, units `1238.3422`. Or on the server: `./backend/auxesis-server flow 2026-07-13 1238342.20 --units 1238.3422 --note "Corpus at inception" --dir data`.
2. **Investors:** add each investor in `investors`. The email must be the Google account they will sign in with, whether that's Gmail or @iima.ac.in. Then add their units in `investor_txns`: date, `subscription`, amount paid, and units (leave units 0 to price them at that day's NAV). The Admin page shows *Not yet allocated* until every unit is assigned.
3. **Tracker:** set `EXCEL_URL` to the OneDrive link shared as *Anyone with the link can view*, or upload the .xlsx on the Admin page. Syncs run on weekdays at 16:10, 19:10 and 22:10 IST (`SYNC_CRON`).
4. **Team:** add people in `team` (group `leadership`, `pgp2` or `pgp1`, `order`, and a photo).
5. **Manager's note:** edit `fund_settings` (also the risk-free rate used for Sharpe and alpha, and the motto on the home page).

## Writing a report

In the dashboard → `reports`: title, `slug` (the URL, e.g. `letter-q2-fy27`), type (shown as the label, e.g. *Quarterly Letter*), category (Letters, Factsheets, Research or Macro), date, access (`public` or `investors`), author, `dek` (the italic line under the title), `summary` (the card text), `kicker` and `kickerSub` (the big gold word on the banner and the line under it), optional `facts` and PDF, and tick **published**.

`facts` is JSON: `[{"k":"FUND · Q2 FY27","v":"+4.1%","up":true},{"k":"NIFTY 50","v":"+2.3%"}]`.

The body is Markdown with a few extras:

```markdown
The first paragraph becomes the lead, with a gold drop cap.

## A section heading          ← numbered, and listed in the contents

> A pull quote between gold rules.

::chart Growth of ₹100 since inception. Auxesis Capital (gold) against the Nifty 50 (dashed).

::table TOP CONTRIBUTORS · Q2 FY27
| Company | Avg. weight | Contribution |
| --- | --- | --- |
| Dixon Technologies | 3.8% | +1.12 pts |
| Infosys | 6.6% | −0.38 pts |

::sign With conviction, | Name Surname | FUND MANAGER · AUXESIS CAPITAL
```

Lists, `**bold**`, `*italic*`, `[links](https://…)` and `![images](https://…)` work too. In the last column of a table, a leading `+` shows green and `−` shows red. For investors-only pieces, the first three blocks are the public preview.

## How it's built

| Part | Tech | Where |
| --- | --- | --- |
| Backend | [PocketBase](https://pocketbase.io) v0.40 as a Go framework: SQLite, Google OAuth2, admin dashboard at `/_/`, custom routes under `/api/aux/*` | `backend/` |
| Tracker reader | A small .xlsx reader that decodes Excel linked data types (`xl/richData`) | `backend/fund/excel.go` |
| NAV engine | Average-cost books per stock and per pod, daily mark-to-market, risk statistics, XIRR | `backend/fund/calc.go`, `analytics.go` |
| Web app | Preact + Vite, about 35 KB gzipped, self-hosted fonts (Instrument Serif, Source Sans 3, Source Serif 4, IBM Plex Mono) | `web/` |
| API types | Shared TypeScript shapes of the Go responses | `shared/types.ts` |
| Design handoff | The Claude Design mockups and chat | `design/` |

## Run it locally

Needs Go ≥ 1.24 (it fetches the newer toolchain go.mod asks for) and Node ≥ 20.

```sh
npm install
npm run build                                   # web app -> backend/pb_public
cd backend
go run . import ~/Downloads/Auxesis_Capital_Tracker.xlsx
go run . seed --demo-prices                      # sample corpus, investors, team, reports; synthetic prices if Yahoo is unreachable
go run . serve                                   # http://127.0.0.1:8090
```

Google sign-in needs keys (`http://localhost:8090/auth/callback` works as a redirect URI). Without them, create a superuser (`go run . superuser upsert you@x.com 'pass-1234567'`), open `/_/`, and use **impersonate** on a user. For frontend work, run `npm run backend` and `npm run dev` together; Vite on :5173 proxies `/api`.

Tests: `npm run backend:test` covers the tracker reader (linked stock cells, text tickers, unreadable rows, the options table), the NAV engine (flows, units, realised P&L, stale prices, short positions), risk statistics, XIRR, the Yahoo client and report previews. `AUXESIS_TRACKER=/path/to/tracker.xlsx go test ./fund -run Real -v` also checks a real workbook; the workbook itself is kept out of git. `npm run typecheck` checks the web app.

## Deploy on the campus server (students.iima.ac.in/auxesiscapital)

Same shape as garba2026. It needs Node 20+ and git; Go is installed into `./.tools` if missing.

```sh
git clone https://github.com/arnav127/auxesis-capital.git && cd auxesis-capital
./start.sh            # first run creates .env and stops: fill in GOOGLE_*, ADMIN_EMAILS, EXCEL_URL
./start.sh            # installs, builds for /auxesiscapital/, starts under PM2 on 127.0.0.1:8091, health-checks
pm2 startup           # once: run the command it prints (with sudo) so it survives reboots
```

- **Updates:** `git pull && ./start.sh`.
- **Logs:** `pm2 logs auxesiscapital`.
- **Settings:** in `.env` (see `.env.example`). After editing it, run `pm2 startOrReload ecosystem.config.cjs --update-env`.
- **Backups:** back up `DATA_DIR` (default `./data`), or schedule backups in the dashboard (Settings → Backups).
- **The server needs outbound HTTPS** to `query1.finance.yahoo.com` for prices and to SharePoint for the tracker link. If campus blocks them, set `PRICE_HISTORY=off` and upload the tracker on the Admin page; NAV then uses the prices saved in each day's workbook.

### Apache

Paste `deploy/apache-auxesiscapital.conf` inside the existing `<VirtualHost *:443>` for students.iima.ac.in:

```apache
RedirectMatch 301 ^/auxesiscapital$ /auxesiscapital/
ProxyPass        /auxesiscapital/ http://127.0.0.1:8091/ timeout=120
ProxyPassReverse /auxesiscapital/ http://127.0.0.1:8091/
<Location /auxesiscapital/>
    RequestHeader set X-Forwarded-Proto "https"
    RequestHeader set X-Forwarded-Prefix "/auxesiscapital"
</Location>
```

```sh
sudo a2enmod proxy proxy_http headers && sudo apachectl configtest && sudo systemctl reload apache2
```
