// Fails the run before any journey when it did not come through make e2e, which loads this worktree's ports.
export default function globalSetup(): void {
  if (!process.env.WEB_PORT || !process.env.HTTP_ADDR) {
    throw new Error('no WEB_PORT or HTTP_ADDR: run this through make e2e (after make setup)');
  }
}
