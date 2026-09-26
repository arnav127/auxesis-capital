import { useEffect, useMemo, useRef, useState } from 'preact/hooks';
import type { Fact, PublicView, Report, ReportDraft } from '../../../shared/types.ts';
import { Spinner } from '../components/Chrome.tsx';
import { api, ApiError, fmtDate, link, navigate, useApi } from '../lib.ts';
import { ArticleView } from './Article.tsx';

const KINDS: [string, ReportDraft['category']][] = [
  ['Quarterly Letter', 'Letters'], ['Annual Review', 'Letters'], ['Monthly Factsheet', 'Factsheets'],
  ['Research', 'Research'], ['Initiating Coverage', 'Research'], ['Sector Review', 'Research'], ['Macro Note', 'Macro'],
];
const today = () => new Date(Date.now() + 5.5 * 3600e3).toISOString().slice(0, 10);
const slugify = (s: string) => s.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 80);

const blank = (): ReportDraft => ({
  id: '', slug: '', title: '', type: 'Quarterly Letter', category: 'Letters', date: today(), access: 'investors', author: '', dek: '', summary: '',
  kicker: '', kickerSub: '', facts: [], body: '', pages: '', published: false, pdf: '', images: [], updated: '',
});

/** Every publication, drafts included. */
export function PublicationsAdmin() {
  const { data, loading, error } = useApi<ReportDraft[]>('/admin/reports');
  return (
    <div class="wrap page">
      <div class="dash-head">
        <div>
          <a class="textlink" {...link('/admin')}>← Admin</a>
          <h1 class="display h-md">Write &amp; <em class="gold-em">publish.</em></h1>
        </div>
        <a class="btn btn-ivory" {...link('/admin/publications/new')}>New publication</a>
      </div>
      {loading && !data && <Spinner />}
      {error && <div class="alert">{error.message}</div>}
      {data && data.length === 0 && <div class="panel empty-state"><span class="label">Nothing written yet. Start with “New publication”.</span></div>}
      {data && data.length > 0 && (
        <div class="panel" style={{ borderRadius: 0 }}>
          {data.map((r) => (
            <a class="lrow" key={r.id} {...link('/admin/publications/' + r.id)} style={{ gridTemplateColumns: '110px minmax(0,1fr) 150px 120px 96px' }}>
              <span class="mono dim" style={{ fontSize: 11 }}>{fmtDate(r.date)}</span>
              <span class="t">{r.title}</span>
              <span class="label">{r.type}</span>
              <span class={r.access === 'public' ? 'tag tag-open' : 'tag tag-inv'} style={{ justifySelf: 'start' }}>{r.access === 'public' ? 'OPEN' : 'INVESTORS'}</span>
              <span class="mono" style={{ fontSize: 11, letterSpacing: '.12em', textAlign: 'right', color: r.published ? 'var(--up)' : 'var(--dim)' }}>{r.published ? 'LIVE' : 'DRAFT'}</span>
            </a>
          ))}
        </div>
      )}
    </div>
  );
}

const SNIPPETS: [string, string, string][] = [
  ['Heading', '\n\n## ', 'Section heading'],
  ['Quote', '\n\n> ', 'A line worth pulling out.'],
  ['Chart', '\n\n::chart ', 'Growth of ₹1,000 this cycle. Auxesis Capital (gold) against the Nifty 500 (dashed) and Nifty 50 (dotted).'],
  ['Table', '\n\n::table TITLE\n| Company | Weight | Contribution |\n| --- | --- | --- |\n| ', 'Name | 5.0% | +0.40 pts |'],
  ['Sign-off', '\n\n::sign ', 'With conviction, | Name Surname | FUND MANAGER · AUXESIS CAPITAL'],
];

