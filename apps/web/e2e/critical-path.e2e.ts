import { expect, test } from '@playwright/test';

// The app's critical path against this worktree's make dev (NFR-B2, rules.md#web-e2e-journeys).
test('critical path: the API is healthy and the web shell shows it', async ({ page, request }) => {
  await test.step('GET /healthz answers ok and echoes its X-Request-Id', async () => {
    const id = `e2e-${String(Date.now())}`;
    const res = await request.get(`http://${process.env.HTTP_ADDR ?? ''}/healthz`, { headers: { 'X-Request-Id': id } });
    expect(res.status()).toBe(200);
    expect(res.headers()['x-request-id']).toBe(id);
    expect(await res.json()).toEqual({ status: 'ok' });
  });

  await test.step('the web shell renders the API status without errors', async () => {
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(e.message));
    page.on('console', (m) => {
      if (m.type() === 'error') errors.push(m.text());
    });
    await page.goto('/');
    await expect(page.getByRole('heading', { level: 1 })).toHaveText('harness-template');
    await expect(page.getByText('API: ok')).toBeVisible();
    expect(errors).toEqual([]);
  });
});
