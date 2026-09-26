import { useEffect, useState } from 'preact/hooks';
import type { GrowthPoint } from '../../../shared/types.ts';
import { pb, rupees } from '../lib.ts';
import { LineChart } from './LineChart.tsx';

/**
 * Reports are Markdown with a few extras (see README "Writing a report"):
 *
 *   First paragraph           lead, with a gold drop cap
 *   ## Heading                numbered section, listed in the contents
 *   > A line                  pull quote between gold rules
 *   ::chart Caption           the fund's growth-of-₹1,000 chart, numbered FIG. n
 *   ::table TITLE + a table   titled table (last columns right-aligned; +/− figures coloured)
 *   ::sign Line | Name | Role signature
 *   ![caption](img:file.png)    an image uploaded in the editor (or any https:// image)
 *   lists, **bold**, *italic*, [links](https://…)
 */

type Block =
  | { t: 'lead' | 'p'; html: string }
  | { t: 'h2'; text: string; n: number }
  | { t: 'h3'; html: string }
  | { t: 'quote'; html: string }
  | { t: 'chart'; caption: string; n: number }
  | { t: 'table'; title: string; head: string[]; rows: string[][] }
  | { t: 'sign'; parts: string[] }
  | { t: 'list'; ordered: boolean; items: string[] }
  | { t: 'img'; alt: string; src: string };

const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

