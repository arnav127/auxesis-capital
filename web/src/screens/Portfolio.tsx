import { useMemo, useState } from 'preact/hooks';
import type { PortfolioView, ReportMeta } from '../../../shared/types.ts';
import { Spinner } from '../components/Chrome.tsx';
import { LineChart, type Pt } from '../components/LineChart.tsx';
import {
  DOWN, UP, cycleEnd, cycleLabel, firstName, fmtDate, greeting, link, meStore, pct, pctPlain, rupees, rupeesShort, tone, units, useApi,
} from '../lib.ts';

const RANGES = [['1M', 22], ['3M', 64], ['6M', 127], ['CYCLE', 0]] as const;
const MONTHS = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC'];
const PALETTE = ['#D1B27A', '#EBD5A6', '#B8914F', '#F1EADB', '#b8b0a0', '#8f9bb3', '#6b7fa6', '#4a5f8a', '#34476e'];
const podName = (p: string) => p.replace(/^pod\s*/i, 'Pod ').trim();
const navFmt = (v: number) => '₹' + (v >= 100 ? v.toLocaleString('en-IN', { maximumFractionDigits: 0 }) : v.toFixed(2));
const ratio = (x: number) => (isFinite(x) && x !== 0 ? x.toFixed(2) : '—');

export function Portfolio() {
  const me = meStore.use();
  const { data: v, error, loading } = useApi<PortfolioView>('/portfolio');
  const reports = useApi<ReportMeta[]>('/reports');
  const [range, setRange] = useState<(typeof RANGES)[number][0]>('CYCLE');

  const chart = useMemo(() => {
    if (!v || v.series.length < 2) return null;
    const n = RANGES.find((r) => r[0] === range)![1];
    const s = n ? v.series.slice(-n) : v.series;
    // Day one of the fund is measured from the issue price.
    const base = !n || n >= v.series.length ? [{ d: v.inception, nav: v.startNav, n50: v.series[0].n50 }, ...s] : s;
    const f0 = base[0].nav;
    const b0 = base.find((p) => p.n50)?.n50;
    const pts: Pt[] = s.map((p) => ({ d: p.d, f: p.nav, b: b0 && p.n50 ? (p.n50 / b0) * f0 : undefined }));
    const last = pts[pts.length - 1];
    const fR = last.f / f0 - 1;
    const bR = last.b != null ? last.b / f0 - 1 : null;
    return { pts, fR, bR };
  }, [v, range]);

  if (loading && !v) return <Spinner />;
  if (error || !v) return <div class="wrap page"><div class="alert">{error?.message || 'Could not load the portfolio.'}</div></div>;

  const hasData = v.series.length > 0 && v.units > 0;
  const mine = v.me;
  const r = v.risk;
  const name = mine?.name || me?.investor?.name || me?.user.name || 'Investor';
  const maxW = Math.max(0.1, ...v.positions.map((p) => p.weight));

  const kpis = [
    { label: 'NAV PER UNIT', value: hasData ? rupees(v.nav, 2) : '—', color: 'var(--ink)', sub: [hasData ? `${v.navChange >= 0 ? '▲' : '▼'} ${pct(v.navChange, 2)}` : '', 'today'], subColor: tone(v.navChange) },
    mine
      ? { label: 'YOUR HOLDING', value: rupees(mine.value), color: 'var(--ink)', sub: [units(mine.units), 'units'], subColor: 'var(--ink)' }
      : { label: 'FUND SIZE', value: rupeesShort(v.aum), color: 'var(--ink)', sub: [units(v.units), 'units in issue'], subColor: 'var(--ink)' },
    mine
      ? { label: 'YOUR RETURN', value: pct(mine.return), color: 'var(--gold-2)', sub: [mine.xirr != null ? 'XIRR ' + pct(mine.xirr) : 'on ' + rupees(mine.invested), mine.xirr != null ? 'on ' + rupees(mine.invested) : ''], subColor: UP }
      : { label: 'FUND RETURN · THIS CYCLE', value: pct(r.fundReturn), color: 'var(--gold-2)', sub: [r.hasBenchmark ? 'Nifty ' + pct(r.benchReturn) : '', 'same period'], subColor: 'var(--ink)' },
    { label: 'ALPHA · ANNUALISED', value: r.hasBenchmark ? pct(r.alpha) : '—', color: 'var(--gold-2)', sub: [r.hasBenchmark ? 'β ' + r.beta.toFixed(2) : '', 'vs Nifty 50'], subColor: 'var(--ink)' },
  ];

  const riskRows: [string, string][] = [
    ['Beta', ratio(r.beta)], ['Sharpe ratio', ratio(r.sharpe)], ['Sortino ratio', ratio(r.sortino)], ['Information ratio', ratio(r.information)],
    ['Volatility', pctPlain(r.volatility)], ['Max drawdown', pct(r.maxDrawdown)], ['Tracking error', pctPlain(r.trackingError)],
    [r.annualised ? 'CAGR' : 'Return, this cycle', pct(r.cagr)],
  ];
  const capture = [
    { k: 'Upside capture', v: r.upCapture, color: '#D1B27A' },
    { k: 'Downside capture', v: r.downCapture, color: '#5d6f91' },
  ];

  // Allocation: top sectors, the rest grouped, cash last.
  const nonCash = v.sectors.filter((s) => s.name !== 'Cash');
  const cash = v.sectors.find((s) => s.name === 'Cash');
  const top = nonCash.slice(0, 8);
  const restW = nonCash.slice(8).reduce((a, s) => a + s.weight, 0);
  const alloc = [
    ...top.map((s, i) => ({ name: s.name, w: s.weight, c: PALETTE[i % PALETTE.length] })),
    ...(restW > 0 ? [{ name: `Other sectors (${nonCash.length - 8})`, w: restW, c: '#34476e' }] : []),
    ...(cash ? [{ name: 'Cash', w: cash.weight, c: '#2a3858' }] : []),
  ];

  const investorReports = (reports.data || []).slice(0, 4);

  return (
    <div class="wrap page">
      <div class="dash-head">
        <div>
          <span class="live"><i />{hasData ? `NAV AS OF ${fmtDate(v.asOf).toUpperCase()} · CLOSE` : 'AWAITING FIRST NAV'}</span>
          <h1 class="display h-md">{greeting()}, <em class="gold-em">{firstName(name)}.</em></h1>
        </div>
        <div class="meta">
          {mine?.folio && <div><span>FOLIO</span><span>{mine.folio}</span></div>}
          {mine?.since && <div><span>INVESTED</span><span>{fmtDate(mine.since).toUpperCase()}</span></div>}
          <div><span>CYCLE</span><span>{v.inception ? `${cycleLabel(v.inception)} · FROM ${fmtDate(v.inception).toUpperCase()}` : '—'}</span></div>
          <div><span>BENCHMARK</span><span>NIFTY 50</span></div>
        </div>
      </div>

      {v.warnings && v.warnings.length > 0 && (
        <div class="alert">
          <b>Admin note.</b> {v.warnings.length} holding{v.warnings.length > 1 ? 's have' : ' has'} no market price and {v.warnings.length > 1 ? 'are' : 'is'} valued at the last trade price. <a {...link('/admin')}>See details</a>.
        </div>
      )}

      <div class="kpis">
        {kpis.map((k) => (
          <div class="kpi" key={k.label}>
            <span class="label">{k.label}</span>
            <span class="v" style={{ color: k.color }}>{k.value}</span>
            <span class="s"><span style={{ color: k.subColor }}>{k.sub[0]}</span><span>{k.sub[1]}</span></span>
          </div>
        ))}
      </div>

      <div class="row" style={{ animation: 'rise .9s .2s both' }}>
        <div class="main card">
          <div class="card-head" style={{ marginBottom: 28, alignItems: 'flex-start' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              <span class="label">NAV per unit vs Nifty 50 (rebased)</span>
              {chart && (
                <div style={{ display: 'flex', alignItems: 'baseline', gap: 18, flexWrap: 'wrap' }}>
                  <span class="serif" style={{ fontSize: 40, lineHeight: 1, color: 'var(--gold-2)' }}>{pct(chart.fR)}</span>
                  {chart.bR != null && <span class="mono muted" style={{ fontSize: 12 }}>Nifty {pct(chart.bR)}</span>}
                  {chart.bR != null && <span class="mono" style={{ fontSize: 12, color: tone(chart.fR - chart.bR) }}>{(chart.fR - chart.bR >= 0 ? '+' : '−') + Math.abs((chart.fR - chart.bR) * 100).toFixed(1)} pts excess</span>}
                </div>
              )}
            </div>
            <div class="seg" role="group" aria-label="Range">
              {RANGES.map(([label, n]) => (
                <button key={label} class={range === label ? 'on' : ''} aria-pressed={range === label} disabled={n > 0 && v.series.length < n / 2} onClick={() => setRange(label)}>{label}</button>
              ))}
            </div>
          </div>
          {chart ? (
            <div style={{ paddingBottom: 30 }}>
              <LineChart id="dashFill" points={chart.pts} height={340} yFormat={navFmt} tip={{ fund: 'NAV', bench: 'Nifty (rebased)', format: (x) => rupees(x, 2) }} />
            </div>
          ) : (
            <div class="empty-state"><span class="label">The chart fills in as daily NAVs are struck.</span></div>
          )}
        </div>

        <div class="side card" style={{ display: 'flex', flexDirection: 'column', gap: 22 }}>
          <span class="label">Risk &amp; alpha · this cycle</span>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6, paddingBottom: 20, borderBottom: '1px solid var(--line)' }}>
            <span style={{ fontSize: 13, color: 'var(--muted)' }}>Jensen’s alpha, annualised</span>
            <span class="serif" style={{ fontSize: 64, lineHeight: 1, color: 'var(--gold-2)' }}>{r.hasBenchmark ? pct(r.alpha) : '—'}</span>
            {!r.annualised && r.days > 0 && <span class="mono dim" style={{ fontSize: 10.5 }}>{r.days} trading days: ratios will steady as history builds</span>}
          </div>
          <div class="metric-grid">{riskRows.map(([k, x]) => <div key={k}><span>{k}</span><span>{x}</span></div>)}</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12, paddingTop: 20, borderTop: '1px solid var(--line)' }}>
            {capture.map((c) => (
              <div key={c.k} style={{ display: 'flex', flexDirection: 'column', gap: 7 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12.5, color: 'var(--muted)' }}><span>{c.k}</span><span class="mono" style={{ color: 'var(--ink)' }}>{r.hasBenchmark ? Math.round(c.v * 100) + '%' : '—'}</span></div>
                <div class="bar"><i style={{ width: Math.max(0, Math.min(100, c.v * 66.6)) + '%', background: c.color }} /><b /></div>
              </div>
            ))}
            <span class="mono dim" style={{ fontSize: 11 }}>Marker = 100% of benchmark move</span>
          </div>
        </div>
      </div>

      <div class="row">
        <div class="main panel" style={{ borderRadius: 0 }}>
          <div class="card-head" style={{ padding: '26px 28px 18px' }}>
            <span class="label">Holdings · {v.positions.length} positions</span>
            <span class="label dim" style={{ letterSpacing: '.1em' }}>Weight · 1D · Since entry</span>
          </div>
          <div class="table-scroll">
            <div class="htable">
              <div class="hrow head"><span>#</span><span>COMPANY</span><span>WEIGHT</span><span style={{ textAlign: 'right' }}>1D</span><span style={{ textAlign: 'right' }}>RETURN</span></div>
              {v.positions.map((p, i) => (
                <div class="hrow" key={p.symbol}>
                  <span class="n">{String(i + 1).padStart(2, '0')}</span>
                  <div class="co">
                    <span title={p.name}>{titleCase(p.name || p.symbol)}</span>
                    <span>{p.symbol} · {p.sector}{p.pods?.length ? ' · ' + p.pods.map(podName).join(', ') : ''}</span>
                  </div>
                  <div class="wbar"><div><i style={{ width: (p.weight / maxW) * 100 + '%' }} /></div><span>{pctPlain(p.weight)}</span></div>
                  <span class="num" style={{ color: p.stale ? 'var(--dim)' : tone(p.day1) }}>{p.stale ? '—' : pct(p.day1, 2)}</span>
                  <span class="num" style={{ color: tone(p.return) }} title={p.stale ? 'No market price: valued at the last trade price' : `Avg cost ${rupees(p.avgCost, 2)} · Last ${rupees(p.price, 2)}`}>{pct(p.return)}{p.stale && <span class="stale">*</span>}</span>
                </div>
              ))}
              <div class="hrow" style={{ color: 'var(--muted)', borderBottom: 0 }}>
                <span />
                <span style={{ fontSize: 14 }}>Cash &amp; equivalents</span>
                <div class="wbar"><div><i style={{ width: Math.max(0, (v.cashWeight / maxW) * 100) + '%', background: '#5d6f91' }} /></div><span>{pctPlain(v.cashWeight)}</span></div>
                <span /><span />
              </div>
            </div>
          </div>
        </div>

        <div class="side" style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
          <div class="card" style={{ display: 'flex', flexDirection: 'column', gap: 22 }}>
            <span class="label">Sector allocation</span>
            <div class="alloc">{alloc.map((s) => <i key={s.name} style={{ width: Math.max(0, s.w) * 100 + '%', background: s.c }} title={s.name} />)}</div>
            <div class="alloc-list">
              {alloc.map((s) => <div key={s.name}><i style={{ background: s.c }} /><span>{s.name}</span><span>{pctPlain(s.w)}</span></div>)}
            </div>
          </div>
          {v.settings.managerNote && (
            <div class="note-card">
              <span class="eyebrow" style={{ fontSize: 10.5, letterSpacing: '.16em' }}>From the fund manager</span>
              <p>“{v.settings.managerNote}”</p>
              <span style={{ fontSize: 12.5, color: 'var(--muted)' }}>{[v.settings.managerNoteBy || 'Fund Manager', v.settings.managerNoteDate && fmtDate(v.settings.managerNoteDate)].filter(Boolean).join(' · ')}</span>
            </div>
          )}
        </div>
      </div>

      {v.pods.length > 0 && (
        <div class="panel" style={{ borderRadius: 0 }}>
          <div class="card-head" style={{ padding: '26px 28px 18px' }}>
            <span class="label">The pods · P&amp;L this cycle</span>
            <span class="label dim" style={{ letterSpacing: '.1em' }}>Realised {rupeesShort(v.realised)} across the fund</span>
          </div>
          <div class="pods" style={{ borderLeft: 0, borderRight: 0, borderBottom: 0 }}>
            {v.pods.map((p) => (
              <div class="pod" key={p.pod}>
                <span class="eyebrow" style={{ fontSize: 10.5, letterSpacing: '.16em' }}>{podName(p.pod)}</span>
                <span class="v" style={{ color: p.pnl >= 0 ? 'var(--gold-2)' : DOWN }}>{p.pnl >= 0 ? '+' : ''}{rupeesShort(p.pnl)}</span>
                <dl>
                  <dt>Realised</dt><dd style={{ color: tone(p.realised) }}>{rupeesShort(p.realised)}</dd>
                  <dt>Unrealised</dt><dd style={{ color: tone(p.unrealised) }}>{rupeesShort(p.unrealised)}</dd>
                  <dt>Open positions</dt><dd>{p.open}</dd>
                  <dt>Trades</dt><dd>{p.trades}</dd>
                </dl>
              </div>
            ))}
          </div>
        </div>
      )}

      {v.monthly.length > 0 && (
        <div class="card" style={{ overflowX: 'auto' }}>
          <div class="card-head" style={{ marginBottom: 22 }}>
            <span class="label">Monthly returns · NAV</span>
            <span class="label dim">%</span>
          </div>
          <div class="heat">
            <div><span />{MONTHS.map((m) => <span class="hd" key={m}>{m}</span>)}<span class="hd gold">YEAR</span></div>
            {v.monthly.map((row) => (
              <div key={row.year}>
                <span class="yr">{row.year}</span>
                {row.months.map((m, i) => {
                  if (m == null) return <div class="c empty" key={i} />;
                  const a = Math.min(0.85, 0.12 + (Math.abs(m) / 0.06) * 0.7);
                  const bg = m >= 0 ? `rgba(209,178,122,${a.toFixed(2)})` : `rgba(224,138,118,${(a * 0.8).toFixed(2)})`;
                  return <div class="c" key={i} style={{ background: bg, color: a > 0.5 && m >= 0 ? 'var(--bg)' : 'var(--ink)' }}>{(m < 0 ? '−' : '') + Math.abs(m * 100).toFixed(1)}</div>;
                })}
                <div class="ytd" style={{ color: row.ytd >= 0 ? 'var(--gold-2)' : DOWN }}>{pct(row.ytd)}</div>
              </div>
            ))}
          </div>
        </div>
      )}

      {mine && mine.txns && mine.txns.length > 0 && (
        <div class="card">
          <div class="card-head" style={{ marginBottom: 12 }}>
            <span class="label">Your account · {mine.folio || mine.email}</span>
            <span class="label dim">{units(mine.units)} units · {rupees(mine.value)}</span>
          </div>
          <div class="table-scroll">
            <table class="txns" style={{ minWidth: 520 }}>
              <thead><tr><th>Date</th><th>Type</th><th class="r">Amount</th><th class="r">NAV</th><th class="r">Units</th></tr></thead>
              <tbody>
                {mine.txns.map((t, i) => (
                  <tr key={i}>
                    <td>{fmtDate(t.date)}</td>
                    <td style={{ textTransform: 'capitalize' }}>{t.kind.replace('_', ' ')}{t.note ? <span class="dim"> · {t.note}</span> : null}</td>
                    <td class="r">{t.amount ? rupees(t.amount) : '—'}</td>
                    <td class="r">{rupees(t.nav, 2)}</td>
                    <td class="r" style={{ color: /redemption|out/.test(t.kind) ? DOWN : 'var(--ink)' }}>{/redemption|out/.test(t.kind) ? '−' : ''}{units(t.units)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <div class="note-card" style={{ flexDirection: 'row', flexWrap: 'wrap', gap: '14px 40px', alignItems: 'baseline', background: 'linear-gradient(160deg,rgba(31,63,115,.35),rgba(10,22,40,.2))' }}>
        <span class="eyebrow" style={{ fontSize: 10.5, letterSpacing: '.16em', flex: '0 0 auto' }}>{v.inception ? `The ${cycleLabel(v.inception)} cycle` : 'This cycle'}</span>
        <span style={{ flex: '1 1 480px', fontSize: 15, lineHeight: 1.65, color: 'var(--ink-2)' }}>
          The fund is fully liquidated in {cycleEnd(v.inception)} and every unit is paid out at the final NAV. PGP1 investors may instead choose to carry their units into the next cycle, where they are re-issued at ₹1,000 a unit; graduating investors are always paid out.
        </span>
      </div>

      {investorReports.length > 0 && (
        <div class="panel" style={{ borderRadius: 0 }}>
          <div class="card-head" style={{ padding: '26px 28px 18px' }}>
            <span class="label">Latest for investors</span>
            <a class="textlink" style={{ fontSize: 10.5, letterSpacing: '.14em' }} {...link('/publications')}>All reports →</a>
          </div>
          {investorReports.map((rp) => (
            <a class="lrow" key={rp.slug} {...link('/publications/' + rp.slug)}>
              <span class="mono dim" style={{ fontSize: 11 }}>{fmtDate(rp.date)}</span>
              <span class="t">{rp.title}</span>
              <span class="label">{rp.type}</span>
              <span class="textlink" style={{ fontSize: 11, textAlign: 'right' }}>Read →</span>
            </a>
          ))}
        </div>
      )}

      <p class="fine" style={{ marginTop: 0 }}>
        NAV is the portfolio marked to each day’s closing prices plus cash, divided by units in issue, before brokerage.
        {v.lastSync ? ` Trades last synced ${new Date(v.lastSync).toLocaleString('en-IN', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Asia/Kolkata' })} IST.` : ''}
        {v.positions.some((p) => p.stale) ? ' * No market price yet: valued at the last trade price.' : ''}
      </p>
    </div>
  );
}

const ACRONYMS = new Set(['ETF', 'ABB', 'HFCL', 'BSE', 'NSE', 'MCX', 'TD', 'RR', 'PB', 'JK', 'ICICI', 'HDFC', 'SBI', 'ITC', 'BLW', 'CNC', 'ESDS', 'AMC', 'IN', 'PVT', '3I', 'NASDAQ', 'BEES', 'CO']);

/** "HDFC BANK LIMITED" → "HDFC Bank Limited"; names already in mixed case are left alone. */
function titleCase(s: string): string {
  if (s !== s.toUpperCase()) return s;
  return s.split(/\s+/).map((w, i) => {
    if (ACRONYMS.has(w.replace(/[^A-Z0-9]/g, ''))) return w;
    if (i > 0 && /^(AND|OF|THE)$/.test(w)) return w.toLowerCase();
    return w.toLowerCase().replace(/(^|[-(.])([a-z])/g, (_, p, c) => p + c.toUpperCase());
  }).join(' ');
}
