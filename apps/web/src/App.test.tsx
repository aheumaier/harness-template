import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderToString } from 'react-dom/server';
import { expect, test } from 'vitest';

import { App } from './App';

test('renders the shell before the health check resolves', () => {
  const html = renderToString(
    <QueryClientProvider client={new QueryClient()}>
      <App />
    </QueryClientProvider>,
  );
  expect(html).toContain('harness-template');
  expect(html).toContain('checking…');
});
