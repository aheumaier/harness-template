// Harness file (protected). Frontend module boundaries. docs/harness/rules.md#web-*
/** @type {import('dependency-cruiser').IConfiguration} */
module.exports = {
  forbidden: [
    {
      name: 'web-domain-pure',
      comment:
        '[web-domain-pure] packages/domain must not import React, TanStack, the api-client or any app. Fix: keep domain logic pure; do the wiring in apps/web and pass data in.',
      severity: 'error',
      from: { path: '^packages/domain/' },
      to: {
        path: [
          'node_modules/(react|react-dom|scheduler|@tanstack)/', // also matches pnpm's .pnpm/<pkg>/node_modules/<pkg>/
          '^(react|react-dom|scheduler)(/|$)', // unresolved (not a declared dependency) — still forbidden
          '^@tanstack/',
          '^@harness-template/(?!domain)',
          '^packages/api-client/',
          '^apps/',
        ],
      },
    },
    {
      name: 'web-no-cross-app',
      comment:
        '[web-no-cross-app] An app must not import another app. Fix: move the shared code to packages/ (pure logic to packages/domain) and import it from there.',
      severity: 'error',
      from: { path: '^apps/([^/]+)/' },
      to: { path: '^apps/', pathNot: '^apps/$1/' },
    },
  ],
  options: {
    doNotFollow: { path: 'node_modules' },
    exclude: { path: '(^|/)(dist|coverage)/' },
    tsPreCompilationDeps: true,
    tsConfig: { fileName: 'tsconfig.base.json' },
    enhancedResolveOptions: {
      exportsFields: ['exports'],
      conditionNames: ['import', 'require', 'node', 'default', 'types'],
      extensions: ['.ts', '.tsx', '.js', '.mjs', '.cjs', '.json'],
    },
  },
};
