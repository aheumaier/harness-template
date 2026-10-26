import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

// Ports come from the worktree's .env.local (scripts/setup-worktree.sh) so worktrees never collide.
const apiPort = process.env.API_PORT ?? '8080';
const webPort = Number(process.env.WEB_PORT ?? '5173');

export default defineConfig({
  plugins: [react()],
  server: {
    port: webPort,
    strictPort: true,
    // Same-origin API calls: the generated client uses relative paths (e.g. /healthz).
    proxy: { '/healthz': `http://localhost:${apiPort}` },
  },
  test: { environment: 'node' },
});
