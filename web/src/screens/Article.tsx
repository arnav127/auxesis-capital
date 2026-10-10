import { useEffect, useMemo, useState } from 'preact/hooks';
import type { GrowthPoint, PublicView, Report } from '../../../shared/types.ts';
import { ArticleBody, parse, sectionId } from '../components/Article.tsx';
import { GoogleG, Spinner } from '../components/Chrome.tsx';
import { ApiError, asset, fmtDate, link, meStore, navigate, pb, startSignIn, useApi, useMedia } from '../lib.ts';

export function Article({ slug }: { slug: string }) {
  const me = meStore.use();
  const { data: r, error, loading } = useApi<Report>('/reports/' + encodeURIComponent(slug), [!!me]);
  const pub = useApi<PublicView>(r && /::chart/.test(r.body) ? '/public' : null);

  if (loading && !r) return <Spinner />;
  if (error || !r) {
    return (
      <div class="wrap page-top" style={{ paddingBottom: 110, display: 'flex', flexDirection: 'column', gap: 24, alignItems: 'flex-start' }}>
        <span class="eyebrow">Publications</span>
        <h1 class="display h-md">{error?.status === 404 ? 'We couldn’t find that piece.' : 'Something went wrong.'}</h1>
        <a class="btn btn-ghost" {...link('/publications')}>← All publications</a>
      </div>
    );
  }
  return <ArticleView r={r} growth={pub.data?.growth || []} />;
}

