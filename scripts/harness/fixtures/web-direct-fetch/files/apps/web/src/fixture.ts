export async function loadThing(): Promise<unknown> {
  const res = await fetch('/things/1');
  return res.json();
}
