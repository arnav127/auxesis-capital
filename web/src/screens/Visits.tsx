import { useState } from 'preact/hooks';
import type { DayVisits, VisitStats } from '../../../shared/types.ts';
import { Spinner } from '../components/Chrome.tsx';
import { fmtDate, link, useApi } from '../lib.ts';

const GOLD = '#D1B27A';
const BLUE = '#7f93bd';

const ist = (s: string, opts: Intl.DateTimeFormatOptions) => new Date(s).toLocaleString('en-IN', { ...opts, timeZone: 'Asia/Kolkata' });
const when = (s: string) => ist(s, { day: 'numeric', month: 'short', hour: 'numeric', minute: '2-digit' });

function ago(s: string) {
  const m = Math.round((Date.now() - new Date(s).getTime()) / 60000);
  if (m < 2) return 'just now';
  if (m < 60) return `${m} min ago`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h} h ago`;
  const d = Math.round(h / 24);
  return d === 1 ? 'yesterday' : `${d} days ago`;
}

function minutes(a: string, b: string) {
  const m = Math.round((new Date(b).getTime() - new Date(a).getTime()) / 60000);
  return m < 1 ? '<1 min' : `${m} min`;
}

/** Daily bars for one measure, with a hover readout. */
function Bars({ title, days, value, color, unit }: { title: string; days: DayVisits[]; value: (d: DayVisits) => number; color: string; unit: (d: DayVisits) => string }) {
  const [hover, setHover] = useState<number | null>(null);
  const W = 600, H = 120, gap = 2;
  const max = Math.max(1, ...days.map(value));
  const bw = W / days.length;
  const total = days.reduce((s, d) => s + value(d), 0);
  const shown = hover != null ? days[hover] : null;
  return (
    <div class="card" style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      <div class="card-head">
        <span class="label">{title}</span>
        <span class="mono" style={{ fontSize: 12, color: 'var(--ink-2)' }} aria-live="polite">
          {shown ? `${fmtDate(shown.d)} · ${unit(shown)}` : `${total} in 30 days`}
        </span>
      </div>
      <svg viewBox={`0 0 ${W} ${H + 18}`} style={{ width: '100%', height: 'auto', display: 'block' }} role="img" aria-label={`${title}, last 30 days`} onMouseLeave={() => setHover(null)}>
        <line x1={0} x2={W} y1={H} y2={H} stroke="rgba(241,234,219,.14)" />
        {days.map((d, i) => {
          const v = value(d);
          const h = v ? Math.max(3, (v / max) * (H - 6)) : 0;
          const x = i * bw + gap / 2;
          const w = bw - gap;
          return (
            <g key={d.d} onMouseEnter={() => setHover(i)} onClick={() => setHover(i)}>
              <rect x={i * bw} y={0} width={bw} height={H} fill="transparent" />
              {h > 0 && <path d={`M${x},${H} V${H - h + 3} q0,-3 3,-3 h${w - 6} q3,0 3,3 V${H} Z`} fill={color} opacity={hover == null || hover === i ? 1 : 0.45} />}
            </g>
          );
        })}
        <text x={0} y={H + 14} fill="var(--dim)" font-size="10" font-family="var(--mono)">{fmtDate(days[0].d).toUpperCase()}</text>
        <text x={W} y={H + 14} fill="var(--dim)" font-size="10" font-family="var(--mono)" text-anchor="end">TODAY</text>
      </svg>
    </div>
  );
}

/** Admin: who signs in, how often, and what they read. */
export function Visits() {
  const { data: v, error, loading } = useApi<VisitStats>('/admin/visits');
  const [copied, setCopied] = useState(false);
  if (loading && !v) return <Spinner />;
  if (error || !v) return <div class="wrap page"><div class="alert">{error?.message || 'Could not load visits.'}</div></div>;

  const titles = new Map(v.reads.map((r) => [r.slug, r.title]));
  const page = (p: string) => {
    if (p === '/') return 'Home';
    if (p.startsWith('/publications/')) return titles.get(p.slice(14)) || p.slice(14);
    return p.slice(1).replace(/^./, (c) => c.toUpperCase());
  };
  const inv = v.investors;
  const share = inv.total ? Math.round((inv.signedIn / inv.total) * 100) : 0;
  const kpis = [
    { label: 'INVESTORS SIGNED IN', value: `${inv.signedIn}/${inv.total}`, sub: [`${share}%`, 'have ever signed in'] },
    { label: 'ACTIVE INVESTORS · 7 DAYS', value: String(inv.active7), sub: [String(inv.active30), 'in 30 days'] },
    { label: 'SIGNED-IN VISITS · 30 DAYS', value: String(v.visits30), sub: [String(v.visits7), 'in 7 days'] },
    { label: 'PUBLIC VISITORS · 30 DAYS', value: String(v.public.month), sub: [`${v.public.today} today`, `· ${v.public.views30} page views`] },
  ];
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(v.never.map((p) => p.email).join(', '));
      setCopied(true);
    } catch { /* clipboard blocked */ }
  };

  return (
    <div class="wrap page">
      <div class="dash-head">
        <div>
          <span class="live"><i />FUND ADMIN</span>
          <h1 class="display h-md">Who&rsquo;s <em class="gold-em">reading.</em></h1>
          <div class="btn-row"><a class="btn btn-ghost btn-sm" {...link('/admin')}>← Back to the books</a></div>
        </div>
        <p style={{ margin: 0, maxWidth: 420, fontSize: 13.5, lineHeight: 1.6, color: 'var(--muted)' }}>
          Investors and guests are logged by name when they sign in and use the site. Public visitors are counted anonymously; no IP addresses are stored. Admins&rsquo; own visits are not counted.
        </p>
      </div>

      <div class="kpis">
        {kpis.map((k) => (
          <div class="kpi" key={k.label}>
            <span class="label">{k.label}</span>
            <span class="v">{k.value}</span>
            <span class="s"><span style={{ color: 'var(--ink)' }}>{k.sub[0]}</span><span>{k.sub[1]}</span></span>
          </div>
        ))}
      </div>

      <div class="cards-3" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(min(100%, 420px), 1fr))' }}>
        <Bars title="Signed-in visits per day" days={v.daily} value={(d) => d.visits} color={GOLD} unit={(d) => `${d.visits} visit${d.visits === 1 ? '' : 's'} · ${d.people} ${d.people === 1 ? 'person' : 'people'}`} />
        <Bars title="Unique visitors per day, all pages" days={v.daily} value={(d) => d.public} color={BLUE} unit={(d) => `${d.public} visitor${d.public === 1 ? '' : 's'}`} />
      </div>

      <div class="card">
        <div class="card-head" style={{ marginBottom: 12 }}>
          <span class="label">People</span>
          <span class="label dim">{v.people.length} signed in since logging began</span>
        </div>
        {v.people.length === 0 ? <p class="dim" style={{ margin: 0, fontSize: 14 }}>No visits logged yet.</p> : (
          <div class="table-scroll">
            <table class="txns visits" style={{ minWidth: 640 }}>
              <thead><tr><th>Name</th><th class="r">Visits</th><th class="r">Pages</th><th>Last seen</th><th>First seen</th><th>Device</th></tr></thead>
              <tbody>
                {v.people.map((p) => (
                  <tr key={p.email}>
                    <td>{p.name || p.email}{p.role === 'guest' && <span class="tag tag-open" style={{ marginLeft: 8, fontSize: 9 }}>GUEST</span>}<div class="dim" style={{ fontSize: 12 }}>{p.email}</div></td>
                    <td class="r">{p.visits}</td>
                    <td class="r">{p.pages}</td>
                    <td title={when(p.last)}>{ago(p.last)}</td>
                    <td class="dim">{fmtDate(p.first.slice(0, 10))}</td>
                    <td class="dim">{p.device || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div class="cards-3" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(min(100%, 420px), 1fr))', alignItems: 'start' }}>
        <div class="card" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div class="card-head">
            <span class="label">Not signed in yet · {v.never.length}</span>
            {v.never.length > 0 && <button class="textlink" style={{ fontSize: 10.5 }} onClick={copy}>{copied ? 'Copied' : 'Copy emails'}</button>}
          </div>
          {v.never.length === 0 && <p class="dim" style={{ margin: 0, fontSize: 14 }}>Every investor has signed in.</p>}
          <div style={{ maxHeight: 360, overflowY: 'auto', display: 'flex', flexDirection: 'column' }}>
            {v.never.map((p) => (
              <div key={p.email} style={{ display: 'flex', justifyContent: 'space-between', gap: 12, fontSize: 13.5, padding: '8px 0', borderTop: '1px solid var(--line)' }}>
                <span>{p.name}</span><span class="dim" style={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>{p.email}</span>
              </div>
            ))}
          </div>
        </div>
        <div class="card" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <span class="label">Publications · signed-in readers</span>
          {v.reads.length === 0 && <p class="dim" style={{ margin: 0, fontSize: 14 }}>No reads logged yet.</p>}
          {v.reads.map((r) => (
            <div key={r.slug} style={{ display: 'flex', justifyContent: 'space-between', gap: 12, fontSize: 14, padding: '8px 0', borderTop: '1px solid var(--line)' }}>
              <a class="textlink" style={{ textTransform: 'none', letterSpacing: 0, fontSize: 14 }} {...link('/publications/' + r.slug)}>{r.title}</a>
              <span class="mono">{r.readers}</span>
            </div>
          ))}
        </div>
      </div>

      <div class="card">
        <div class="card-head" style={{ marginBottom: 12 }}>
          <span class="label">Recent visits</span>
          <span class="label dim">Times in IST · latest {v.recent.length}</span>
        </div>
        {v.recent.length === 0 ? <p class="dim" style={{ margin: 0, fontSize: 14 }}>No visits logged yet.</p> : (
          <div class="table-scroll">
            <table class="txns visits" style={{ minWidth: 720 }}>
              <thead><tr><th>When</th><th>Who</th><th>Pages viewed</th><th class="r">Length</th><th>Device</th></tr></thead>
              <tbody>
                {v.recent.map((r, i) => (
                  <tr key={i}>
                    <td style={{ whiteSpace: 'nowrap' }}>{when(r.at)}{r.login && <span class="tag tag-inv" style={{ marginLeft: 8, fontSize: 9 }}>SIGN-IN</span>}</td>
                    <td>{r.name || r.email}{r.role === 'guest' && <span class="dim"> · guest</span>}</td>
                    <td style={{ color: 'var(--ink-2)' }}>{r.paths.length ? r.paths.map(page).join(', ') : <span class="dim">—</span>}</td>
                    <td class="r">{minutes(r.at, r.last)}</td>
                    <td class="dim">{r.device || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
