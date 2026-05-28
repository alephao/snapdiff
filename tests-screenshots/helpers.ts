import { Page, expect, TestInfo } from '@playwright/test';

/**
 * Append ?noanim=1 (or extend existing query) so the server emits
 * <body class="no-anim"> and the JetBrains Mono caret / blinking
 * dot don't flicker the screenshot.
 */
export function noAnimURL(pathAndQuery: string): string {
  if (pathAndQuery.includes('noanim=')) return pathAndQuery;
  const sep = pathAndQuery.includes('?') ? '&' : '?';
  return `${pathAndQuery}${sep}noanim=1`;
}

/**
 * Wait for embedded webfonts to finish loading before capturing.
 */
export async function awaitFonts(page: Page) {
  await page.evaluate(() => document.fonts.ready);
}

/**
 * Navigate + wait for fonts + small settle delay.
 */
export async function gotoStable(page: Page, pathAndQuery: string) {
  await page.goto(noAnimURL(pathAndQuery), { waitUntil: 'networkidle' });
  await awaitFonts(page);
  // Tiny settle so any layout that depends on font metrics is done.
  await page.waitForTimeout(50);
}

/**
 * Like gotoStable, but for the `snapdiff gallery` server. Uses an absolute
 * URL pulled from SNAPDIFF_GALLERY_URL (set by global-setup), since the
 * default baseURL points at the review-mode server.
 */
export async function gotoStableGallery(page: Page, pathAndQuery: string) {
  const base = process.env.SNAPDIFF_GALLERY_URL;
  if (!base) throw new Error('SNAPDIFF_GALLERY_URL not set — globalSetup did not run?');
  await page.goto(base + noAnimURL(pathAndQuery), { waitUntil: 'networkidle' });
  await awaitFonts(page);
  await page.waitForTimeout(50);
}

/**
 * Snapshot name following snapdiff's expected axis_regex:
 *   <page>.<scenario>.<viewport>.png
 */
export function snap(info: TestInfo, page: string, scenario: string): string {
  return `${page}.${scenario}.${info.project.name}.png`;
}

/**
 * POST a verdict to a specific item via the snapdiff HTTP API.
 * The HTTP form handler redirects to "/", which the caller doesn't care about.
 */
export async function postVerdict(
  baseURL: string,
  id: string,
  status: 'approved' | 'rejected',
  comment = '',
): Promise<void> {
  const body = new URLSearchParams({ status, comment }).toString();
  const r = await fetch(`${baseURL}/diff/${id}/verdict`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body,
    redirect: 'manual',
  });
  if (r.status >= 400) {
    throw new Error(`POST verdict ${id} → ${r.status}`);
  }
}

/**
 * POST a bulk verdict.
 */
export async function postBulk(
  baseURL: string,
  status: 'approved' | 'rejected',
  filters: Record<string, string> = {},
  comment = '',
): Promise<void> {
  const params = new URLSearchParams({ status, comment });
  for (const [k, v] of Object.entries(filters)) params.set(`filter_${k}`, v);
  const r = await fetch(`${baseURL}/diff/bulk-verdict`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: params.toString(),
    redirect: 'manual',
  });
  if (r.status >= 400) {
    throw new Error(`POST bulk → ${r.status}`);
  }
}

export { expect };