/** Create or edit a publication. id is "new" for a fresh one. */
export function ReportEditor({ id }: { id: string }) {
  const isNew = id === 'new';
  const loaded = useApi<ReportDraft>(isNew ? null : '/admin/reports/' + id);
  const pub = useApi<PublicView>('/public');
  const [d, setD] = useState<ReportDraft | null>(isNew ? blank() : null);
  const [slugTouched, setSlugTouched] = useState(!isNew);
  const [tab, setTab] = useState<'write' | 'preview'>('write');
  const [pdf, setPdf] = useState<File | null>(null);
  const [removePdf, setRemovePdf] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState('');
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);
  const bodyRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => { if (loaded.data) setD(loaded.data); }, [loaded.data]);
  useEffect(() => {
    const warn = (e: BeforeUnloadEvent) => { if (dirty) { e.preventDefault(); e.returnValue = ''; } };
    addEventListener('beforeunload', warn);
    return () => removeEventListener('beforeunload', warn);
  }, [dirty]);

  const set = (patch: Partial<ReportDraft>) => {
    setD((cur) => {
      const next = { ...cur!, ...patch };
      if (patch.title !== undefined && !slugTouched) next.slug = slugify(patch.title);
      if (patch.type !== undefined) {
        const k = KINDS.find(([t]) => t.toLowerCase() === patch.type!.toLowerCase());
        if (k) next.category = k[1];
      }
      return next;
    });
    setDirty(true);
  };

  const save = async (publish?: boolean): Promise<ReportDraft | null> => {
    if (!d) return null;
    setBusy(publish === undefined ? 'save' : 'publish');
    setMsg(null);
    try {
      const fd = new FormData();
      fd.append('data', JSON.stringify({ ...d, published: publish ?? d.published }));
      if (pdf) fd.append('pdf', pdf);
      if (removePdf) fd.append('removePdf', '1');
      const saved = await api<ReportDraft>('/admin/reports', { body: fd });
      setD(saved);
      setPdf(null);
      setRemovePdf(false);
      setDirty(false);
      setSlugTouched(true);
      setMsg({ ok: true, text: saved.published ? (publish ? 'Published. It is live on the site.' : 'Saved. The live version is updated.') : 'Draft saved. Only admins can see it.' });
      if (isNew) history.replaceState(null, '', location.pathname.replace(/new$/, saved.id));
      return saved;
    } catch (e) {
      setMsg({ ok: false, text: (e as ApiError).message });
      return null;
    } finally {
      setBusy('');
    }
  };

  // Ctrl/Cmd+S saves.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if ((e.metaKey || e.ctrlKey) && e.key === 's') { e.preventDefault(); save(); } };
    addEventListener('keydown', onKey);
    return () => removeEventListener('keydown', onKey);
  });

  const insert = (before: string, sample = '', after = '') => {
    const ta = bodyRef.current;
    if (!ta || !d) return;
    const { selectionStart: a, selectionEnd: b, value } = ta;
    const chosen = value.slice(a, b) || sample;
    const lead = a === 0 ? before.replace(/^\n+/, '') : before;
    const next = value.slice(0, a) + lead + chosen + after + value.slice(b);
    set({ body: next });
    requestAnimationFrame(() => { ta.focus(); ta.setSelectionRange(a + lead.length, a + lead.length + chosen.length); });
  };

  const uploadImages = async (files: File[]) => {
    if (!files.length || !d) return;
    let cur = d;
    if (!cur.id || dirty) {
      const saved = await save();
      if (!saved) return;
      cur = saved;
    }
    setBusy('image');
    try {
      const fd = new FormData();
      files.forEach((f) => fd.append('images', f));
      const r = await api<{ names: string[] }>(`/admin/reports/${cur.id}/images`, { body: fd });
      setD((x) => ({ ...x!, images: [...x!.images, ...r.names] }));
      insert('\n\n', r.names.map((n) => `![Caption](img:${n})`).join('\n\n'));
    } catch (e) {
      setMsg({ ok: false, text: (e as ApiError).message });
    } finally {
      setBusy('');
    }
  };

  const del = async () => {
    if (!d?.id || !confirm(`Delete “${d.title}”? This can't be undone.`)) return;
    try {
      await api('/admin/reports/' + d.id, { method: 'DELETE' });
      setDirty(false);
      navigate('/admin/publications');
    } catch (e) {
      setMsg({ ok: false, text: (e as ApiError).message });
    }
  };

  const preview: Report | null = useMemo(() => {
    if (!d) return null;
    const words = d.body.split(/\s+/).filter(Boolean).length;
    return {
      slug: d.slug || 'draft', title: d.title, type: d.type, category: (d.category || 'Letters') as Report['category'], date: d.date, access: d.access,
      author: d.author, dek: d.dek, summary: d.summary, kicker: d.kicker, kickerSub: d.kickerSub, pages: d.pages, readMins: Math.max(1, Math.round(words / 220)),
      hasPdf: !!(pdf || (d.pdf && !removePdf)), locked: false, facts: d.facts, body: d.body, preview: false,
      toc: [...d.body.matchAll(/^##\s+(.+?)\s*$/gm)].map((m) => m[1]), next: null,
    };
  }, [d, pdf, removePdf]);

  if (!d) return loaded.error ? <div class="wrap page"><div class="alert">{loaded.error.message}</div></div> : <Spinner />;
  const words = d.body.split(/\s+/).filter(Boolean).length;
  const setFact = (i: number, patch: Partial<Fact>) => set({ facts: d.facts.map((f, j) => (j === i ? { ...f, ...patch } : f)) });

  return (
    <div class="wrap page editor">
      <div class="dash-head" style={{ alignItems: 'center' }}>
        <div style={{ gap: 10 }}>
          <a class="textlink" {...link('/admin/publications')} onClick={(e) => { if (dirty && !confirm('Leave without saving?')) e.preventDefault(); else link('/admin/publications').onClick(e); }}>← All publications</a>
          <span class="live"><i style={{ background: d.published ? 'var(--up)' : 'var(--dim)', animation: d.published ? undefined : 'none' }} />{d.published ? 'LIVE ON THE SITE' : 'DRAFT'}{dirty ? ' · UNSAVED CHANGES' : ''}</span>
        </div>
        <div class="btn-row">
          <div class="seg" role="tablist">
            <button class={tab === 'write' ? 'on' : ''} onClick={() => setTab('write')}>WRITE</button>
            <button class={tab === 'preview' ? 'on' : ''} onClick={() => setTab('preview')}>PREVIEW</button>
          </div>
          <button class="btn btn-ghost btn-sm" disabled={!!busy} onClick={() => save()}>{busy === 'save' ? 'Saving…' : d.published ? 'Save' : 'Save draft'}</button>
          {d.published
            ? <button class="btn btn-ghost btn-sm" disabled={!!busy} onClick={() => save(false)}>Unpublish</button>
            : <button class="btn btn-gold" disabled={!!busy} onClick={() => save(true)}>{busy === 'publish' ? 'Publishing…' : 'Publish'}</button>}
          {d.published && d.slug && <a class="btn btn-ghost btn-sm" href={link('/publications/' + d.slug).href} target="_blank" rel="noopener">View ↗</a>}
        </div>
      </div>

      {msg && <div class={msg.ok ? 'okay' : 'alert'} role="status">{msg.text}</div>}

      {tab === 'preview' && preview ? (
        <div class="panel" style={{ margin: '0 calc(-1 * var(--gutter))', borderLeft: 0, borderRight: 0, background: 'var(--bg)' }}>
          <ArticleView r={preview} growth={pub.data?.growth || []} preview />
        </div>
      ) : (
        <div class="editor-grid">
          <div class="editor-main">
            <input class="ed-title" value={d.title} placeholder="Title, e.g. Letter to Investors, Q2 FY27" onInput={(e) => set({ title: e.currentTarget.value })} />
            <textarea class="ed-dek" rows={2} value={d.dek} placeholder="The italic line under the title: what this piece argues, in a sentence." onInput={(e) => set({ dek: e.currentTarget.value })} />
            <div class="ed-toolbar">
              {SNIPPETS.map(([label, before, sample]) => <button key={label} type="button" onClick={() => insert(before, sample)}>{label}</button>)}
              <button type="button" onClick={() => insert('**', 'bold', '**')}><b>B</b></button>
              <button type="button" onClick={() => insert('*', 'italic', '*')}><i>I</i></button>
              <button type="button" onClick={() => insert('[', 'link text', '](https://)')}>Link</button>
              <label class="ed-img">{busy === 'image' ? 'Uploading…' : 'Image'}<input type="file" accept="image/*" multiple class="sr-only" onChange={(e) => { const files = Array.from(e.currentTarget.files || []); e.currentTarget.value = ''; uploadImages(files); }} /></label>
              <span class="dim mono" style={{ marginLeft: 'auto', fontSize: 10.5 }}>{words} words · {Math.max(1, Math.round(words / 220))} min</span>
            </div>
            <textarea
              ref={bodyRef}
              class="ed-body"
              value={d.body}
              onInput={(e) => set({ body: e.currentTarget.value })}
              placeholder={'Dear investors, the first paragraph becomes the lead, with a gold drop cap.\n\n## A section heading\n\nParagraphs are separated by a blank line.\n\n> A pull quote.\n\n::chart Growth of ₹1,000 this cycle.'}
            />
            <details class="ed-help">
              <summary class="label">How to format</summary>
              <ul>
                <li>The first paragraph is the lead, with a gold drop cap. Separate paragraphs with a blank line.</li>
                <li><code>## Heading</code> starts a numbered section, listed in the contents.</li>
                <li><code>&gt; text</code> is a pull quote. <code>::chart caption</code> draws the growth of ₹1,000 in the fund this cycle against the Nifty 500 and Nifty 50.</li>
                <li><code>::table TITLE</code> followed by a table. In the last column, <code>+</code> figures show green and <code>−</code> figures red.</li>
                <li><code>::sign Line | Name | Role</code> is the sign-off. <b>Image</b> uploads a picture and inserts it; edit the caption in the brackets.</li>
                <li>For investors-only pieces, the first three blocks are the public preview.</li>
              </ul>
            </details>
          </div>

          <aside class="editor-side">
            <div class="card ed-fields">
              <span class="label">Details</span>
              <label>Kind
                <input list="kinds" value={d.type} onInput={(e) => set({ type: e.currentTarget.value })} />
                <datalist id="kinds">{KINDS.map(([k]) => <option key={k} value={k} />)}</datalist>
              </label>
              <label>Category
                <select value={d.category} onChange={(e) => set({ category: e.currentTarget.value as ReportDraft['category'] })}>
                  {['Letters', 'Factsheets', 'Research', 'Macro'].map((c) => <option key={c}>{c}</option>)}
                </select>
              </label>
              <label>Who can read it
                <select value={d.access} onChange={(e) => set({ access: e.currentTarget.value as ReportDraft['access'] })}>
                  <option value="investors">Investors only (public sees a preview)</option>
                  <option value="public">Everyone</option>
                </select>
              </label>
              <div class="ed-2">
                <label>Date<input type="date" value={d.date} onInput={(e) => set({ date: e.currentTarget.value })} /></label>
                <label>Author<input value={d.author} placeholder="Fund Manager" onInput={(e) => set({ author: e.currentTarget.value })} /></label>
              </div>
              <label>Address
                <div class="ed-slug"><span>/publications/</span><input value={d.slug} onInput={(e) => { setSlugTouched(true); set({ slug: slugify(e.currentTarget.value) }); }} /></div>
              </label>
              <label>Card summary<textarea rows={3} value={d.summary} placeholder="Two lines for the publications page (optional)." onInput={(e) => set({ summary: e.currentTarget.value })} /></label>
            </div>

            <div class="card ed-fields">
              <span class="label">Banner</span>
              <div class="ed-2">
                <label>Big word<input value={d.kicker} placeholder="Q2 FY27" onInput={(e) => set({ kicker: e.currentTarget.value })} /></label>
                <label>Line under it<input value={d.kickerSub} placeholder="JULY · AUGUST · SEPTEMBER 2026" onInput={(e) => set({ kickerSub: e.currentTarget.value })} /></label>
              </div>
            </div>

            <div class="card ed-fields">
              <span class="label">Key figures (beside the text)</span>
              {d.facts.map((f, i) => (
                <div class="ed-fact" key={i}>
                  <input value={f.k} placeholder="FUND · Q2 FY27" onInput={(e) => setFact(i, { k: e.currentTarget.value })} />
                  <input value={f.v} placeholder="+4.1%" onInput={(e) => setFact(i, { v: e.currentTarget.value })} />
                  <label title="Show in gold"><input type="checkbox" checked={!!f.up} onChange={(e) => setFact(i, { up: e.currentTarget.checked })} />gold</label>
                  <button type="button" class="textlink" style={{ color: 'var(--down)', fontSize: 10 }} onClick={() => set({ facts: d.facts.filter((_, j) => j !== i) })}>×</button>
                </div>
              ))}
              {d.facts.length < 5 && <button type="button" class="textlink" style={{ fontSize: 10.5, alignSelf: 'flex-start' }} onClick={() => set({ facts: [...d.facts, { k: '', v: '' }] })}>+ Add figure</button>}
            </div>

            <div class="card ed-fields">
              <span class="label">PDF (optional)</span>
              {d.pdf && !removePdf && !pdf && <span style={{ fontSize: 13.5 }}>Attached. <button type="button" class="textlink" style={{ fontSize: 10, color: 'var(--down)' }} onClick={() => { setRemovePdf(true); setDirty(true); }}>Remove</button></span>}
              {pdf && <span style={{ fontSize: 13.5 }}>{pdf.name} <span class="dim">(saved with the next save)</span></span>}
              <label class="btn btn-ghost btn-sm" style={{ cursor: 'pointer', alignSelf: 'flex-start', textTransform: 'none', letterSpacing: 0, fontFamily: 'var(--sans)', fontSize: 13.5, color: 'var(--ink)' }}>
                {d.pdf || pdf ? 'Replace PDF' : 'Attach PDF'}
                <input type="file" accept="application/pdf" class="sr-only" onChange={(e) => { const f = e.currentTarget.files?.[0]; if (f) { setPdf(f); setRemovePdf(false); setDirty(true); } e.currentTarget.value = ''; }} />
              </label>
              <label>Pages<input value={d.pages} placeholder="14 pp" onInput={(e) => set({ pages: e.currentTarget.value })} /></label>
            </div>

            {d.id && <button type="button" class="textlink" style={{ color: 'var(--down)', alignSelf: 'flex-start' }} onClick={del}>Delete this publication</button>}
          </aside>
        </div>
      )}
    </div>
  );
}
