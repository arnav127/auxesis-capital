import { useRef, useState } from 'preact/hooks';
import type { AdminStatus, FlowRow, Holding, ImportRow, InvestorImport, SyncSummary } from '../../../shared/types.ts';
import { Spinner } from '../components/Chrome.tsx';
import { api, ApiError, fmtDate, link, pb, pct, rupees, units, useApi } from '../lib.ts';

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
          <div class="btn-row">
            <a class="btn btn-gold" {...link('/admin/publications/new')}>+ New publication</a>
            <a class="btn btn-ghost btn-sm" {...link('/admin/publications')}>All publications &amp; drafts</a>
          </div>
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
          <FundCapital flows={s.flows} onChange={() => setTick((t) => t + 1)} />
          <p style={{ margin: 0, fontSize: 13, color: 'var(--muted)', lineHeight: 1.6 }}>
            {s.flowsFromInvestors
              ? 'Units in issue are the sum of the investors’ allotments below. NAV = portfolio value ÷ units in issue.'
              : 'NAV = portfolio value ÷ units from capital_flows. Each investor’s units come from their allotments below; the two should match once every investor is entered.'}
          </p>
        </div>
      </div>

      {s.warnings && s.warnings.length > 0 && (
        <div class="card">
          <span class="label">Valuation warnings</span>
          <ul style={{ margin: '14px 0 0', paddingLeft: 18, color: 'var(--ink-2)', fontSize: 14, lineHeight: 1.7 }}>{s.warnings.map((w, i) => <li key={i}>{w}</li>)}</ul>
        </div>
      )}

      <Investors list={s.investors} onChange={() => setTick((t) => t + 1)} />

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

interface Draft { id: string; name: string; email: string; folio: string; amount: string; nav: string; units: string; date: string }
const blank: Draft = { id: '', name: '', email: '', folio: '', amount: '', nav: '1000', units: '', date: '' };
const num = (x: string) => Number(String(x).replace(/[₹,\s]/g, '')) || 0;

function allotment(h: Holding) {
  return (h.txns || []).find((t) => t.kind === 'subscription');
}

