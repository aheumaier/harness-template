import type { ReactNode } from 'react';
import { renderToString } from 'react-dom/server';

import { AppProviders } from './AppProviders';

// Test helper: renders ui to HTML inside the providers main.tsx gives the app.
export function renderWithProviders(ui: ReactNode): string {
  return renderToString(<AppProviders>{ui}</AppProviders>);
}
