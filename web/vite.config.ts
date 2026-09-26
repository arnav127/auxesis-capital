import { defineConfig } from 'vite';
import preact from '@preact/preset-vite';
import { fileURLToPath } from 'node:url';

// The build lands in backend/pb_public so the PocketBase binary serves the site on campus.
// VITE_BASE: the sub-path the site is served under, e.g. /auxesiscapital/ for https://students.iima.ac.in/auxesiscapital/.
const base = (process.env.VITE_BASE || '/').replace(/\/?$/, '/');

export default defineConfig({
  base,
  root: fileURLToPath(new URL('.', import.meta.url)),
  envDir: fileURLToPath(new URL('..', import.meta.url)),
  plugins: [preact()],
  build: {
    outDir: fileURLToPath(new URL('../backend/pb_public', import.meta.url)),
    emptyOutDir: true,
    target: 'es2020',
    assetsInlineLimit: 0,
  },
  server: {
    port: 5173,
    host: true,
    // Dev: PocketBase runs at the root, so strip the base before proxying /api.
    proxy: { [`${base}api`]: { target: 'http://127.0.0.1:8090', rewrite: (p) => p.slice(base.length - 1) } },
  },
});