/** The article page. Also used by the editor's preview (with `preview` set, links stay inert). */
export function ArticleView({ r, growth, preview }: { r: Report; growth: GrowthPoint[]; preview?: boolean }) {
  const blocks = useMemo(() => parse(r.body), [r.body]);
  const [signInError, setSignInError] = useState('');

  const signIn = () => startSignIn('/publications/' + r.slug).catch((e: ApiError) => setSignInError(e.message));
  const toc = r.toc.length ? r.toc : ['Summary'];
  const goTo = (i: number) => {
    const el = document.getElementById(sectionId(i + 1));
    if (el) window.scrollTo({ top: el.getBoundingClientRect().top + window.scrollY - 100, behavior: 'smooth' });
    else if (r.locked && !preview) navigate('/login');
  };
  const pdf = usePdf(r.hasPdf && !r.locked && !preview ? r.slug : null);
  const toPdf = () => {
    const el = document.getElementById('report-pdf');
    if (el) window.scrollTo({ top: el.getBoundingClientRect().top + window.scrollY - 90, behavior: 'smooth' });
  };

  return (
    <article>
      <header class="wrap art-head" style={preview ? { paddingTop: 56 } : undefined}>
        <div class="crumbs">
          <a {...link('/publications')}>← PUBLICATIONS</a>
          <span class="rule" />
          <span class="gold">{(r.type || 'Publication').toUpperCase()}</span>
        </div>
        <h1 class="display">{r.title || 'Untitled'}</h1>
        {r.dek && <p class="dek">{r.dek}</p>}
        <div class="byline">
          <div>
            {r.author && <dl><dt>WRITTEN BY</dt><dd>{r.author}</dd></dl>}
            <dl><dt>PUBLISHED</dt><dd>{fmtDate(r.date)}</dd></dl>
            {r.hasPdf
              ? <dl><dt>FORMAT</dt><dd>PDF report{r.pages ? ' · ' + r.pages : ''}</dd></dl>
              : <dl><dt>READING TIME</dt><dd>{r.readMins} min</dd></dl>}
            <dl><dt>ACCESS</dt><dd style={{ color: r.access === 'public' ? 'var(--ink)' : 'var(--gold)' }}>{r.access === 'public' ? 'Open to all' : 'Investors only'}</dd></dl>
          </div>
          {r.hasPdf && (
            <div class="btn-row">
              <button class="btn btn-gold btn-lg" onClick={toPdf}>Read the full report ↓</button>
              {!r.locked && <button class="btn btn-ghost btn-sm" onClick={() => !preview && pdf.download()} disabled={!preview && !pdf.url}>Download PDF</button>}
            </div>
          )}
        </div>
      </header>

      {r.hasPdf ? <PdfSection r={r} pdf={pdf} preview={preview} onSignIn={signIn} signInError={signInError} /> : (
      <div class="banner">
        <div class="circle-field" data-px={preview ? undefined : '0.1'} />
        <div class="vignette" />
        <div>
          <span class="kicker shimmer">{r.kicker || r.category || 'Auxesis'}</span>
          {r.kickerSub && <span class="label" style={{ letterSpacing: '.3em' }}>{r.kickerSub}</span>}
        </div>
      </div>
      )}

      <div class="wrap art-body">
        <aside class="art-aside">
          <div class="toc">
            <span class="label">In this piece</span>
            {r.hasPdf && <button onClick={toPdf} class="toc-pdf"><span>PDF</span>Full report</button>}
            {toc.map((t, i) => <button key={i} onClick={() => goTo(i)}><span>{String(i + 1).padStart(2, '0')}</span>{t}</button>)}
          </div>
          {r.facts && r.facts.length > 0 && (
            <div class="facts">
              {r.facts.map((f, i) => <div key={i}><span class="label dim" style={{ fontSize: 10 }}>{f.k}</span><span class="v" style={{ color: f.up ? 'var(--gold-2)' : 'var(--ink)' }}>{f.v}</span></div>)}
            </div>
          )}
        </aside>
        <div class="prose">
          <ArticleBody blocks={blocks} growth={growth} slug={r.slug} />
          {r.locked && (
            <>
              <div class="gate-fade" />
              <div class="gate">
                <span class="eyebrow" style={{ fontSize: 10.5, letterSpacing: '.2em' }}>Investors only</span>
                <h3>The rest of this piece is<br />reserved for investors.</h3>
                <p style={{ margin: 0, fontSize: 15.5, lineHeight: 1.6, color: 'var(--ink-2)', maxWidth: 460 }}>Sign in with the Google account registered with the fund to continue reading, and to see the holdings and figures it refers to.</p>
                <button class="btn btn-google" onClick={signIn}><span class="g"><GoogleG /></span>Continue with Google</button>
                {signInError && <div class="alert">{signInError}</div>}
              </div>
            </>
          )}
        </div>
      </div>

      {!preview && r.next && r.next.slug !== r.slug && (
        <a class="next" {...link('/publications/' + r.next.slug)}>
          <div class="wrap">
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16, maxWidth: 900 }}>
              <span class="eyebrow" style={{ fontSize: 10.5, letterSpacing: '.2em' }}>Next · {r.next.type}</span>
              <span class="t">{r.next.title}</span>
            </div>
            <span class="serif gold" style={{ fontSize: 64, lineHeight: 1 }}>→</span>
          </div>
        </a>
      )}
    </article>
  );
}

type Pdf = ReturnType<typeof usePdf>;

/** Fetches a publication's PDF once (with the investor's session) and keeps it as a blob URL. */
function usePdf(slug: string | null) {
  const [state, setState] = useState<{ url?: string; error?: string; loading: boolean }>({ loading: !!slug });
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    if (!slug) return;
    let url = '';
    let live = true;
    setState({ loading: true });
    fetch(`${pb.baseURL}/api/aux/reports/${encodeURIComponent(slug)}/pdf`, { headers: pb.authStore.token ? { Authorization: pb.authStore.token } : {} })
      .then(async (res) => {
        if (!res.ok) throw new Error((await res.json().catch(() => ({}))).message || 'The PDF could not be loaded.');
        const blob = await res.blob();
        url = URL.createObjectURL(blob.type === 'application/pdf' ? blob : new Blob([blob], { type: 'application/pdf' }));
        if (live) setState({ url, loading: false });
        else URL.revokeObjectURL(url);
      })
      .catch((e: Error) => live && setState({ error: e.message, loading: false }));
    return () => {
      live = false;
      if (url) URL.revokeObjectURL(url);
    };
  }, [slug, attempt]);
  return {
    ...state,
    retry: () => setAttempt((n) => n + 1),
    // Without 'noopener': a blob URL is only reliable in a new tab that keeps its opener.
    open: () => state.url && window.open(state.url, '_blank'),
    download: () => {
      if (!state.url || !slug) return;
      const a = document.createElement('a');
      a.href = state.url;
      a.download = slug + '.pdf';
      a.click();
    },
  };
}

