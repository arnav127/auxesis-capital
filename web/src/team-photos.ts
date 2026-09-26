import type { Member } from '../../shared/types.ts';
import { pb } from './lib.ts';

/**
 * Team portraits bundled with the site, cropped and encoded as WebP in two widths each:
 * `<slug>-portrait-{480,960}.webp` (4:5, leaders) and `<slug>-square-{320,640}.webp` (1:1, analysts).
 * Vite fingerprints them under /assets/, which the server caches for a year (immutable).
 * A photo uploaded to the member's `team` record takes precedence.
 */
const files = import.meta.glob<string>('./assets/team/*.webp', { eager: true, query: '?url', import: 'default' });

const byName: Record<string, string> = {};
for (const [path, url] of Object.entries(files)) byName[path.slice(path.lastIndexOf('/') + 1, -'.webp'.length)] = url;

const slug = (name: string) => name.trim().toLowerCase().normalize('NFKD').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');

export type PhotoShape = 'portrait' | 'square';

const SIZES: Record<PhotoShape, [number, number, string]> = {
  portrait: [480, 960, '(max-width: 700px) 92vw, 380px'],
  square: [320, 640, '(max-width: 520px) 92vw, 240px'],
};

export interface Photo { src: string; srcset?: string; sizes?: string }

export function teamPhoto(m: Member, shape: PhotoShape): Photo | undefined {
  if (m.photo) return { src: `${pb.baseURL}${m.photo}?thumb=${shape === 'portrait' ? '600x750' : '400x400'}` };
  const [sm, lg, sizes] = SIZES[shape];
  const s = slug(m.name);
  const a = byName[`${s}-${shape}-${sm}`];
  const b = byName[`${s}-${shape}-${lg}`];
  if (!a) return undefined;
  return b ? { src: a, srcset: `${a} ${sm}w, ${b} ${lg}w`, sizes } : { src: a };
}
