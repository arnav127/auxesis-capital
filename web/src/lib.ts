import { useEffect, useState } from 'preact/hooks';
import * as kit from '@iima/kit';
import { ApiError, BASE, setBase, storage, type Store } from '@iima/kit';
import type { MeView } from '../../shared/types.ts';

// ---------- API ----------

/**
 * The site lives under a sub-path, e.g. https://students.iima.ac.in/auxesiscapital/ (VITE_BASE at build time).
 * BASE is that prefix without the trailing slash ('' at the root). App routes below are written without it.
 * Sign-in is the platform's: the session cookie reaches the backend as a verified token, so
 * nothing here holds a token.
 */
setBase(import.meta.env.BASE_URL);
export { ApiError, BASE, storage };
export const asset = kit.asset;

/** Calls a custom /api/aux route. */
export function api<T>(path: string, init?: { method?: string; body?: unknown }): Promise<T> {
  return kit.api<T>('/api/aux' + path, init);
}

/** Loads a route's data, re-running when deps change. */
export function useApi<T>(path: string | null, deps: unknown[] = []) {
  const [state, setState] = useState<{ data?: T; error?: ApiError; loading: boolean }>({ loading: !!path });
  useEffect(() => {
    if (!path) return;
    let live = true;
    setState((s) => ({ ...s, loading: true }));
    api<T>(path)
      .then((data) => live && setState({ data, loading: false }))
      .catch((error: ApiError) => live && setState({ error, loading: false }));
    return () => { live = false; };
  }, [path, ...deps]);
  return state;
}

// ---------- sign-in (the platform's; works on mobile browsers) ----------

/** Sends the browser to the platform sign-in. `next` is where to land afterwards. */
export async function startSignIn(next = '/portfolio', switchAccount = false) {
  kit.login(next, switchAccount);
}

/** Who is signed in to the platform, even if the fund does not know them (for the login page). */
export const platformUser = () => kit.me('auxesiscapital');

export function signOut() {
  setMe(null);
  kit.logout('/');
}

/** True while the media query matches (e.g. a desktop-width window). */
export function useMedia(query: string) {
  const [on, setOn] = useState(() => matchMedia(query).matches);
  useEffect(() => {
    const m = matchMedia(query);
    const f = () => setOn(m.matches);
    m.addEventListener('change', f);
    return () => m.removeEventListener('change', f);
  }, [query]);
  return on;
}

// ---------- tiny store ----------

/** A kit store with a Preact hook: store.use() re-renders on change. */
function withUse<T>(s: Store<T>) {
  const use = (): T => {
    const [, force] = useState(0);
    const value = s.get();
    useEffect(() => {
      const off = s.subscribe(() => force((n) => n + 1));
      // The store may have changed between this render and subscribing (e.g. refreshMe finishing).
      if (s.get() !== value) force((n) => n + 1);
      return off;
    }, []);
    return value;
  };
  return Object.assign(s, { use });
}

export function createStore<T>(initial: T) {
  return withUse(kit.createStore(initial));
}

/** Shown straight away from the last visit, then confirmed by refreshMe. */
export const meStore = createStore<MeView | null>(storage.get<MeView>('aux:me'));
/** True until the first profile check has finished (so guarded pages don't bounce a signed-in visitor). */
export const mePending = createStore<boolean>(true);

function setMe(me: MeView | null) {
  storage.set('aux:me', me);
  meStore.set(me);
}

export async function refreshMe(): Promise<MeView | null> {
  try {
    const me = await api<MeView>('/me');
    setMe(me);
    return me;
  } catch (e) {
    if (e instanceof ApiError && (e.status === 401 || e.status === 403 || e.status === 404)) setMe(null);
    return meStore.get();
  } finally {
    mePending.set(false);
  }
}

// ---------- router ----------

export const appPath = kit.appPath;
export const routeStore = withUse(kit.routeStore);
export const navigate = kit.navigate;
export const link = kit.link;
kit.startRouter();

// ---------- formatting ----------

