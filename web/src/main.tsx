import { render } from 'preact';
import { useEffect } from 'preact/hooks';
import '@fontsource/instrument-serif/400.css';
import '@fontsource/instrument-serif/400-italic.css';
import '@fontsource-variable/source-sans-3/index.css';
import '@fontsource-variable/source-serif-4/index.css';
import '@fontsource-variable/source-serif-4/wght-italic.css';
import '@fontsource/ibm-plex-mono/400.css';
import '@fontsource/ibm-plex-mono/500.css';
import './styles.css';
import { Footer, Header, Spinner, useScrollEffects } from './components/Chrome.tsx';
import { link, meStore, navigate, refreshMe, renewSession, routeStore } from './lib.ts';
import { Admin } from './screens/Admin.tsx';
import { Article } from './screens/Article.tsx';
import { Home } from './screens/Home.tsx';
import { AuthCallback, Login } from './screens/Login.tsx';
import { Portfolio } from './screens/Portfolio.tsx';
import { Reports } from './screens/Reports.tsx';
import { Team } from './screens/Team.tsx';

const TITLES: Record<string, string> = {
  '/': 'Auxesis Capital · IIM Ahmedabad',
  '/login': 'Investor sign-in · Auxesis Capital',
  '/portfolio': 'Portfolio · Auxesis Capital',
  '/publications': 'Letters & research · Auxesis Capital',
  '/team': 'The team · Auxesis Capital',
  '/admin': 'Admin · Auxesis Capital',
};

function RequireAuth({ admin, children }: { admin?: boolean; children: preact.ComponentChildren }) {
  const me = meStore.use();
  const route = routeStore.use();
  useEffect(() => {
    if (!me) navigate('/login?next=' + encodeURIComponent(route), true);
    else if (admin && me.user.role !== 'admin') navigate('/portfolio', true);
  }, [me, route]);
  if (!me || (admin && me.user.role !== 'admin')) return <Spinner />;
  return <>{children}</>;
}

function NotFound() {
  return (
    <div class="wrap page-top" style={{ paddingBottom: 140, display: 'flex', flexDirection: 'column', gap: 24, alignItems: 'flex-start' }}>
      <span class="eyebrow">404</span>
      <h1 class="display h-xl">Not<br /><em class="gold-em">here.</em></h1>
      <a class="btn btn-ghost" {...link('/')}>Back to the fund</a>
    </div>
  );
}

function App() {
  const route = routeStore.use();
  useScrollEffects(route);
  const article = route.match(/^\/publications\/([a-z0-9-]+)\/?$/);
  useEffect(() => {
    document.title = TITLES[route] || (article ? 'Publications · Auxesis Capital' : 'Auxesis Capital');
  }, [route]);

  let page;
  if (route === '/') page = <Home />;
  else if (route === '/login') page = <Login />;
  else if (route === '/auth/callback') page = <AuthCallback />;
  else if (route === '/portfolio') page = <RequireAuth><Portfolio /></RequireAuth>;
  else if (route === '/publications') page = <Reports />;
  else if (article) page = <Article key={article[1]} slug={article[1]} />;
  else if (route === '/team') page = <Team />;
  else if (route === '/admin') page = <RequireAuth admin><Admin /></RequireAuth>;
  else page = <NotFound />;

  const bare = route === '/login' || route === '/auth/callback';
  return (
    <div class="shell">
      <div class="shell-glow" />
      <Header />
      <main>{page}</main>
      {!bare && <Footer />}
    </div>
  );
}

renewSession().then(refreshMe);
render(<App />, document.getElementById('app')!);