function inline(s: string): string {
  let h = esc(s.trim());
  h = h.replace(/`([^`]+)`/g, '<code>$1</code>');
  h = h.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  h = h.replace(/(^|[^*])\*([^*\s][^*]*)\*/g, '$1<em>$2</em>');
  h = h.replace(/\[([^\]]+)\]\(((?:https?:\/\/|\/|#|mailto:)[^)\s]+)\)/g, (_, t, u) => `<a href="${u}"${u.startsWith('http') ? ' target="_blank" rel="noopener"' : ''}>${t}</a>`);
  return h;
}

const isTableLine = (l: string) => /^\s*\|.*\|\s*$/.test(l);
const cells = (l: string) => l.trim().replace(/^\||\|$/g, '').split('|').map((c) => c.trim());

export function parse(md: string): Block[] {
  const raw = md.replace(/\r\n/g, '\n').split(/\n\s*\n/).map((b) => b.trim()).filter(Boolean);
  const out: Block[] = [];
  let h = 0, fig = 0, leadDone = false;
  for (let i = 0; i < raw.length; i++) {
    const b = raw[i];
    const lines = b.split('\n');
    if (b.startsWith('::chart')) {
      out.push({ t: 'chart', caption: b.slice(7).trim(), n: ++fig });
    } else if (b.startsWith('::sign')) {
      out.push({ t: 'sign', parts: b.slice(6).split('|').map((x) => x.trim()) });
    } else if (b.startsWith('::table') || lines.every(isTableLine)) {
      let title = '';
      let tl = lines;
      if (b.startsWith('::table')) {
        title = lines[0].slice(7).trim();
        tl = lines.slice(1);
        if (tl.length === 0 && i + 1 < raw.length && raw[i + 1].split('\n').every(isTableLine)) tl = raw[++i].split('\n');
      }
      const rows = tl.filter((l) => isTableLine(l) && !/^\s*\|?\s*:?-{2,}/.test(l)).map(cells);
      if (rows.length) out.push({ t: 'table', title, head: rows[0], rows: rows.slice(1) });
    } else if (/^##\s/.test(b) && lines.length === 1) {
      out.push({ t: 'h2', text: b.replace(/^##\s+/, ''), n: ++h });
    } else if (/^###\s/.test(b) && lines.length === 1) {
      out.push({ t: 'h3', html: inline(b.replace(/^###\s+/, '')) });
    } else if (lines.every((l) => l.startsWith('>'))) {
      out.push({ t: 'quote', html: inline(lines.map((l) => l.replace(/^>\s?/, '')).join(' ')) });
    } else if (lines.every((l) => /^\s*[-*]\s+/.test(l) || /^\s{2,}/.test(l))) {
      out.push({ t: 'list', ordered: false, items: lines.filter((l) => /^\s*[-*]\s+/.test(l)).map((l) => inline(l.replace(/^\s*[-*]\s+/, ''))) });
    } else if (lines.every((l) => /^\s*\d+[.)]\s+/.test(l))) {
      out.push({ t: 'list', ordered: true, items: lines.map((l) => inline(l.replace(/^\s*\d+[.)]\s+/, ''))) });
    } else if (/^!\[[^\]]*\]\([^)]+\)$/.test(b)) {
      const m = b.match(/^!\[([^\]]*)\]\(([^)\s]+)\)$/)!;
      if (/^(https?:\/\/|\/|img:)/.test(m[2])) out.push({ t: 'img', alt: m[1], src: m[2] });
    } else {
      const html = inline(lines.join(' '));
      out.push({ t: leadDone ? 'p' : 'lead', html });
      leadDone = true;
    }
  }
  return out;
}

export const sectionId = (n: number) => `sec-${n}`;

const signedCell = (c: string) => (/^[+]/.test(c) ? 'var(--up)' : /^[−-]\s?\d/.test(c) ? 'var(--down)' : undefined);

export function ArticleBody({ blocks, growth, slug }: { blocks: Block[]; growth: GrowthPoint[]; slug: string }) {
  return (
    <>
      {blocks.map((b, i) => {
        switch (b.t) {
          case 'lead': {
            // Drop cap: the first letter of the text (skipping any leading tag).
            const m = b.html.match(/^((?:<[^>]+>)*)([A-Za-z0-9“"‘'])/);
            return m ? (
              <p class="lead" key={i}><span class="cap" aria-hidden="true">{m[2]}</span><span class="sr-only">{m[2]}</span><span dangerouslySetInnerHTML={{ __html: m[1] + b.html.slice(m[0].length) }} /></p>
            ) : <p class="lead" key={i} dangerouslySetInnerHTML={{ __html: b.html }} />;
          }
          case 'p':
            return <p key={i} dangerouslySetInnerHTML={{ __html: b.html }} />;
          case 'h2':
            return <h2 key={i} id={sectionId(b.n)}><span>{String(b.n).padStart(2, '0')}</span>{b.text}</h2>;
          case 'h3':
            return <h3 key={i} dangerouslySetInnerHTML={{ __html: b.html }} />;
          case 'quote':
            return <figure class="pull" key={i}><i aria-hidden="true" /><blockquote dangerouslySetInnerHTML={{ __html: b.html }} /></figure>;
          case 'chart':
            return (
              <figure key={i}>
                <div class="panel" style={{ padding: '24px 24px 44px' }}>
                  <LineChart id={`fig${b.n}`} points={growth} height={240} yFormat={(v) => rupees(v)} tip={{ fund: 'FUND', bench: 'NIFTY 50', format: (v) => rupees(v, 2) }} />
                </div>
                <figcaption><span>FIG. {b.n}</span>{b.caption}</figcaption>
              </figure>
            );
          case 'table':
            return (
              <div class="tbl" key={i}>
                {b.title && <span class="label">{b.title}</span>}
                <table>
                  <thead><tr>{b.head.map((h, j) => <th key={j}>{h}</th>)}</tr></thead>
                  <tbody>{b.rows.map((r, j) => (
                    <tr key={j}>{r.map((c, k) => <td key={k} style={{ color: k > 0 && k === r.length - 1 ? signedCell(c) : undefined }} dangerouslySetInnerHTML={{ __html: inline(c) }} />)}</tr>
                  ))}</tbody>
                </table>
              </div>
            );
          case 'sign':
            return <div class="sign" key={i}>{b.parts.map((p, j) => <span key={j}>{p}</span>)}</div>;
          case 'list':
            return b.ordered
              ? <ol key={i}>{b.items.map((it, j) => <li key={j} dangerouslySetInnerHTML={{ __html: it }} />)}</ol>
              : <ul key={i}>{b.items.map((it, j) => <li key={j} dangerouslySetInnerHTML={{ __html: it }} />)}</ul>;
          case 'img':
            return b.src.startsWith('img:')
              ? <figure key={i}><ReportImage slug={slug} name={b.src.slice(4)} alt={b.alt} />{b.alt && <figcaption>{b.alt}</figcaption>}</figure>
              : <img key={i} src={b.src} alt={b.alt} loading="lazy" />;
        }
      })}
    </>
  );
}

/** An image uploaded to a report. Fetched with the reader's sign-in so investors-only images stay private. */
function ReportImage({ slug, name, alt }: { slug: string; name: string; alt: string }) {
  const [src, setSrc] = useState('');
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let url = '';
    let live = true;
    fetch(`${pb.baseURL}/api/aux/reports/${encodeURIComponent(slug)}/img/${encodeURIComponent(name)}`, { headers: pb.authStore.token ? { Authorization: pb.authStore.token } : {} })
      .then((r) => (r.ok ? r.blob() : Promise.reject()))
      .then((b) => { if (live) { url = URL.createObjectURL(b); setSrc(url); } })
      .catch(() => live && setFailed(true));
    return () => { live = false; if (url) URL.revokeObjectURL(url); };
  }, [slug, name]);
  if (failed) return <div class="panel empty-state"><span class="label dim">Image unavailable: {name}</span></div>;
  return src ? <img src={src} alt={alt} style={{ margin: 0 }} /> : <div class="panel skeleton" style={{ height: 240 }} />;
}
