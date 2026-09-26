import { useState } from 'preact/hooks';
import type { ReportMeta } from '../../../shared/types.ts';
import { Spinner } from '../components/Chrome.tsx';
import { asset, fmtDate, link, meStore, useApi } from '../lib.ts';

const FILTERS = ['All', 'Letters', 'Factsheets', 'Research', 'Macro'] as const;

export const accessTag = (r: ReportMeta, authed: boolean) =>
  r.access === 'public' ? { cls: 'tag tag-open', label: 'OPEN' } : { cls: 'tag tag-inv', label: authed ? 'INVESTOR' : 'INVESTORS ONLY' };

export const pagesOrRead = (r: ReportMeta) => r.pages || `${r.readMins} min read`;

export function ReportCard({ r }: { r: ReportMeta }) {
  const me = meStore.use();
  const tag = accessTag(r, !!me);
  return (
    <a class="rcard" {...link('/publications/' + r.slug)}>
      <div class="rcard-top"><span class="muted">{r.type.toUpperCase()}</span><span class={tag.cls}>{tag.label}</span></div>
      <h3>{r.title}</h3>
      <div class="rcard-foot"><span>{fmtDate(r.date)}</span><span>{pagesOrRead(r)}</span></div>
    </a>
  );
}

export function Reports() {
  const me = meStore.use();
  const { data, loading, error } = useApi<ReportMeta[]>('/reports', [!!me]);
  const [filter, setFilter] = useState<(typeof FILTERS)[number]>('All');
  const featured = data?.[0];
  const rest = (data || []).slice(1).filter((r) => filter === 'All' || r.category === filter);

  return (
    <div class="wrap page-top" style={{ paddingBottom: 110, display: 'flex', flexDirection: 'column', gap: 56 }}>
      <div class="pub-head">
        <div>
          <span class="eyebrow">Publications</span>
          <h1 class="display h-xl">Letters &amp;<br /><em class="gold-em">research.</em></h1>
        </div>
        <p class="lede" style={{ maxWidth: 480 }}>Quarterly letters, monthly factsheets and the sector research behind each position. Some publications are open to everyone; the rest are reserved for investors.</p>
      </div>

      {loading && !data && <Spinner />}
      {error && <div class="alert">{error.message}</div>}
      {data && data.length === 0 && <div class="panel empty-state"><span class="label">Nothing published yet.</span></div>}

      {featured && (
        <a class="featured" {...link('/publications/' + featured.slug)}>
          <div>
            <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }} class="mono">
              <span class="badge">LATEST</span>
              <span class="label">{featured.type} · {fmtDate(featured.date)}</span>
            </div>
            <h2>{featured.title}</h2>
            {(featured.summary || featured.dek) && <p style={{ margin: 0, fontSize: 15.5, lineHeight: 1.65, color: 'var(--ink-2)', maxWidth: 520 }}>{featured.summary || featured.dek}</p>}
            <div style={{ display: 'flex', gap: 14, alignItems: 'center' }} class="mono">
              <span class="textlink">{featured.locked ? 'Read a preview →' : `Read the ${featured.category === 'Letters' ? 'letter' : 'report'} →`}</span>
              <span class="label dim">{pagesOrRead(featured)}</span>
            </div>
          </div>
          <div class="art" aria-hidden="true">
            <div class="paper" data-px="-0.06">
              <img src={asset('auxesis-mark.png')} alt="" />
              <div>
                <span class="k">{featured.type.toUpperCase()}</span>
                <span class="t">{featured.kicker || fmtDate(featured.date)}</span>
                <hr />
                <span class="f">AUXESIS CAPITAL · IIM AHMEDABAD</span>
              </div>
            </div>
          </div>
        </a>
      )}

      {data && data.length > 1 && (
        <div>
          <div class="filters" role="group" aria-label="Filter publications">
            {FILTERS.map((f) => <button key={f} class={f === filter ? 'on' : ''} aria-pressed={f === filter} onClick={() => setFilter(f)}>{f.toUpperCase()}</button>)}
          </div>
          {rest.map((r) => {
            const tag = accessTag(r, !!me);
            return (
              <a class="prow" key={r.slug} {...link('/publications/' + r.slug)}>
                <span class="mono dim" style={{ fontSize: 11.5 }}>{fmtDate(r.date)}</span>
                <span class="t">{r.title}</span>
                <span class="label">{r.type}</span>
                <span class="mono dim" style={{ fontSize: 11 }}>{pagesOrRead(r)}</span>
                <span class={tag.cls}>{tag.label}</span>
              </a>
            );
          })}
          {rest.length === 0 && <p class="label dim" style={{ padding: '26px 0', borderTop: '1px solid var(--line)' }}>Nothing in this category yet.</p>}
        </div>
      )}
    </div>
  );
}
