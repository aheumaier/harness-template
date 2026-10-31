import { expect, test } from 'vitest';

import { App } from './App';
import { renderWithProviders } from './renderWithProviders';

test('renders the shell before the health check resolves', () => {
  const html = renderWithProviders(<App />);
  expect(html).toContain('harness-template');
  expect(html).toContain('checking…');
});
