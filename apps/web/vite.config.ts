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
  test: {
    environment: 'node',
    // `pnpm coverage` (make coverage): scripts/harness/coverage.sh sets the provider and reporter. Include every
    // source file, so a file no test imports counts as uncovered instead of missing (test files are excluded by
    // Vitest's default). No `thresholds`: coverage is a trend, not a gate
    // (docs/adr/0020-test-coverage-is-a-trend.md).
    coverage: { include: ['src/**'] },
  },
});
