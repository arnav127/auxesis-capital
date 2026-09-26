import type { Member } from '../../../shared/types.ts';
import { Portrait, Spinner } from '../components/Chrome.tsx';
import { asset, pb, useApi } from '../lib.ts';

const COHORTS = [
  { key: 'pgp2', label: 'PGP2 · Senior analysts' },
  { key: 'pgp1', label: 'PGP1 · Analysts' },
] as const;

/** The desk's year: July onwards is the new cycle. */
const deskYear = () => {
  const d = new Date();
  const y = d.getMonth() >= 6 ? d.getFullYear() : d.getFullYear() - 1;
  return `${y}–${String((y + 1) % 100).padStart(2, '0')}`;
};

const photoUrl = (m: Member, thumb: string) => (m.photo ? `${pb.baseURL}${m.photo}?thumb=${thumb}` : '');

export function Team() {
  const { data, loading, error } = useApi<Member[]>('/team');
  const leaders = (data || []).filter((m) => m.group === 'leadership');

  return (
    <div>
      <section class="wrap team-hero">
        <div class="team-ghost" data-px="0.3" aria-hidden="true">the desk</div>
        <div class="pub-head" style={{ position: 'relative' }}>
          <div>
            <span class="eyebrow">The investing team · {deskYear()}</span>
            <h1 class="display h-xl">The people<br /><em class="gold-em">behind the fund.</em></h1>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 18, maxWidth: 480 }}>
            <p class="lede">Auxesis Capital is managed by students of the Post Graduate Programme at IIM Ahmedabad, under Beta, the Finance &amp; Investments Club. The desk changes each year as one batch graduates and the next takes over.</p>
            <img src={asset('beta-logo-light.png')} alt="Beta, The Finance & Investments Club" style={{ height: 54, width: 'auto', alignSelf: 'flex-start', opacity: 0.9 }} />
          </div>
        </div>
      </section>

      {loading && !data && <Spinner />}
      {error && <div class="wrap"><div class="alert">{error.message}</div></div>}

      {leaders.length > 0 && (
        <section class="wrap" style={{ paddingBottom: 96 }}>
          <div class="group-head">Leadership</div>
          <div class="leaders">
            {leaders.map((l, i) => (
              <div class="leader" key={l.id}>
                <Portrait name={l.name} photo={photoUrl(l, '600x750')}><span class="idx">{String(i + 1).padStart(2, '0')}</span></Portrait>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  <span class="eyebrow" style={{ fontSize: 10.5, letterSpacing: '.16em' }}>{l.role}</span>
                  <span class="nm">{l.name}</span>
                  <span style={{ fontSize: 13.5, color: 'var(--muted)' }}>
                    {[l.org, l.batch].filter(Boolean).join(' · ')}
                    {l.linkedin && <> · <a href={l.linkedin} target="_blank" rel="noopener">LinkedIn</a></>}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      {COHORTS.map((c) => {
        const members = (data || []).filter((m) => m.group === c.key);
        if (!members.length) return null;
        return (
          <section class="wrap" style={{ paddingBottom: 96 }} key={c.key}>
            <div class="group-head"><span>{c.label}</span><span class="dim">{members.length} members</span></div>
            <div class="members">
              {members.map((m) => (
                <div class="member" key={m.id}>
                  <Portrait name={m.name} photo={photoUrl(m, '400x400')} />
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
                    <span style={{ fontSize: 16, fontWeight: 500 }}>{m.linkedin ? <a href={m.linkedin} target="_blank" rel="noopener" style={{ color: 'inherit' }}>{m.name}</a> : m.name}</span>
                    <span style={{ fontSize: 13, color: 'var(--muted)' }}>{m.role}</span>
                  </div>
                </div>
              ))}
            </div>
          </section>
        );
      })}
    </div>
  );
}
