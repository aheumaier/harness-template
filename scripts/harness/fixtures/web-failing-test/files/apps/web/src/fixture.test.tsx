import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderToString } from 'react-dom/server';
import { expect, test } from 'vitest';

import { App } from './App';

test('renders a heading the app does not have', () => {
  const html = renderToString(
    <QueryClientProvider client={new QueryClient()}>
      <App />
    </QueryClientProvider>,
  );
  expect(html).toContain('fx-missing-heading');
});
