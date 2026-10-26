// Generates TanStack Query hooks + zod schemas from api/openapi.yaml into src/gen (never hand-edit; `make gen`).
// Responses are parsed with zod inside the generated fetchers: data is validated at the boundary.
import { defineConfig } from 'orval';

export default defineConfig({
  api: {
    input: { target: '../../api/openapi.yaml' },
    output: {
      mode: 'tags-split',
      target: 'src/gen/endpoints',
      schemas: { path: 'src/gen/model', type: 'zod' },
      client: 'react-query',
      httpClient: 'fetch',
      clean: true,
      override: {
        // Throw on non-2xx (TanStack error channel) and zod-parse only success bodies.
        fetch: { runtimeValidation: true, forceSuccessResponse: true, includeHttpResponseReturnType: false },
        query: { runtimeValidation: true, signal: true },
      },
    },
  },
});