/** Investors and their allotment: add, edit, remove, or paste many from Excel. */
function Investors({ list, onChange }: { list: Holding[]; onChange: () => void }) {
  const [draft, setDraft] = useState<Draft | null>(null);
  const [bulk, setBulk] = useState<string | null>(null);
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);
  const [busy, setBusy] = useState(false);

  const call = async (fn: () => Promise<{ message: string }>, done?: () => void) => {
    setBusy(true);
    setMsg(null);
    try {
      const r = await fn();
      setMsg({ ok: true, text: r.message });
      done?.();
      onChange();
    } catch (e) {
      setMsg({ ok: false, text: (e as ApiError).message });
    } finally {
      setBusy(false);
    }
  };
  const edit = (h: Holding) => {
    const a = allotment(h);
    setBulk(null);
    setDraft({ id: h.id, name: h.name, email: h.email, folio: h.folio, amount: String(a?.amount ?? h.invested), nav: String(a?.nav ?? 1000), units: String(a?.units ?? h.units), date: a?.date ?? '' });
  };
  const save = (d: Draft) =>
    call(() => api('/admin/investors', { body: { id: d.id, name: d.name, email: d.email, folio: d.folio, amount: num(d.amount), nav: num(d.nav), units: num(d.units), date: d.date } }), () => setDraft(null));
  const remove = (h: Holding) => {
    if (confirm(`Remove ${h.name} and their ${units(h.units)} units?`)) call(() => api('/admin/investors/' + h.id, { method: 'DELETE' }));
  };
  const saveBulk = (text: string) => {
    const rows = text.split(/\r?\n/).map((l) => l.split(/\t|,(?=(?:[^"]*"[^"]*")*[^"]*$)/).map((c) => c.replace(/^"|"$/g, '').trim())).filter((c) => c.some(Boolean) && !/e-?mail/i.test(c.join(' ')));
    call(() => api('/admin/investors/bulk', { body: { rows: rows.map((c) => ({ name: c[0], email: c[1], amount: num(c[2]), nav: num(c[3] || '1000'), units: num(c[4] || ''), date: c[5] || '' })) } }), () => setBulk(null));
  };
  const [preview, setPreview] = useState<(ImportRow & { on: boolean })[] | null>(null);
  const [previewInfo, setPreviewInfo] = useState('');
  const readImport = async (f: File) => {
    setDraft(null);
    setBulk(null);
    setMsg(null);
    setBusy(true);
    try {
      const fd = new FormData();
      fd.append('file', f);
      const r = await api<InvestorImport>('/admin/investors/import', { body: fd });
      setPreview(r.rows.map((x) => ({ ...x, on: !x.issue })));
      setPreviewInfo(`${f.name} · sheet “${r.sheet}” · ${r.rows.length} rows · ${units(r.units)} units · ${rupees(r.amount, 2)}`);
    } catch (e) {
      setMsg({ ok: false, text: (e as ApiError).message });
    } finally {
      setBusy(false);
    }
  };
  const confirmImport = () => {
    const rows = (preview || []).filter((r) => r.on);
    call(() => api('/admin/investors/import/confirm', { body: { rows } }), () => setPreview(null));
  };
  const autoUnits = draft && num(draft.amount) > 0 && num(draft.nav) > 0 ? (num(draft.amount) / num(draft.nav)).toFixed(4) : '';

  return (
    <div class="panel" style={{ borderRadius: 0 }}>
      <div class="card-head" style={{ padding: '26px 28px 18px' }}>
        <span class="label">Investors · {list.length}</span>
        <div class="btn-row">
          <button class="btn btn-ivory btn-sm" disabled={busy} onClick={() => { setBulk(null); setDraft({ ...blank }); }}>Add investor</button>
          <button class="btn btn-ghost btn-sm" disabled={busy} onClick={() => { setDraft(null); setBulk(''); }}>Paste from Excel</button>
          <label class="btn btn-ghost btn-sm" style={{ cursor: 'pointer' }}>
            Import a file
            <input type="file" accept=".csv,.xlsx,text/csv" class="sr-only" onChange={(e) => { const f = (e.currentTarget as HTMLInputElement).files?.[0]; (e.currentTarget as HTMLInputElement).value = ''; if (f) readImport(f); }} />
          </label>
        </div>
      </div>
      <div style={{ padding: '0 28px' }}>
        {msg && <div class={msg.ok ? 'okay' : 'alert'} style={{ marginBottom: 16 }}>{msg.text}</div>}
        {draft && (
          <form class="inv-form" onSubmit={(e) => { e.preventDefault(); save(draft); }}>
            <label>Name<input required value={draft.name} onInput={(e) => setDraft({ ...draft, name: e.currentTarget.value })} /></label>
            <label>Google email<input required type="email" value={draft.email} onInput={(e) => setDraft({ ...draft, email: e.currentTarget.value })} placeholder="name@gmail.com" /></label>
            <label>Invested (₹)<input required inputMode="decimal" value={draft.amount} onInput={(e) => setDraft({ ...draft, amount: e.currentTarget.value, units: '' })} placeholder="20000" /></label>
            <label>NAV at allotment<input required inputMode="decimal" value={draft.nav} onInput={(e) => setDraft({ ...draft, nav: e.currentTarget.value, units: '' })} /></label>
            <label>Units<input inputMode="decimal" value={draft.units} onInput={(e) => setDraft({ ...draft, units: e.currentTarget.value })} placeholder={autoUnits || 'amount ÷ NAV'} /></label>
            <label>Allotted on<input type="date" value={draft.date} onInput={(e) => setDraft({ ...draft, date: e.currentTarget.value })} /></label>
            <label>Folio (optional)<input value={draft.folio} onInput={(e) => setDraft({ ...draft, folio: e.currentTarget.value })} placeholder="AUX-0001" /></label>
            <div class="btn-row" style={{ alignSelf: 'end' }}>
              <button class="btn btn-gold" type="submit" disabled={busy}>{busy ? 'Saving…' : draft.id ? 'Save changes' : 'Add'}</button>
              <button class="btn btn-ghost btn-sm" type="button" onClick={() => setDraft(null)}>Cancel</button>
            </div>
            <p class="dim" style={{ gridColumn: '1 / -1', margin: 0, fontSize: 12.5 }}>Leave units empty to use invested ÷ NAV. Leave the date empty to use the day this cycle began. The investor signs in with this email.</p>
          </form>
        )}
        {bulk != null && (
          <form onSubmit={(e) => { e.preventDefault(); saveBulk(bulk); }} style={{ display: 'flex', flexDirection: 'column', gap: 12, marginBottom: 20 }}>
            <p style={{ margin: 0, fontSize: 13.5, color: 'var(--ink-2)', lineHeight: 1.6 }}>
              Copy rows from Excel and paste them here, one investor per line, in this column order:
              <span class="mono gold" style={{ fontSize: 12 }}> Name · Email · Invested · NAV · Units · Date</span>.
              NAV defaults to 1000, units to invested ÷ NAV and date to the day this cycle began. Existing emails are updated.
            </p>
            <textarea class="inv-bulk" rows={8} value={bulk} onInput={(e) => setBulk(e.currentTarget.value)} placeholder={'Heth Doshi\thethdoshi@gmail.com\t50000\t1000\t50'} />
            <div class="btn-row">
              <button class="btn btn-gold" type="submit" disabled={busy || !bulk.trim()}>{busy ? 'Saving…' : 'Add all'}</button>
              <button class="btn btn-ghost btn-sm" type="button" onClick={() => setBulk(null)}>Cancel</button>
            </div>
          </form>
        )}
      </div>
      {preview && (
        <div style={{ padding: '0 28px 24px' }}>
          <div class="inv-form" style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <p style={{ margin: 0, fontSize: 13.5, color: 'var(--ink-2)', lineHeight: 1.6 }}>
              <b>Check before importing.</b> {previewInfo}. Each row becomes an allotment; a person on two rows gets both. Rows already imported are skipped, so importing the same file twice is safe.
            </p>
            <div class="table-scroll">
              <table class="txns" style={{ minWidth: 980 }}>
                <thead><tr><th /><th>Name</th><th>Google email</th><th>Cohort</th><th class="r">Invested</th><th class="r">NAV</th><th class="r">Units</th><th>Note</th></tr></thead>
                <tbody>
                  {preview.map((r, i) => {
                    const set = (patch: Partial<ImportRow & { on: boolean }>) => setPreview(preview.map((x, j) => (j === i ? { ...x, ...patch } : x)));
                    const needsEmail = !r.email;
                    return (
                      <tr key={i} style={{ opacity: r.on ? 1 : 0.45 }}>
                        <td><input type="checkbox" checked={r.on} disabled={needsEmail} onChange={(e) => set({ on: e.currentTarget.checked })} aria-label={'Import ' + r.name} /></td>
                        <td>{r.name}{r.issue && <div style={{ color: 'var(--gold)', fontSize: 12 }}>{r.issue}</div>}</td>
                        <td><input class="cell-input" type="email" value={r.email} placeholder="needed" style={{ borderColor: needsEmail ? 'var(--gold)' : undefined }} onInput={(e) => { const v = e.currentTarget.value.trim(); set({ email: v, on: !!v }); }} /></td>
                        <td>{r.programme}</td>
                        <td class="r">{rupees(r.amount, 2)}</td>
                        <td class="r">{rupees(r.nav)}</td>
                        <td class="r">{units(r.units)}</td>
                        <td class="dim" style={{ fontSize: 12.5 }}>{r.note}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
            <div class="btn-row">
              <button class="btn btn-gold" disabled={busy || !preview.some((r) => r.on)} onClick={confirmImport}>
                {busy ? 'Importing…' : `Import ${preview.filter((r) => r.on).length} rows · ${units(preview.filter((r) => r.on).reduce((a, r) => a + r.units, 0))} units`}
              </button>
              <button class="btn btn-ghost btn-sm" onClick={() => setPreview(null)}>Cancel</button>
            </div>
          </div>
        </div>
      )}
      <div class="table-scroll" style={{ padding: '0 28px 18px' }}>
        <table class="txns" style={{ minWidth: 900 }}>
          <thead><tr><th>Name</th><th>Email</th><th class="r">Invested</th><th class="r">Allot NAV</th><th class="r">Units</th><th class="r">Value</th><th class="r">Return</th><th /></tr></thead>
          <tbody>
            {list.map((h) => {
              const a = allotment(h);
              return (
                <tr key={h.id}>
                  <td>{h.name}{h.folio && <span class="dim mono" style={{ fontSize: 11 }}> · {h.folio}</span>}{(h.txns?.length || 0) > 1 && <span class="dim" style={{ fontSize: 11.5 }}> · {h.txns!.length} allotments</span>}</td>
                  <td class="mono" style={{ fontSize: 12 }}>{h.email}</td>
                  <td class="r">{rupees(h.invested)}</td>
                  <td class="r">{a ? rupees(a.nav, 2) : '—'}</td>
                  <td class="r">{units(h.units)}</td>
                  <td class="r">{rupees(h.value)}</td>
                  <td class="r" style={{ color: h.return >= 0 ? 'var(--up)' : 'var(--down)' }}>{h.invested ? pct(h.return) : '—'}</td>
                  <td class="r" style={{ whiteSpace: 'nowrap' }}>
                    <button class="textlink" style={{ fontSize: 10.5, marginRight: 14 }} onClick={() => edit(h)}>Edit</button>
                    <button class="textlink" style={{ fontSize: 10.5, color: 'var(--down)' }} onClick={() => remove(h)}>Remove</button>
                  </td>
                </tr>
              );
            })}
            {list.length === 0 && <tr><td colSpan={8} class="dim">No investors yet. Add them one by one or paste the list from Excel.</td></tr>}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/** Money into or out of the fund as a whole. While empty, units in issue are the investors’ allotments. */
function FundCapital({ flows, onChange }: { flows: FlowRow[]; onChange: () => void }) {
  const [open, setOpen] = useState(false);
  const [f, setF] = useState({ date: '2026-07-13', amount: '', units: '', note: '' });
  const [err, setErr] = useState('');
  const save = async () => {
    setErr('');
    try {
      await api('/admin/flows', { body: { date: f.date, amount: num(f.amount), units: num(f.units), note: f.note } });
      setOpen(false);
      setF({ date: '2026-07-13', amount: '', units: '', note: '' });
      onChange();
    } catch (e) {
      setErr((e as ApiError).message);
    }
  };
  const remove = async (id: string) => {
    if (!confirm('Remove this entry? NAV is recomputed.')) return;
    try { await api('/admin/flows/' + id, { method: 'DELETE' }); onChange(); } catch (e) { setErr((e as ApiError).message); }
  };
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10, paddingTop: 14, borderTop: '1px solid var(--line)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
        <span class="label dim">Fund capital</span>
        {!open && <button class="textlink" style={{ fontSize: 10.5 }} onClick={() => setOpen(true)}>Add entry</button>}
      </div>
      {flows.length === 0 && !open && <span style={{ fontSize: 13, color: 'var(--muted)' }}>None recorded: units follow the investor list.</span>}
      {flows.map((x) => (
        <div key={x.id} style={{ display: 'flex', justifyContent: 'space-between', gap: 10, fontSize: 13 }}>
          <span>{fmtDate(x.date)} · {rupees(x.amount, 2)}{x.units ? ` · ${units(x.units)} units` : ''}{x.note ? <span class="dim"> · {x.note}</span> : null}</span>
          <button class="textlink" style={{ fontSize: 10, color: 'var(--down)' }} onClick={() => remove(x.id)}>Remove</button>
        </div>
      ))}
      {open && (
        <form class="inv-form" style={{ marginBottom: 0, padding: 14 }} onSubmit={(e) => { e.preventDefault(); save(); }}>
          <label>Date<input type="date" required value={f.date} onInput={(e) => setF({ ...f, date: e.currentTarget.value })} /></label>
          <label>Amount (₹)<input required inputMode="decimal" value={f.amount} onInput={(e) => setF({ ...f, amount: e.currentTarget.value })} placeholder="1238342.20" /></label>
          <label>Units<input inputMode="decimal" value={f.units} onInput={(e) => setF({ ...f, units: e.currentTarget.value })} placeholder="at previous NAV" /></label>
          <label>Note<input value={f.note} onInput={(e) => setF({ ...f, note: e.currentTarget.value })} placeholder="Capital raised for the cycle" /></label>
          <div class="btn-row"><button class="btn btn-gold" type="submit">Save</button><button class="btn btn-ghost btn-sm" type="button" onClick={() => setOpen(false)}>Cancel</button></div>
        </form>
      )}
      {err && <div class="alert">{err}</div>}
    </div>
  );
}
