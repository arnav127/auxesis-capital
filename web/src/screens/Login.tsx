import { useEffect, useState } from 'preact/hooks';
import { GoogleG, Spinner } from '../components/Chrome.tsx';
import { asset, finishSignIn, meStore, navigate, startSignIn } from '../lib.ts';

export function Login() {
  const me = meStore.use();
  const params = new URLSearchParams(location.search);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(params.get('error') ?? '');

  useEffect(() => { if (me) navigate(params.get('next') || '/portfolio', true); }, [me]);

  async function go() {
    setBusy(true);
    setError('');
    try {
      await startSignIn(params.get('next') || '/portfolio');
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  }

  return (
    <section class="login">
      <div class="login-art">
        <div class="circles" />
        <div style={{ position: 'relative', display: 'flex', flexDirection: 'column', gap: 26, animation: 'rise 1s .1s both' }}>
          <span class="eyebrow">Auxesis Capital · IIM Ahmedabad</span>
          <h1 class="display">The investor<br /><em class="shimmer">portal.</em></h1>
        </div>
        <div class="feature-row">
          <div><span class="eyebrow" style={{ fontSize: 10.5, letterSpacing: '.14em' }}>Daily</span><span>NAV and your holding value</span></div>
          <div><span class="eyebrow" style={{ fontSize: 10.5, letterSpacing: '.14em' }}>Complete</span><span>Every position, weight and return</span></div>
          <div><span class="eyebrow" style={{ fontSize: 10.5, letterSpacing: '.14em' }}>First</span><span>Letters and research on release</span></div>
        </div>
      </div>
      <div class="login-form">
        <div>
          <img src={asset('auxesis-mark-dark.png')} alt="Auxesis Capital" style={{ height: 64, width: 'auto', alignSelf: 'flex-start' }} />
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <h2 class="display" style={{ fontSize: 44, lineHeight: 1, letterSpacing: '-.01em' }}>Sign in</h2>
            <p style={{ margin: 0, fontSize: 15, lineHeight: 1.6, color: 'var(--muted)' }}>Use the Google account registered with the fund at the time of your investment.</p>
          </div>
          <button class="btn btn-google" onClick={go} disabled={busy}>
            <span class="g"><GoogleG size={17} /></span>{busy ? 'Opening Google…' : 'Continue with Google'}
          </button>
          {error && <div class="alert" role="alert">{error}</div>}
          <div class="notes">
            <div><span>01</span>Sign-in is with Google only. Your password is never shared with Auxesis.</div>
            <div><span>02</span>Only email addresses registered with the fund can sign in.</div>
            <div><span>03</span>Not an investor yet? Write to the Investments Cell at Beta.</div>
          </div>
        </div>
      </div>
    </section>
  );
}

/** Google redirects back here after sign-in. */
export function AuthCallback() {
  useEffect(() => {
    finishSignIn(new URLSearchParams(location.search))
      .then((next) => navigate(next, true))
      .catch((e) => navigate(`/login?error=${encodeURIComponent((e as Error).message)}`, true));
  }, []);
  return <Spinner />;
}
