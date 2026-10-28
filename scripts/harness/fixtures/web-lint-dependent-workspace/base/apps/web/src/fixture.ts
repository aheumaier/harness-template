import { problemOf } from '@harness-template/api-client';

export function report(): void {
  problemOf(new Error('x'));
}
