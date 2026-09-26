import type { Member, PublicView, ReportMeta } from '../../../shared/types.ts';
import { GoogleG, Portrait } from '../components/Chrome.tsx';
import { HeroLine, LineChart, placeholderCurve } from '../components/LineChart.tsx';
import { asset, fmtDate, fmtMonthLong, fmtMonthYear, link, pct, pb, useApi } from '../lib.ts';
import { ReportCard } from './Reports.tsx';

const principles = [
  { n: 'I', t: 'Owner’s research', d: 'Every position begins as a written thesis, presented to the desk and defended against a dissenting analyst before capital is committed.' },
  { n: 'II', t: 'Discipline in sizing', d: 'Capital is split across independent pods, each accountable for its own book, with position and sector limits reviewed by the risk desk.' },
  { n: 'III', t: 'Full accountability', d: 'NAV is struck every trading day. Every trade, every mistake and every exit is reported to investors in our letters.' },
];

export function Home() {
  const pub = useApi<PublicView>('/public');
  const reports = useApi<ReportMeta[]>('/reports');
  const team = useApi<Member[]>('/team');
  const p = pub.data;
  const has = !!p?.hasData;
  const growth = has ? p!.growth : [];
  const last = growth[growth.length - 1];
  const leaders = (team.data || []).filter((m) => m.group === 'leadership').slice(0, 3);

  const ticker: [string, string][] = [
    ...(has ? ([['SINCE INCEPTION', pct(p!.fundItd)], ['NIFTY 50', pct(p!.nifty50Itd)], ['POSITIONS', String(p!.positions)]] as [string, string][]) : []),
    ['STYLE', 'Research-led, long-only'], ['UNIVERSE', 'Indian listed equity'],
    ...(has ? ([['INCEPTION', fmtMonthLong(p!.inception)]] as [string, string][]) : []),
    ['MANAGED BY', 'Beta, IIM Ahmedabad'], ['NAV', 'Struck daily'], ['REPORTING', 'Monthly & quarterly'],
  ];

  const heroMove = (e: MouseEvent) => {
    const el = e.currentTarget as HTMLElement;
    const r = el.getBoundingClientRect();
    el.style.setProperty('--mx', (((e.clientX - r.left) / r.width) * 100).toFixed(1) + '%');
    el.style.setProperty('--my', (((e.clientY - r.top) / r.height) * 100).toFixed(1) + '%');
  };

  return (
    <div>
      <section class="hero" onMouseMove={heroMove}>
        <div class="hero-glow" />
        <div class="hero-cols"><div>{[0, 1, 2, 3, 4, 5].map((i) => <div key={i} />)}</div></div>
        <div class="hero-greek" data-px="0.45" aria-hidden="true">αὔξησις</div>
        <div class="hero-circles" data-px="0.2" aria-hidden="true"><div /><div /></div>
        <div class="hero-circle-sm" data-px="0.32" aria-hidden="true"><div /></div>
        <div class="hero-chart" data-px="-0.08" aria-hidden="true">
          <div>{(has || !pub.loading) && <HeroLine points={growth.length > 1 ? growth : placeholderCurve} />}</div>
          <div />
        </div>

        <div class="wrap hero-body">
          <div class="eyebrow hero-eyebrow"><span>Investment fund</span><span class="rule" /><span>Beta · IIM Ahmedabad</span></div>
          <h1>
            <span><span>Conviction,</span></span>
            <span><span class="shimmer">compounded.</span></span>
          </h1>
          <div class="hero-grid">
            <div class="hero-copy">
              <p>Auxesis Capital is the student-managed equity fund of Beta, the Finance &amp; Investments Club of IIM Ahmedabad. We run a research-led portfolio of Indian listed companies.</p>
              <div class="btn-row">
                <a class="btn btn-ivory" {...link('/login')}>Enter investor portal <span style={{ fontSize: 16 }}>→</span></a>
                <a class="btn btn-ghost" {...link('/publications')}>Read our letters</a>
              </div>
            </div>
            <div class="stat-row">
              <div><span class="label">Since inception</span><span class="v" style={{ color: 'var(--gold-2)' }}>{has ? pct(p!.fundItd) : '—'}</span></div>
              <div><span class="label">Nifty 50</span><span class="v">{has ? pct(p!.nifty50Itd) : '—'}</span></div>
              <div><span class="label">Inception</span><span class="v">{has ? fmtMonthYear(p!.inception) : '—'}</span></div>
            </div>
          </div>
        </div>
      </section>

      <div class="ticker" aria-label="Fund facts">
        <div class="ticker-track">
          {[...ticker, ...ticker].map(([k, v], i) => (
            <div class="ticker-item" key={i} aria-hidden={i >= ticker.length}><span class="k">{k}</span><span>{v}</span><i /></div>
          ))}
        </div>
      </div>

      <section class="wrap philosophy">
        <div style={{ position: 'relative' }}>
          <div class="sticky">
            <span class="eyebrow">01 · The name</span>
            <h2 class="display">Auxesis<span class="gold">.</span></h2>
            <div class="pron">αὔξησις · /ɔːkˈsiːsɪs/ · Greek, n.</div>
            <p class="lede" style={{ fontSize: 17, maxWidth: 440 }}>Growth by increase: the steady enlargement of a thing through the accumulation of its parts. It is how we think about capital, and how we think about the analysts who manage it.</p>
          </div>
        </div>
        <div>
          {principles.map((x) => (
            <div class="principle" key={x.n}>
              <span class="n">{x.n}</span>
              <div><h3>{x.t}</h3><p>{x.d}</p></div>
            </div>
          ))}
        </div>
      </section>

      <div class="wrap motto" aria-hidden="true"><span /><span class="diamond" /><b>{p?.motto || 'Per ardua ad alta'}</b><span class="diamond" /><span /></div>

      <section class="wrap" style={{ paddingBottom: 'clamp(96px,12vw,160px)' }}>
        <div class="sec-head" style={{ marginBottom: 40 }}>
          <div>
            <span class="eyebrow">02 · Track record</span>
            <h2 class="display h-lg">Growth of <em class="gold-em">₹100</em><br />since inception</h2>
          </div>
          <div class="legend"><span><i class="l-fund" />AUXESIS CAPITAL</span><span><i class="l-bench" />NIFTY 50</span></div>
        </div>
        <div class="panel chart-panel">
          {has && growth.length > 1 ? (
            <div style={{ paddingBottom: 30 }}>
              <LineChart id="pubFill" points={growth} height="clamp(240px,32vw,400px)" yFormat={(v) => v.toFixed(0)} endDot tip={{ fund: 'FUND', bench: 'NIFTY 50', format: (v) => '₹' + v.toFixed(1) }} />
            </div>
          ) : (
            <div class="empty-state"><span class="label">The track record appears here after the first NAV is struck.</span></div>
          )}
        </div>
        <div class="cells">
          <div><span class="label">₹100 became</span><span class="v" style={{ color: 'var(--gold-2)' }}>{last ? '₹' + last.f.toFixed(1) : '—'}</span></div>
          <div><span class="label">Nifty 50</span><span class="v">{last?.b != null ? '₹' + last.b.toFixed(1) : '—'}</span></div>
          <div><span class="label">Excess return</span><span class="v">{last?.b != null ? (last.f - last.b >= 0 ? '+' : '−') + Math.abs(last.f - last.b).toFixed(1) + ' pts' : '—'}</span></div>
          <a class="cta" {...link('/login')}>
            <span style={{ fontSize: 14, lineHeight: 1.5, color: 'var(--ink-2)' }}>NAV, holdings, risk analytics and investor letters are available in the portal.</span>
            <span class="textlink">Sign in →</span>
          </a>
        </div>
        <p class="fine">Returns are computed on NAV per unit{has ? ` as of ${fmtDate(p!.asOf)}` : ''}, marked to closing prices, before brokerage. The benchmark is the Nifty 50 price index over the same days. Past performance does not indicate future results.</p>
      </section>

      <section class="letters">
        <div class="wrap section">
          <div class="sec-head">
            <div>
              <span class="eyebrow">03 · Research &amp; letters</span>
              <h2 class="display h-lg">We write down<br /><em class="gold-em">what we think.</em></h2>
            </div>
            <a class="textlink" {...link('/publications')}>All publications →</a>
          </div>
          {reports.data && reports.data.length > 0 ? (
            <div class="cards-3">{reports.data.slice(0, 3).map((r) => <ReportCard r={r} key={r.slug} />)}</div>
          ) : (
            <p class="lede">{reports.loading ? '' : 'Our first letters are on their way.'}</p>
          )}
        </div>
      </section>

      <section class="wrap section teaser">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 22 }}>
          <span class="eyebrow">04 · The desk</span>
          <h2 class="display h-lg">Run by students.<br /><em class="gold-em">Held to account.</em></h2>
          <p class="lede" style={{ maxWidth: 460 }}>An investing team drawn from both years of the PGP at IIM Ahmedabad, organised into independent pods and overseen by the fund manager and the Investments Cell of Beta.</p>
          <div><a class="btn btn-ghost btn-sm" style={{ padding: '14px 22px', fontSize: 15 }} {...link('/team')}>Meet the team →</a></div>
        </div>
        <div class="teaser-grid">
          {(leaders.length ? leaders : [null, null, null]).map((l, i) => (
            <div key={i} style={{ display: 'flex', flexDirection: 'column', gap: 12, marginTop: i * 40 }}>
              <Portrait class="r34" name={l?.name || ''} photo={l?.photo ? pb.baseURL + l.photo + '?thumb=600x750' : ''} />
              <div class="eyebrow" style={{ fontSize: 10, letterSpacing: '.14em' }}>{l?.role || ['Coordinator', 'Fund Manager', 'Head, Investments Cell'][i]}</div>
            </div>
          ))}
        </div>
      </section>

      <section class="cta-section">
        <div class="circle-field" data-px="0.12" />
        <div class="cta-inner">
          <img src={asset('auxesis-mark-dark.png')} alt="" style={{ height: 72, width: 'auto' }} />
          <h2 class="display">Your capital,<br /><em class="shimmer">in full view.</em></h2>
          <p class="lede" style={{ maxWidth: 520 }}>Investors see daily NAV, every position, attribution against the Nifty 50 and each letter the moment it is published.</p>
          <a class="btn btn-google" {...link('/login')}><span class="g"><GoogleG /></span>Continue with Google</a>
        </div>
      </section>
    </div>
  );
}
