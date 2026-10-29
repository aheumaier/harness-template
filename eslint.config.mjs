// Harness file (protected). TypeScript lint + frontend boundary rules. docs/harness/rules.md#web-*
import js from '@eslint/js';
import reactHooks from 'eslint-plugin-react-hooks';
import tseslint from 'typescript-eslint';

const directHttp = {
  message:
    '[web-no-direct-http] Call the API only through generated hooks. Fix: add the operation to api/openapi.yaml, run `make gen`, and use use<OperationId> from @harness-template/api-client (e.g. useGetHealth).',
};

export default tseslint.config(
  { ignores: ['**/node_modules/**', '**/dist/**', '**/coverage/**', 'packages/api-client/src/gen/**', 'apps/api/**', '.bin/**', 'scripts/harness/fixtures/**'] },
  { linterOptions: { reportUnusedDisableDirectives: 'error' } },
  js.configs.recommended,
  tseslint.configs.strictTypeChecked,
  {
    languageOptions: { parserOptions: { projectService: true, tsconfigRootDir: import.meta.dirname } },
  },
  { files: ['**/*.{js,mjs,cjs}'], extends: [tseslint.configs.disableTypeChecked] },
  { files: ['**/*.cjs'], languageOptions: { sourceType: 'commonjs' } },
  { files: ['apps/web/**/*.{ts,tsx}'], extends: [reactHooks.configs.flat['recommended-latest']] },

  // web-no-direct-http: only the generated client talks HTTP.
  {
    files: ['**/*.{ts,tsx}'],
    ignores: ['packages/api-client/**'],
    rules: {
      'no-restricted-globals': ['error', { name: 'fetch', ...directHttp }, { name: 'XMLHttpRequest', ...directHttp }],
      'no-restricted-properties': [
        'error',
        { object: 'window', property: 'fetch', ...directHttp },
        { object: 'globalThis', property: 'fetch', ...directHttp },
      ],
      'no-restricted-imports': [
        'error',
        {
          paths: ['axios', 'ky', 'ofetch', 'node-fetch', 'got', 'superagent'].map((name) => ({ name, ...directHttp })),
          patterns: [
            {
              group: ['@harness-template/api-client/*'],
              message: '[web-no-direct-http] Import from the package entry @harness-template/api-client, not its internals.',
            },
          ],
        },
      ],
    },
  },

  // ADR 0024 size and complexity limits (rules.md#code-function-length, #code-complexity, #code-parameters,
  // #code-file-length). ESLint 10.13's documented defaults while the web code is a scaffold; they only go down.
  // Every line counts (skipBlankLines and skipComments false, as in apps/api/.golangci.yml).
  {
    rules: {
      'max-lines-per-function': ['error', { max: 50, skipBlankLines: false, skipComments: false }],
      complexity: ['error', 20],
      'max-params': ['error', 3],
      'max-lines': ['error', { max: 300, skipBlankLines: false, skipComments: false }],
    },
  },
  // Tests, Playwright journeys included, are exempt from the per-function limits and stay under max-lines.
  {
    files: ['**/*.test.{ts,tsx}', '**/*.e2e.ts'],
    rules: { 'max-lines-per-function': 'off', complexity: 'off', 'max-params': 'off' },
  },

  // web-domain-pure: packages/domain runs anywhere (tests, workers, server) — no browser globals.
  {
    files: ['packages/domain/**/*.ts'],
    rules: {
      'no-restricted-globals': [
        'error',
        ...['window', 'document', 'navigator', 'location', 'localStorage', 'sessionStorage', 'fetch'].map((name) => ({
          name,
          message: `[web-domain-pure] packages/domain must not use browser globals (${name}). Fix: pass the value in as a parameter from apps/web.`,
        })),
      ],
    },
  },
);
