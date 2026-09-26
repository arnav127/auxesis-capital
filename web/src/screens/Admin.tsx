import { useRef, useState } from 'preact/hooks';
import type { AdminStatus, SyncSummary } from '../../../shared/types.ts';
import { Spinner } from '../components/Chrome.tsx';
import { api, ApiError, pb, pct, rupees, units, useApi } from '../lib.ts';

const when = (s: string) => new Date(s).toLocaleString('en-IN', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Asia/Kolkata' });

/** For fund admins (ADMIN_EMAILS): sync the tracker, fetch prices and check the books. */
export function Admin() {
  const [tick, setTick] = useState(0);
  const { data: s, error, loading } = useApi<AdminStatus>('/admin/status', [tick]);
  const [busy, setBusy] = useState('');
  const [result, setResult] = useState<{ ok: boolean; text: string; warnings?: string[] } | null>(null);
  const [over, setOver] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const run = async (label: string, fn: () => Promise<SyncSummary | { message: string }>) => {
    setBusy(label);
    setResult(null);
    try {
      const r = await fn();
      setResult({ ok: true, text: r.message, warnings: 'warnings' in r ? r.warnings || [] : [] });
    } catch (e) {
      setResult({ ok: false, text: (e as ApiError).message });
    } finally {
      setBusy('');
      setTick((t) => t + 1);
    }
  };
  const upload = (f: File | undefined) => {
    if (!f) return;
    const fd = new FormData();
    fd.append('file', f);
    run('upload', () => api<SyncSummary>('/admin/upload', { body: fd }));
  };

  if (loading && !s) return <Spinner />;
  if (error || !s) return <div class="wrap page"><div class="alert">{error?.message || 'Could not load.'}</div></div>;

  const unallocated = s.flowUnits - s.investorUnits;

  return (
    <div class="wrap page">
      <div class="dash-head">
        <div>
          <span class="live"><i />FUND ADMIN</span>
          <h1 class="display h-md">The <em class="gold-em">books.</em></h1>
        </div>
        <div class="meta">
          <div><span>NAV</span><span>{s.nav ? rupees(s.nav, 4) : '—'}</span></div>
          <div><span>AS OF</span><span>{s.asOf || '—'}</span></div>
          <div><span>TRADES</span><span>{s.counts.trades}</span></div>
          <div><span>PRICES</span><span>{s.counts.prices}</span></div>
        </div>
      </div>

      {result && (
        <div class={result.ok ? 'okay' : 'alert'} role="status">
          {result.text}
          {result.warnings && result.warnings.length > 0 && <ul style={{ margin: '8px 0 0', paddingLeft: 18 }}>{result.warnings.map((w, i) => <li key={i}>{w}</li>)}</ul>}
        </div>
      )}

      <div class="admin-grid">
        <div class="card" style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
          <span class="label">The tracker</span>
          <p style={{ margin: 0, color: 'var(--ink-2)', fontSize: 14.5, lineHeight: 1.6 }}>
            Trades are read from the <b>Overall Fund</b> sheet of the tracker. {s.excelUrl ? 'The server downloads it from the OneDrive link after market close on weekdays.' : 'No OneDrive link is set (EXCEL_URL), so upload the file after each day’s trades.'}
          </p>
          {s.excelUrl && <button class="btn btn-ivory btn-sm" disabled={!!busy} onClick={() => run('sync', () => api<SyncSummary>('/admin/sync', { method: 'POST' }))}>{busy === 'sync' ? 'Syncing…' : 'Sync from OneDrive now'}</button>}
          <label
            class={'dropzone' + (over ? ' over' : '')}
            onDragOver={(e) => { e.preventDefault(); setOver(true); }}
            onDragLeave={() => setOver(false)}
            onDrop={(e) => { e.preventDefault(); setOver(false); upload(e.dataTransfer?.files[0]); }}
          >
            <span class="serif" style={{ fontSize: 26 }}>{busy === 'upload' ? 'Importing…' : 'Upload the tracker'}</span>
            <span class="label dim">Drop the .xlsx here or click to choose</span>
            <input ref={fileRef} type="file" accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" class="sr-only" onChange={(e) => { upload((e.currentTarget as HTMLInputElement).files?.[0]); (e.currentTarget as HTMLInputElement).value = ''; }} />
          </label>
        </div>

        <div class="card" style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
          <span class="label">Prices</span>
          <p style={{ margin: 0, color: 'var(--ink-2)', fontSize: 14.5, lineHeight: 1.6 }}>
            Daily closes come from Yahoo Finance (NSE and the Nifty 50 / Nifty 500), topped up with the prices saved in the tracker. {s.priceHistory ? '' : 'Downloads are switched off (PRICE_HISTORY=off).'}
          </p>
          {s.priceHistory && (
            <div class="btn-row">
              <button class="btn btn-ghost btn-sm" disabled={!!busy} onClick={() => run('prices', () => api<{ message: string }>('/admin/prices', { method: 'POST' }))}>Fetch new closes</button>
              <button class="btn btn-ghost btn-sm" disabled={!!busy} onClick={() => run('prices', () => api<{ message: string }>('/admin/prices?full=1', { method: 'POST' }))}>Re-download all</button>
            </div>
          )}
          {s.noHistory.length > 0 && (
            <p style={{ margin: 0, fontSize: 13, color: 'var(--muted)', lineHeight: 1.6 }}>
              <b style={{ color: 'var(--gold)' }}>No price history yet:</b> {s.noHistory.join(', ')}. If one keeps failing, set its Yahoo symbol in <i>instruments</i>, or add closes in <i>prices</i>.
            </p>
          )}
          <a class="textlink" href={pb.baseURL + '/_/'} target="_blank" rel="noopener">Open the database dashboard →</a>
        </div>

        <div class="card" style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <span class="label">Units reconciliation</span>
          <div class="metric-grid">
            <div><span>Capital in (flows)</span><span>{rupees(s.flowAmount)}</span></div>
            <div><span>Units in issue</span><span>{units(s.flowUnits)}</span></div>
            <div><span>Held by investors</span><span>{units(s.investorUnits)}</span></div>
            <div><span>Not yet allocated</span><span style={{ color: Math.abs(unallocated) > 0.001 ? 'var(--gold)' : 'var(--up)' }}>{units(unallocated)}</span></div>
          </div>
          <p style={{ margin: 0, fontSize: 13, color: 'var(--muted)', lineHeight: 1.6 }}>
            NAV = portfolio value ÷ units from <i>capital_flows</i>. Each investor’s units come from their rows in <i>investor_txns</i>; the two should match once every investor is entered.
          </p>
        </div>
      </div>

      {s.warnings && s.warnings.length > 0 && (
        <div class="card">
          <span class="label">Valuation warnings</span>
          <ul style={{ margin: '14px 0 0', paddingLeft: 18, color: 'var(--ink-2)', fontSize: 14, lineHeight: 1.7 }}>{s.warnings.map((w, i) => <li key={i}>{w}</li>)}</ul>
        </div>
      )}

      <div class="panel" style={{ borderRadius: 0 }}>
        <div class="card-head" style={{ padding: '26px 28px 18px' }}>
          <span class="label">Investors · {s.investors.length}</span>
          <span class="label dim">Add or edit them in the dashboard: investors, investor_txns</span>
        </div>
        <div class="table-scroll" style={{ padding: '0 28px 18px' }}>
          <table class="txns" style={{ minWidth: 720 }}>
            <thead><tr><th>Name</th><th>Email</th><th>Folio</th><th class="r">Units</th><th class="r">Invested</th><th class="r">Value</th><th class="r">Return</th></tr></thead>
            <tbody>
              {s.investors.map((h) => (
                <tr key={h.id}>
                  <td>{h.name}</td>
                  <td class="mono" style={{ fontSize: 12 }}>{h.email}</td>
                  <td class="mono" style={{ fontSize: 12 }}>{h.folio}</td>
                  <td class="r">{units(h.units)}</td>
                  <td class="r">{rupees(h.invested)}</td>
                  <td class="r">{rupees(h.value)}</td>
                  <td class="r" style={{ color: h.return >= 0 ? 'var(--up)' : 'var(--down)' }}>{h.invested ? pct(h.return) : '—'}</td>
                </tr>
              ))}
              {s.investors.length === 0 && <tr><td colSpan={7} class="dim">No investors yet.</td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      <div class="card">
        <span class="label">Sync log</span>
        <div class="log" style={{ marginTop: 14 }}>
          {s.syncs.map((r, i) => (
            <div key={i}>
              <span class="mono" style={{ fontSize: 11, color: 'var(--dim)', letterSpacing: '.08em' }}>
                {when(r.at)} · {r.source.toUpperCase()}{r.by ? ' · ' + r.by : ''} · <span style={{ color: r.ok ? 'var(--up)' : 'var(--down)' }}>{r.ok ? 'OK' : 'FAILED'}</span>
              </span>
              <span>{r.message}</span>
              {r.warnings && r.warnings.length > 0 && (
                <details><summary class="label" style={{ cursor: 'pointer' }}>{r.warnings.length} note{r.warnings.length > 1 ? 's' : ''}</summary><ul>{r.warnings.map((w, j) => <li key={j}>{w}</li>)}</ul></details>
              )}
            </div>
          ))}
          {s.syncs.length === 0 && <div class="dim">Nothing synced yet.</div>}
        </div>
      </div>
    </div>
  );
}