const MINUS = '−';

/** +12.3% / −4.5% */
export function pct(x: number | null | undefined, d = 1): string {
  if (x == null || !isFinite(x)) return '—';
  const v = Math.abs(x * 100).toFixed(d);
  if (Number(v) === 0) return (0).toFixed(d) + '%';
  return (x >= 0 ? '+' : MINUS) + v + '%';
}

/** 12.3% without a sign. */
export const pctPlain = (x: number, d = 1) => (isFinite(x) ? (x < 0 ? MINUS : '') + Math.abs(x * 100).toFixed(d) + '%' : '—');

export const signed = (x: number, d = 2) => (x >= 0 ? '+' : MINUS) + Math.abs(x).toFixed(d);

const inr0 = new Intl.NumberFormat('en-IN', { maximumFractionDigits: 0 });
const inr2 = new Intl.NumberFormat('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

/** ₹12,34,567 (Indian grouping). */
export const rupees = (x: number, decimals = 0) => (x < 0 ? MINUS : '') + '₹' + (decimals ? inr2 : inr0).format(Math.abs(x));

/** ₹12.4 L / ₹1.2 Cr */
export function rupeesShort(x: number): string {
  const a = Math.abs(x);
  const s = x < 0 ? MINUS : '';
  if (a >= 1e7) return `${s}₹${(a / 1e7).toFixed(2)} Cr`;
  if (a >= 1e5) return `${s}₹${(a / 1e5).toFixed(2)} L`;
  return s + '₹' + inr0.format(a);
}

export const units = (x: number) => unitsFmt(Math.abs(x) < 5e-4 ? 0 : x);
const unitsFmt = (x: number) => new Intl.NumberFormat('en-IN', { minimumFractionDigits: 3, maximumFractionDigits: 3 }).format(x);

const parseDay = (d: string) => new Date(d + 'T00:00:00');

/** 25 Sep 2026 */
export const fmtDate = (d: string) => (d ? parseDay(d).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' }) : '');
/** Jul ’26 */
export const fmtMonthYear = (d: string) => (d ? parseDay(d).toLocaleDateString('en-GB', { month: 'short' }) + ' ’' + d.slice(2, 4) : '');
/** July 2026 */
export const fmtMonthLong = (d: string) => (d ? parseDay(d).toLocaleDateString('en-GB', { month: 'long', year: 'numeric' }) : '');
export const fmtDayMonth = (d: string) => parseDay(d).toLocaleDateString('en-GB', { day: '2-digit', month: 'short' });

/** "2026–27" for a cycle that began in July 2026. */
export const cycleLabel = (start: string) => (start ? `${start.slice(0, 4)}–${String((Number(start.slice(0, 4)) + 1) % 100).padStart(2, '0')}` : '');
/** When the cycle that began in `start` is liquidated: "February–March 2027". */
export const cycleEnd = (start: string) => (start ? `February–March ${Number(start.slice(0, 4)) + 1}` : 'February–March');

export const UP = '#74c69d';
export const DOWN = '#e08a76';
export const tone = (x: number) => (x >= 0 ? UP : DOWN);

export const initials = (name: string) => name.split(/\s+/).filter(Boolean).map((w) => w[0]).join('').slice(0, 2).toUpperCase();
export const firstName = (name: string) => {
  const w = name.trim().split(/\s+/);
  // Keep a title with the surname: "Prof. A. Bala" → "Prof. Bala".
  if (w.length > 1 && /^(prof|professor|dr|mr|ms|mrs)\.?$/i.test(w[0])) return `${w[0]} ${w[w.length - 1]}`;
  return w[0] || name;
};

export function greeting(): string {
  const h = Number(new Intl.DateTimeFormat('en-GB', { hour: 'numeric', hour12: false, timeZone: 'Asia/Kolkata' }).format(new Date()));
  if (h < 5) return 'Good evening';
  if (h < 12) return 'Good morning';
  if (h < 17) return 'Good afternoon';
  return 'Good evening';
}
