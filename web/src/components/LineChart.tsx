import { useMemo, useState } from 'preact/hooks';
import { fmtDate, fmtDayMonth, fmtMonthYear } from '../lib.ts';

export interface Pt { d: string; f: number; b?: number }

const W = 1000;
const H = 320;

/** Paths for a fund line (gold), a benchmark line (dashed) and the gold area under the fund. */
export function useChart(points: Pt[]) {
  return useMemo(() => {
    const n = points.length;
    if (n < 2) return null;
    const vals = points.flatMap((p) => (p.b != null ? [p.f, p.b] : [p.f]));
    let mn = Math.min(...vals), mx = Math.max(...vals);
    if (mx - mn < 1e-9) { mn -= 1; mx += 1; }
    const pad = (mx - mn) * 0.08;
    mn -= pad; mx += pad;
    const X = (i: number) => (i / (n - 1)) * W;
    const Y = (v: number) => H - ((v - mn) / (mx - mn)) * H;
    const f = points.map((p, i) => `${i ? 'L' : 'M'}${X(i).toFixed(1)} ${Y(p.f).toFixed(1)}`).join(' ');
    let b = '';
    let started = false;
    points.forEach((p, i) => {
      if (p.b == null) { started = false; return; }
      b += `${started ? 'L' : 'M'}${X(i).toFixed(1)} ${Y(p.b).toFixed(1)} `;
      started = true;
    });
    const long = n > 80;
    const xLabels = [0, 1, 2, 3, 4].map((k) => {
      const d = points[Math.round((k / 4) * (n - 1))].d;
      return long ? fmtMonthYear(d) : fmtDayMonth(d);
    });
    return { f, b, area: `${f} L${W} ${H} L0 ${H} Z`, mn, mx, X, Y, xLabels, n };
  }, [points]);
}

interface Props {
  points: Pt[];
  height: string | number;
  yFormat?: (v: number) => string;
  /** Hover readout labels; omit to disable hover. */
  tip?: { fund: string; bench: string; format: (v: number) => string };
  endDot?: boolean;
  id: string;
}

/** The fund against its benchmark, drawn like the design: gold line, gold wash, dashed ivory benchmark. */
export function LineChart({ points, height, yFormat, tip, endDot, id }: Props) {
  const c = useChart(points);
  const [hover, setHover] = useState<number | null>(null);
  if (!c) return <div style={{ height }} class="chart" />;
  const ticks = [0.15, 0.4, 0.65, 0.9].map((t) => ({ top: t * 100 + '%', v: c.mx - t * (c.mx - c.mn) }));
  const pct = (v: number) => (c.Y(v) / H) * 100 + '%';

  const move = (clientX: number, el: HTMLElement) => {
    const r = el.getBoundingClientRect();
    const i = Math.max(0, Math.min(c.n - 1, Math.round(((clientX - r.left) / r.width) * (c.n - 1))));
    if (i !== hover) setHover(i);
  };
  const h = hover != null && hover < points.length ? points[hover] : null;
  const lp = hover != null ? hover / (c.n - 1) : 0;

  return (
    <div class="chart" style={{ height }}>
      {yFormat && ticks.map((t) => (
        <div class="grid" style={{ top: t.top }} key={t.top}><span>{yFormat(t.v)}</span></div>
      ))}
      <svg class="plot" viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stop-color="#D1B27A" stop-opacity=".2" />
            <stop offset="1" stop-color="#D1B27A" stop-opacity="0" />
          </linearGradient>
        </defs>
        <path d={c.area} fill={`url(#${id})`} />
        {c.b && <path d={c.b} fill="none" stroke="#A7B0C0" stroke-width="1" stroke-dasharray="4 4" vector-effect="non-scaling-stroke" />}
        <path d={c.f} fill="none" stroke="#D1B27A" stroke-width="2" vector-effect="non-scaling-stroke" />
      </svg>
      {endDot && <div class="end-dot" style={{ top: pct(points[points.length - 1].f) }} />}
      {tip && h && (
        <>
          <div class="hover-line" style={{ left: lp * 100 + '%' }} />
          <div class="dot dot-f" style={{ left: lp * 100 + '%', top: pct(h.f) }} />
          {h.b != null && <div class="dot dot-b" style={{ left: lp * 100 + '%', top: pct(h.b) }} />}
          <div class="tip" style={{ left: lp * 100 + '%', transform: `translateX(${lp > 0.62 ? 'calc(-100% - 14px)' : '14px'})` }}>
            <span class="muted" style={{ letterSpacing: '.08em' }}>{fmtDate(h.d).toUpperCase()}</span>
            <span><span class="gold">{tip.fund}</span><span>{tip.format(h.f)}</span></span>
            {h.b != null && <span><span class="muted">{tip.bench}</span><span>{tip.format(h.b)}</span></span>}
          </div>
        </>
      )}
      {tip && (
        <div
          class="catcher"
          onMouseMove={(e) => move(e.clientX, e.currentTarget as HTMLElement)}
          onMouseLeave={() => setHover(null)}
          onTouchStart={(e) => move(e.touches[0].clientX, e.currentTarget as HTMLElement)}
          onTouchMove={(e) => move(e.touches[0].clientX, e.currentTarget as HTMLElement)}
          onTouchEnd={() => setHover(null)}
        />
      )}
      {yFormat && <div class="xlabels">{c.xLabels.map((x, i) => <span key={i}>{x}</span>)}</div>}
    </div>
  );
}

/** The decorative hero line (no axes). */
export function HeroLine({ points }: { points: Pt[] }) {
  const c = useChart(points);
  if (!c) return null;
  return (
    <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" aria-hidden="true">
      <defs>
        <linearGradient id="heroFill" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stop-color="#D1B27A" stop-opacity=".22" />
          <stop offset="1" stop-color="#D1B27A" stop-opacity="0" />
        </linearGradient>
      </defs>
      <path d={c.area} fill="url(#heroFill)" />
      {c.b && <path d={c.b} fill="none" stroke="rgba(241,234,219,.22)" stroke-width="1" stroke-dasharray="3 5" vector-effect="non-scaling-stroke" />}
      <path d={c.f} fill="none" stroke="#D1B27A" stroke-width="1.5" vector-effect="non-scaling-stroke" />
    </svg>
  );
}