/**
 * The PDF is the publication, so it takes the banner's place: embedded on desktop browsers that
 * show PDFs inline, and a cover to tap on phones (which mostly can't).
 */
function PdfSection({ r, pdf, preview, onSignIn, signInError }: { r: Report; pdf: Pdf; preview?: boolean; onSignIn: () => void; signInError: string }) {
  const wide = useMedia('(min-width: 861px)');
  const inline = wide && (navigator as Navigator & { pdfViewerEnabled?: boolean }).pdfViewerEnabled !== false && !preview && !r.locked;
  const meta = `PDF${r.pages ? ' · ' + r.pages : ''}`;

  const actions = r.locked ? (
    <>
      <button class="btn btn-google" onClick={onSignIn}><span class="g"><GoogleG /></span>Sign in to read the report</button>
      {signInError && <div class="alert">{signInError}</div>}
    </>
  ) : (
    <div class="btn-row">
      <button class="btn btn-gold btn-lg" onClick={() => !preview && pdf.open()} disabled={!preview && !pdf.url}>{pdf.loading ? 'Loading the PDF…' : 'Open the PDF ↗'}</button>
      <button class="btn btn-ghost btn-sm" onClick={() => !preview && pdf.download()} disabled={!preview && !pdf.url}>Download</button>
    </div>
  );

  return (
    <section id="report-pdf" class="pdf-section" aria-label="The full report (PDF)">
      <div class="wrap">
        <div class="pdf-bar">
          <div>
            <span class="eyebrow" style={{ fontSize: 10.5, letterSpacing: '.2em' }}>The full report</span>
            <span class="label">{meta}{r.locked ? ' · investors only' : ''}</span>
          </div>
          {inline && pdf.url && (
            <div class="btn-row">
              <button class="btn btn-ghost btn-sm" onClick={pdf.open}>Open in a new tab ↗</button>
              <button class="btn btn-gold" onClick={pdf.download}>Download PDF</button>
            </div>
          )}
        </div>
        {pdf.error && (
          <div class="alert" style={{ display: 'flex', justifyContent: 'space-between', gap: 16, alignItems: 'center', flexWrap: 'wrap' }}>
            {pdf.error}<button class="btn btn-ghost btn-sm" onClick={pdf.retry}>Try again</button>
          </div>
        )}
        {inline && !pdf.error ? (
          <div class="pdf-frame">
            {pdf.url ? <iframe src={pdf.url + '#navpanes=0&view=FitH'} title={`${r.title} (PDF)`} /> : <div class="spinner" role="status" aria-label="Loading the PDF" />}
          </div>
        ) : !pdf.error && (
          <div class="pdf-cover">
            <button class="paper" onClick={() => !preview && !r.locked && pdf.open()} aria-label={`Open ${r.title} (PDF)`} tabIndex={-1}>
              <img src={asset('auxesis-mark.png')} alt="" />
              <div>
                <span class="k">{(r.type || 'Report').toUpperCase()}</span>
                <span class="t">{r.title}</span>
                <hr />
                <span class="f">AUXESIS CAPITAL · {fmtDate(r.date).toUpperCase()}</span>
              </div>
            </button>
            <div class="pdf-cover-text">
              <h2 class="display">Read the full report.</h2>
              <p>The complete {(r.type || 'report').toLowerCase()} is a PDF{r.pages ? ` of ${r.pages}` : ''}. {r.locked ? 'Sign in with the Google account registered with the fund to open it.' : 'Open it here or save a copy.'}</p>
              {actions}
            </div>
          </div>
        )}
      </div>
    </section>
  );
}
