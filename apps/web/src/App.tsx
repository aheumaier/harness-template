import { problemOf, useGetHealth } from '@harness-template/api-client';

export function App() {
  const health = useGetHealth();

  let status: string;
  if (health.isPending) status = 'checking…';
  else if (health.isError) status = problemOf(health.error)?.title ?? 'unreachable';
  else status = health.data.status;

  return (
    <main>
      <h1>harness-template</h1>
      <p>API: {status}</p>
    </main>
  );
}
