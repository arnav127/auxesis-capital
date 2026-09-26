import { defineConfig, loadEnv } from 'vite';
import preact from '@preact/preset-vite';
import { fileURLToPath } from 'node:url';

// The build lands in backend/pb_public so the PocketBase binary serves the site on campus.
// VITE_BASE: the sub-path the site is served under, e.g. /auxesiscapital/ for https://students.iima.ac.in/auxesiscapital/.
const base = (process.env.VITE_BASE || '/').replace(/\/?$/, '/');

// Link previews (WhatsApp, LinkedIn, Slack) need absolute image URLs, so %SITE_URL% in index.html
// becomes APP_URL from the environment or the repo's .env.
const envDir = fileURLToPath(new URL('..', import.meta.url));

export default defineConfig(({ mode }) => ({
  base,
  root: fileURLToPath(new URL('.', import.meta.url)),
  envDir,
  plugins: [
    preact(),
    {
      name: 'site-url',
      transformIndexHtml(html: string) {
        const site = (process.env.APP_URL || loadEnv(mode, envDir, '').APP_URL || 'https://students.iima.ac.in/auxesiscapital').replace(/\/+$/, '');
        return html.replaceAll('%SITE_URL%', site);
      },
    },
  ],
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
}));
