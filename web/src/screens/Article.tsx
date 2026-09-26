import { useMemo, useState } from 'preact/hooks';
import type { PublicView, Report } from '../../../shared/types.ts';
import { ArticleBody, parse, sectionId } from '../components/Article.tsx';
import { GoogleG, Spinner } from '../components/Chrome.tsx';
import { ApiError, fmtDate, link, meStore, navigate, pb, startSignIn, useApi } from '../lib.ts';

export function Article({ slug }: { slug: string }) {
  const me = meStore.use();
  const { data: r, error, loading } = useApi<Report>('/reports/' + encodeURIComponent(slug), [!!me]);
  const pub = useApi<PublicView>(r && /::chart/.test(r.body) ? '/public' : null);
  const blocks = useMemo(() => (r ? parse(r.body) : []), [r?.body]);
  const [signInError, setSignInError] = useState('');
  const [pdfBusy, setPdfBusy] = useState(false);

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

  const signIn = () => startSignIn('/publications/' + r.slug).catch((e: ApiError) => setSignInError(e.message));
  const toc = r.toc.length ? r.toc : ['Summary'];
  const goTo = (i: number) => {
    const el = document.getElementById(sectionId(i + 1));
    if (el) window.scrollTo({ top: el.getBoundingClientRect().top + window.scrollY - 100, behavior: 'smooth' });
    else if (r.locked) navigate('/login');
  };
  const downloadPdf = async () => {
    if (r.locked) return navigate('/login');
    setPdfBusy(true);
    try {
      const res = await fetch(`${pb.baseURL}/api/aux/reports/${encodeURIComponent(r.slug)}/pdf`, { headers: pb.authStore.token ? { Authorization: pb.authStore.token } : {} });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).message || 'The PDF could not be downloaded.');
      const url = URL.createObjectURL(await res.blob());
      const a = document.createElement('a');
      a.href = url;
      a.download = r.slug + '.pdf';
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 10000);
    } catch (e) {
      alert((e as Error).message);
    } finally {
      setPdfBusy(false);
    }
  };

  return (
    <article>
      <header class="wrap art-head">
        <div class="crumbs">
          <a {...link('/publications')}>← PUBLICATIONS</a>
          <span class="rule" />
          <span class="gold">{r.type.toUpperCase()}</span>
        </div>
        <h1 class="display">{r.title}</h1>
        {r.dek && <p class="dek">{r.dek}</p>}
        <div class="byline">
          <div>
            {r.author && <dl><dt>WRITTEN BY</dt><dd>{r.author}</dd></dl>}
            <dl><dt>PUBLISHED</dt><dd>{fmtDate(r.date)}</dd></dl>
            <dl><dt>READING TIME</dt><dd>{r.readMins} min</dd></dl>
            <dl><dt>ACCESS</dt><dd style={{ color: r.access === 'public' ? 'var(--ink)' : 'var(--gold)' }}>{r.access === 'public' ? 'Open to all' : 'Investors only'}</dd></dl>
          </div>
          {r.hasPdf && <button class="btn btn-ghost btn-sm" onClick={downloadPdf} disabled={pdfBusy}>{pdfBusy ? 'Preparing…' : `Download PDF${r.pages ? ' · ' + r.pages : ''}`}</button>}
        </div>
      </header>

      <div class="banner">
        <div class="circle-field" data-px="0.1" />
        <div class="vignette" />
        <div>
          <span class="kicker shimmer">{r.kicker || r.category}</span>
          {r.kickerSub && <span class="label" style={{ letterSpacing: '.3em' }}>{r.kickerSub}</span>}
        </div>
      </div>

      <div class="wrap art-body">
        <aside class="art-aside">
          <div class="toc">
            <span class="label">In this piece</span>
            {toc.map((t, i) => <button key={i} onClick={() => goTo(i)}><span>{String(i + 1).padStart(2, '0')}</span>{t}</button>)}
          </div>
          {r.facts && r.facts.length > 0 && (
            <div class="facts">
              {r.facts.map((f, i) => <div key={i}><span class="label dim" style={{ fontSize: 10 }}>{f.k}</span><span class="v" style={{ color: f.up ? 'var(--gold-2)' : 'var(--ink)' }}>{f.v}</span></div>)}
            </div>
          )}
        </aside>
        <div class="prose">
          <ArticleBody blocks={blocks} growth={pub.data?.growth || []} />
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

      {r.next && r.next.slug !== r.slug && (
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
