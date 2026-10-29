import { afterEach, expect, test, vi } from 'vitest';

import globalSetup from './global-setup';

// make e2e must fail, never skip, when it runs without this worktree's env (rules.md#web-e2e-journeys).
afterEach(() => {
  vi.unstubAllEnvs();
});

test.each([
  { name: 'neither', web: undefined, api: undefined },
  { name: 'no HTTP_ADDR', web: '5173', api: undefined },
  { name: 'no WEB_PORT', web: undefined, api: '127.0.0.1:8080' },
])('fails the run without the worktree env ($name)', ({ web, api }) => {
  vi.stubEnv('WEB_PORT', web);
  vi.stubEnv('HTTP_ADDR', api);
  expect(() => {
    globalSetup();
  }).toThrow('run this through make e2e');
});

test('passes with both', () => {
  vi.stubEnv('WEB_PORT', '5173');
  vi.stubEnv('HTTP_ADDR', '127.0.0.1:8080');
  expect(() => {
    globalSetup();
  }).not.toThrow();
});
