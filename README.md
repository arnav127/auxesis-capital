# Auxesis Capital · IIM Ahmedabad

The public website and investor portal of Auxesis Capital, the student-run fund of Beta, the Finance & Investments Club of IIM Ahmedabad. Built from the Claude Design mockups in `design/` (v4), and hosted like garba2026: one binary behind Apache at `students.iima.ac.in/auxesiscapital`.

- **Public site:** the fund story, the growth of ₹1,000 against the Nifty 500 and Nifty 50, the latest publications and the team.
- **Investor portal** (Google sign-in, registered emails only):
  - NAV per unit, your holding value, return and XIRR.
  - NAV against the Nifty 500 (the main benchmark) and the Nifty 50 over 1M, 3M, 6M and the whole cycle.
  - Alpha, beta, Sharpe, Sortino, drawdown and up/down capture against the Nifty 500, with alpha and beta against the Nifty 50 too.
  - Every holding with its weight, today's move and return since entry.
  - Realised P&L this cycle: total booked (stocks and the options book), stocks sold at a gain or loss, best and worst, and every stock sold with quantity, average cost, average sale price, P&L and return (average-cost accounting, so it adds up to the fund's realised P&L).
  - Sector allocation, monthly returns, your unit ledger and the fund manager's note.
  - For admins only: P&L by pod (Pods A–C and Rebalance Delta, the tracker's Pod D) and which pod holds each position. Investors never receive pod data.
- **Publications as articles:** letters, factsheets and research are written in Markdown and read on the site. Investors-only pieces show a preview to everyone else; the server never sends the rest. An optional PDF is downloadable by those allowed to read the piece.
- **Admin page** (`/admin`, for `ADMIN_EMAILS`): sync or upload the tracker, fetch prices, see warnings, reconcile units, and manage investors (add, edit, paste from Excel, or import a CSV/.xlsx list), plus **Guest access** for people who follow the fund without investing, such as the faculty guide. Guests see the portfolio and investor-only publications, never pods or the admin page; removing one signs them out. **Visits & sign-ins** (`/admin/visits`) shows which investors have signed in and when, who hasn't yet (with a copy-emails button), daily visits, publication readers and a recent-visit log; public visitors are counted anonymously with a salted daily hash, and admins' own visits aren't counted.
- **Report editor** (`/admin/publications`): write letters and research on the site with a formatting toolbar, image uploads, key figures, an optional PDF and a live preview; save drafts and publish when ready.

## How the fund works

Auxesis runs in annual cycles. Capital is raised from the IIMA community each July and units are issued at ₹1,000. The fund is fully liquidated at the end of February or in March, and the proceeds are returned. Investors continuing at IIMA next year may carry their units into the next cycle, where they are re-issued at ₹1,000 in proportion to their value; graduating investors are always paid out. 2026–27 is the second cycle (it began on 13 Jul 2026).

The main benchmark is the **Nifty 500**; the Nifty 50 is shown beside it. The site shows the current cycle. Returns, the growth-of-₹1,000 chart (₹1,000 invested at the start is worth the NAV per unit today), risk figures and pod P&L all run from the day the cycle began. Every figure on the public pages comes from the same computed NAV as the investor portal; until the first NAV is struck, those sections stay empty rather than showing sample numbers.

## Where the numbers come from

| Data | Source |
| --- | --- |
| **Trades** | The tracker workbook: `Overall Fund` sheet, one row per trade from the `Trade ID` header down (sells have negative quantity), plus the small options P&L table above it (e.g. *Sensex 0dte*). Replaced on every sync. |
| Instrument | The *Instrument* cells are Excel "Stocks" linked data types. The server decodes them to the NSE ticker, name and the last price Excel saw. |
| Sector | Refinitiv's industry from Excel's linked stock data (e.g. *Banking Services*, *Pharmaceuticals*). ETFs are grouped from their name: Index, Liquid, Gold & Silver or International ETFs. Override per stock in `instruments` (tick `sectorLocked`). |
| Daily closes | Yahoo Finance (`TICKER.NS`, Nifty 50 `^NSEI`, Nifty 500 `^CRSLDX`), downloaded after each sync, topped up with the prices saved in the workbook. Fix a price by editing `prices` (source `manual` is never overwritten). |
| Investors and units | Entered on the Admin page: name, Google email, amount invested, NAV at allotment (1000 at launch) and units. Stored in `investors` and `investor_txns`. Units in issue are the sum of the allotments, unless you record fund-level money in `capital_flows`. |
| Reports, team, manager's note | `reports`, `team`, `fund_settings` in the database. |

**NAV** = (cash + Σ open quantity × that day's close) ÷ units in issue, every trading day since the cycle began. Cash is the capital in, less the cost of the trades, plus the options P&L. New money buys units at the previous day's NAV. Brokerage on equity trades isn't in the trade log, so NAV is before those charges.

Checked against the tracker from 25 Sep 2026: marked at the workbook's own prices, NAV comes to ₹1017.92, against the sheet's ₹1018.02. Pod A's P&L matches `Overall Positions` to the rupee. Pods B and C differ only by ABREL: the workbook holds two refreshes of its price (₹1232.00 and ₹1230.90), and we use the newer one.

## First-time setup (after deploying)

1. **Investors:** sign in with an `ADMIN_EMAILS` account, open **Admin** → Investors → **Import a file**, and choose the investor list (CSV or .xlsx with columns such as Name, Email, Programme/Cohort, Invested, NAV, Units, Date, Note). Check the preview, fill in any missing emails, and import. Each row becomes an allotment; someone on two rows (new money plus a carried-forward holding) gets both. Re-importing the same file skips rows already in. Single investors can also be added with **Add investor**. The email must be the Google account they'll sign in with. NAV defaults to 1000, units to invested ÷ NAV, and the date to the fund's first day (13 Jul 2026). Together the allotments set the units in issue, so NAV is right once everyone is entered (₹12,38,342.20 → 1238.3422 units).
2. *(Optional)* To track the fund's capital separately from the investor list, record it in `capital_flows` instead (e.g. `./backend/auxesis-server flow 2026-07-13 1238342.20 --units 1238.3422 --dir data`). The Admin page then shows any units not yet allocated to investors.
3. **Tracker:** set `EXCEL_URL` to the OneDrive link shared as *Anyone with the link can view*, or upload the .xlsx on the Admin page. Syncs run on weekdays at 16:10, 19:10 and 22:10 IST (`SYNC_CRON`).
4. **Team:** the 2026–27 team is already in; add photos, LinkedIn links and batch in `team` (group `leadership`, `pgp2` or `pgp1`; `order` sets the sequence).
5. **Manager's note:** edit `fund_settings` (also the risk-free rate used for Sharpe and alpha, and an optional motto for the divider on the home page).

## Writing a report

Open **Admin → Write & publish reports** (`/admin/publications`) and click **New publication**. Write in the big text box, use the toolbar for headings, quotes, the growth chart, tables, sign-offs and images, and switch to **Preview** to see it exactly as readers will. **Save draft** keeps it private to admins; **Publish** puts it live. Images uploaded to an investors-only piece are only shown to signed-in investors.

The same fields are editable in the database dashboard → `reports`: title, `slug` (the URL, e.g. `letter-q2-fy27`), type (shown as the label, e.g. *Quarterly Letter*), category (Letters, Factsheets, Research or Macro), date, access (`public` or `investors`), author, `dek` (the italic line under the title), `summary` (the card text), `kicker` and `kickerSub` (the big gold word on the banner and the line under it), optional `facts` and PDF, and tick **published**.

`facts` is JSON: `[{"k":"FUND · Q2 FY27","v":"+4.1%","up":true},{"k":"NIFTY 50","v":"+2.3%"}]`.

The body is Markdown with a few extras:

```markdown
The first paragraph becomes the lead, with a gold drop cap.

## A section heading          ← numbered, and listed in the contents

> A pull quote between gold rules.

::chart Growth of ₹1,000 this cycle. Auxesis Capital (gold) against the Nifty 500 (dashed) and Nifty 50 (dotted).

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
go run . seed --demo-prices                      # sample corpus, investors and reports; synthetic prices if Yahoo is unreachable
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

On **CentOS / RHEL (httpd)**, the file to edit is under `/etc/httpd/conf.d/` (find it with `sudo grep -ril "students.iima.ac.in" /etc/httpd/`). The proxy and headers modules are loaded by default. SELinux must allow httpd to reach the local port, or you'll get a 503:

```sh
httpd -M | grep -E 'proxy_module|proxy_http|headers'   # all three listed
getsebool httpd_can_network_connect                    # if off: sudo setsebool -P httpd_can_network_connect 1
sudo apachectl configtest && sudo systemctl reload httpd
```

On Debian / Ubuntu (apache2): `sudo a2enmod proxy proxy_http headers && sudo apachectl configtest && sudo systemctl reload apache2`.

If the site answers 503, check the app (`pm2 status`, `curl http://127.0.0.1:8091/api/health`) and `sudo tail /var/log/httpd/error_log`.
