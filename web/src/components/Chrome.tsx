import { useEffect, useState } from 'preact/hooks';
import { asset, initials, link, meStore, routeStore, signOut } from '../lib.ts';
import type { Photo } from '../team-photos.ts';

export const GoogleG = ({ size = 16 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 48 48" aria-hidden="true">
    <path fill="#EA4335" d="M24 9.5c3.5 0 6.6 1.2 9.1 3.6l6.8-6.8C35.8 2.4 30.3 0 24 0 14.6 0 6.6 5.4 2.7 13.3l7.9 6.2C12.5 13.6 17.8 9.5 24 9.5z" />
    <path fill="#4285F4" d="M46.1 24.5c0-1.6-.1-3.1-.4-4.5H24v9h12.4c-.5 2.9-2.2 5.3-4.6 6.9l7.4 5.8c4.3-4 6.9-9.9 6.9-17.2z" />
    <path fill="#FBBC05" d="M10.6 28.5c-.5-1.4-.8-2.9-.8-4.5s.3-3.1.8-4.5l-7.9-6.2C1 16.6 0 20.2 0 24s1 7.4 2.7 10.7l7.9-6.2z" />
    <path fill="#34A853" d="M24 48c6.5 0 11.9-2.1 15.9-5.8l-7.4-5.8c-2.1 1.4-4.8 2.3-8.5 2.3-6.2 0-11.5-4.1-13.4-9.8l-7.9 6.2C6.6 42.6 14.6 48 24 48z" />
  </svg>
);

export const Spinner = () => <div class="center-screen"><div class="spinner" role="status" aria-label="Loading" /></div>;

function Brand({ to }: { to: string }) {
  return (
    <a class="brand" {...link(to)} aria-label="Auxesis Capital, home">
      <img src={asset('auxesis-mark-dark.png')} alt="" width={38} height={34} />
      <span class="brand-words"><b>AUXESIS</b><span>CAPITAL</span></span>
    </a>
  );
}

/** Parallax ([data-px]) and the gold scroll-progress hairline under the header. */
export function useScrollEffects(route: string) {
  useEffect(() => {
    let raf = 0;
    const els = () => Array.from(document.querySelectorAll<HTMLElement>('[data-px]'));
    let nodes = els();
    const bar = document.querySelector<HTMLElement>('.header-progress');
    const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
    const run = () => {
      raf = 0;
      const y = window.scrollY;
      const h = document.documentElement.scrollHeight - innerHeight;
      if (bar) bar.style.transform = `scaleX(${h > 0 ? Math.min(1, y / h).toFixed(4) : 0})`;
      if (reduce) return;
      for (const el of nodes) el.style.transform = `translate3d(0,${(y * (parseFloat(el.dataset.px || '0') || 0)).toFixed(1)}px,0)`;
    };
    const onScroll = () => { if (!raf) raf = requestAnimationFrame(run); };
    // Content arrives after data loads; pick up new parallax layers.
    const mo = new MutationObserver(() => { nodes = els(); onScroll(); });
    mo.observe(document.querySelector('main') || document.body, { childList: true, subtree: true });
    addEventListener('scroll', onScroll, { passive: true });
    addEventListener('resize', onScroll);
    run();
    return () => { removeEventListener('scroll', onScroll); removeEventListener('resize', onScroll); mo.disconnect(); cancelAnimationFrame(raf); };
  }, [route]);
}

export function Header() {
  const me = meStore.use();
  const route = routeStore.use();
  const [open, setOpen] = useState(false);
  useEffect(() => setOpen(false), [route]);
  const section = route.startsWith('/publications') ? '/publications' : route.startsWith('/admin') ? '/admin' : route;
  const items: [string, string][] = me
    ? [['Portfolio', '/portfolio'], ['Publications', '/publications'], ['Team', '/team'], ...(me.user.role === 'admin' ? [['Admin', '/admin'] as [string, string]] : [])]
    : [['The Fund', '/'], ['Publications', '/publications'], ['Team', '/team']];
  const name = me?.user.name || me?.investor?.name || 'Investor';

  return (
    <>
      <header class="header">
        <div class="header-progress" />
        <div class="header-inner">
          <Brand to={me ? '/portfolio' : '/'} />
          <nav class="nav" aria-label="Main">
            {items.map(([label, to]) => <a key={to} {...link(to)} class={section === to ? 'on' : ''}>{label}</a>)}
          </nav>
          <div class="header-right">
            {me ? (
              <div class="who">
                <div class="who-text"><div>{name}</div><div>{me.user.email}</div></div>
                <div class="avatar" aria-hidden="true">{initials(name)}</div>
                <button class="signout" onClick={signOut}>SIGN OUT</button>
              </div>
            ) : (
              <a class="btn btn-gold" {...link('/login')}>Investor login</a>
            )}
            <button class="menu-btn" aria-label="Menu" aria-expanded={open} onClick={() => setOpen(!open)}><span /></button>
          </div>
        </div>
      </header>
      <nav class={'mobile-nav' + (open ? ' open' : '')} aria-label="Mobile">
        {items.map(([label, to]) => <a key={to} {...link(to)} class={section === to ? 'on' : ''}>{label}</a>)}
        {!me && <a {...link('/login')} class="gold">Investor login</a>}
      </nav>
    </>
  );
}

export function Footer() {
  const me = meStore.use();
  const year = new Date().getFullYear();
  return (
    <footer class="footer">
      <div class="wrap footer-inner">
        <div class="footer-top">
          <div class="footer-brand">
            <div class="brand" style={{ cursor: 'default' }}>
              <img src={asset('auxesis-mark-dark.png')} alt="" style={{ height: 44 }} />
              <span class="brand-words"><b style={{ fontSize: 14 }}>AUXESIS</b><span style={{ fontSize: 10 }}>CAPITAL</span></span>
            </div>
            <span class="sep" />
            <img src={asset('beta-logo-light.png')} alt="Beta, The Finance & Investments Club" style={{ height: 40, width: 'auto', opacity: 0.85 }} />
          </div>
          <div class="footer-cols">
            <div><span class="label dim">Fund</span><a {...link('/')}>Overview</a><a {...link('/publications')}>Publications</a><a {...link('/team')}>Team</a></div>
            <div><span class="label dim">Investors</span><a {...link(me ? '/portfolio' : '/login')}>{me ? 'Your portfolio' : 'Portal sign-in'}</a><span>Beta, IIM Ahmedabad</span><span>Vastrapur, Ahmedabad 380015</span></div>
          </div>
        </div>
        <div class="footer-bottom">
          <p>Auxesis Capital is a student-run investment fund operated by Beta, the Finance &amp; Investments Club of IIM Ahmedabad. Information on this site is for registered participants and educational purposes and does not constitute an offer or investment advice.</p>
          <span class="mono" style={{ letterSpacing: '.1em' }}>© {year} AUXESIS CAPITAL</span>
        </div>
      </div>
    </footer>
  );
}

/** Photo, or a monogram on the design's hatched placeholder. */
export function Portrait({ photo, name, children, class: cls, eager }: { photo?: Photo; name: string; children?: preact.ComponentChildren; class?: string; eager?: boolean }) {
  const [broken, setBroken] = useState(false);
  const real = name && !/^name surname$/i.test(name.trim());
  return (
    <div class={'portrait ' + (cls || '')}>
      {children}
      {photo && !broken ? (
        <img src={photo.src} srcset={photo.srcset} sizes={photo.sizes} alt={name} loading={eager ? 'eager' : 'lazy'} decoding="async" onError={() => setBroken(true)} />
      ) : real ? (
        <span class="mono-init">{initials(name)}</span>
      ) : (
        <span class="label dim">Portrait</span>
      )}
    </div>
  );
}
