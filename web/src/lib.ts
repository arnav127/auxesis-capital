import { useEffect, useState } from 'preact/hooks';
import PocketBase, { ClientResponseError } from 'pocketbase';
import type { MeView } from '../../shared/types.ts';

// ---------- API ----------

/**
 * The site can live under a sub-path, e.g. https://students.iima.ac.in/auxesiscapital/ (VITE_BASE at build time).
 * BASE is that prefix without the trailing slash ('' at the root). App routes below are written without it.
 */
export const BASE = import.meta.env.BASE_URL.replace(/\/$/, '');
export const asset = (file: string) => `${BASE}/${file}`;

/** PocketBase serves the site and the API from the same origin and path. */
export const pb = new PocketBase(import.meta.env.VITE_PB_URL || location.origin + BASE);
pb.autoCancellation(false);

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

function toApiError(e: unknown): ApiError {
  if (e instanceof ClientResponseError) {
    if (e.status === 0) return new ApiError(0, "You're offline. Check your connection and try again.");
    return new ApiError(e.status, e.response?.message || e.message || 'Something went wrong');
  }
  return new ApiError(500, e instanceof Error ? e.message : 'Something went wrong');
}

/** Calls a custom /api/aux route. */
export async function api<T>(path: string, init?: { method?: string; body?: unknown }): Promise<T> {
  try {
    return await pb.send<T>('/api/aux' + path, { method: init?.method ?? (init?.body !== undefined ? 'POST' : 'GET'), body: init?.body });
  } catch (e) {
    throw toApiError(e);
  }
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

// ---------- sign-in (Google OAuth2 redirect flow; works on mobile browsers) ----------

const OAUTH_KEY = 'aux:oauth';
export const oauthRedirect = () => `${location.origin}${BASE}/auth/callback`;

/** Sends the browser to Google. `next` is where to land afterwards. */
export async function startSignIn(next = '/portfolio') {
  let methods;
  try {
    methods = await pb.collection('users').listAuthMethods();
  } catch (e) {
    throw toApiError(e);
  }
  const p = methods.oauth2?.providers?.find((x) => x.name === 'google');
  if (!methods.oauth2?.enabled || !p) throw new ApiError(503, 'Sign-in is not set up yet. The fund needs to add its Google keys.');
  storage.set(OAUTH_KEY, { state: p.state, verifier: p.codeVerifier, next });
  location.href = p.authURL + encodeURIComponent(oauthRedirect());
}

export async function finishSignIn(params: URLSearchParams): Promise<string> {
  const saved = storage.get<{ state: string; verifier: string; next: string }>(OAUTH_KEY);
  storage.set(OAUTH_KEY, null);
  if (params.get('error')) throw new ApiError(400, 'Sign-in was cancelled.');
  if (!saved || saved.state !== params.get('state')) throw new ApiError(400, 'Sign-in expired. Please try again.');
  try {
    await pb.collection('users').authWithOAuth2Code('google', params.get('code') ?? '', saved.verifier, oauthRedirect());
  } catch (e) {
    throw toApiError(e);
  }
  await refreshMe();
  return saved.next || '/portfolio';
}

export function signOut() {
  pb.authStore.clear();
  setMe(null);
  navigate('/');
}

// ---------- tiny store ----------

type Listener = () => void;

export function createStore<T>(initial: T) {
  let value = initial;
  const listeners = new Set<Listener>();
  return {
    get: () => value,
    set(next: T) { value = next; listeners.forEach((l) => l()); },
    use(): T {
      const [, force] = useState(0);
      useEffect(() => { const l = () => force((n) => n + 1); listeners.add(l); return () => { listeners.delete(l); }; }, []);
      return value;
    },
  };
}

export const storage = {
  get<T>(key: string): T | null {
    try { const v = localStorage.getItem(key); return v ? (JSON.parse(v) as T) : null; } catch { return null; }
  },
  set(key: string, v: unknown) {
    try { if (v === null) localStorage.removeItem(key); else localStorage.setItem(key, JSON.stringify(v)); } catch { /* private mode */ }
  },
};

export const meStore = createStore<MeView | null>(pb.authStore.isValid ? storage.get<MeView>('aux:me') : null);
/** True until the first profile check has finished (so guarded pages don't bounce a signed-in visitor). */
export const mePending = createStore<boolean>(pb.authStore.isValid && !meStore.get());

function setMe(me: MeView | null) {
  storage.set('aux:me', me);
  meStore.set(me);
}

export async function refreshMe(): Promise<MeView | null> {
  try {
    if (!pb.authStore.isValid) {
      setMe(null);
      return null;
    }
    const me = await api<MeView>('/me');
    setMe(me);
    return me;
  } catch (e) {
    if (e instanceof ApiError && (e.status === 401 || e.status === 403 || e.status === 404)) {
      pb.authStore.clear();
      setMe(null);
    }
    return meStore.get();
  } finally {
    mePending.set(false);
  }
}

/** Renews the session token on start so investors stay signed in. */
export async function renewSession() {
  if (!pb.authStore.isValid) return;
  try {
    await pb.collection('users').authRefresh();
  } catch (e) {
    if (e instanceof ClientResponseError && (e.status === 401 || e.status === 403 || e.status === 404)) {
      pb.authStore.clear();
      setMe(null);
    }
  }
}

// ---------- router ----------

export const appPath = () => location.pathname.slice(BASE.length) || '/';
export const routeStore = createStore(appPath());

export function navigate(to: string, replace = false) {
  if (to === appPath() + location.search) return;
  if (replace) history.replaceState(null, '', BASE + to);
  else history.pushState(null, '', BASE + to);
  routeStore.set(appPath());
  window.scrollTo(0, 0);
}

addEventListener('popstate', () => routeStore.set(appPath()));

/** Props for an <a> that navigates inside the app (and still opens in a new tab on cmd-click). */
export function link(to: string) {
  return {
    href: BASE + to,
    onClick: (e: MouseEvent) => {
      if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
      e.preventDefault();
      navigate(to);
    },
  };
}

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
export const firstName = (name: string) => name.split(/\s+/)[0] || name;

export function greeting(): string {
  const h = Number(new Intl.DateTimeFormat('en-GB', { hour: 'numeric', hour12: false, timeZone: 'Asia/Kolkata' }).format(new Date()));
  if (h < 5) return 'Good evening';
  if (h < 12) return 'Good morning';
  if (h < 17) return 'Good afternoon';
  return 'Good evening';
}
