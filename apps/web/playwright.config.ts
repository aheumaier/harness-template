import { defineConfig, devices } from '@playwright/test';

// make e2e (rules.md#web-e2e-journeys, NFR-B2). The ports are this worktree's, from the env make e2e loads.
// Without them there is no server to start or attach to, and e2e/global-setup.ts fails the run; the config
// itself still loads, so that knip can read it.
const webPort = process.env.WEB_PORT;
const hasEnv = Boolean(webPort && process.env.HTTP_ADDR);

export default defineConfig({
  testDir: 'e2e',
  testMatch: '*.e2e.ts', // not *.test.ts or *.spec.ts: Vitest (make test-web) collects those
  globalSetup: './e2e/global-setup.ts',
  outputDir: '../../.tmp/e2e', // a failed journey's screenshot and trace
  forbidOnly: true,
  retries: 0,
  workers: 1,
  reporter: 'line',
  use: { ...devices['Desktop Chrome'], baseURL: `http://localhost:${webPort ?? ''}`, screenshot: 'only-on-failure', trace: 'retain-on-failure' },
  webServer: hasEnv
    ? {
        // Attach to this worktree's make dev, else start one (its output also lands in .tmp/dev/) and stop it after.
        // The calling make's flags (-k, -j, its jobserver) must not reach it.
        command: `${process.env.MAKE ?? 'make'} -C ../.. dev`,
        env: { MAKEFLAGS: '', MFLAGS: '' },
        // Vite proxies /healthz to the API, so this answers 200 only once both are up and the database answers.
        url: `http://localhost:${webPort ?? ''}/healthz`,
        // make check-all (HARNESS_SCOPE=full) never takes its verdict from a make dev built before the latest
        // edit (ADR 0014): it starts its own, and fails ("already used") while another one runs.
        reuseExistingServer: process.env.HARNESS_SCOPE !== 'full',
        timeout: 120_000, // a cold `go run` compiles the API first
        gracefulShutdown: { signal: 'SIGTERM', timeout: 10_000 },
      }
    : undefined,
});
