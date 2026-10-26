// Public surface of the API client. src/gen/** is generated from api/openapi.yaml (`make gen`) and never
// hand-edited; this file is the only hand-written code in the package.
import { Problem, type ProblemOutput } from './gen/model';

export * from './gen/endpoints';
export * from './gen/model';

/**
 * Returns the RFC 9457 problem carried by an error thrown from a generated fetcher or hook, or undefined
 * when the error is not a well-formed problem (network failure, invalid response body).
 */
export function problemOf(error: unknown): ProblemOutput | undefined {
  if (typeof error !== 'object' || error === null || !('info' in error)) return undefined;
  const parsed = Problem.safeParse(error.info);
  return parsed.success ? parsed.data : undefined;
}
