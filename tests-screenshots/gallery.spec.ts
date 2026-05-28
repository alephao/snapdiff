import { test, expect } from '@playwright/test';
import { gotoStableGallery, snap, awaitFonts } from './helpers';

// Gallery scenarios run against a separate snapdiff process spawned by
// global-setup in `gallery` mode. They don't share state with the diff
// review tests, so the order is purely for readability.

test.describe.configure({ mode: 'serial' });

// ───────── INDEX ─────────

test('gallery index populated', async ({ page }, info) => {
  await gotoStableGallery(page, '/');
  await expect(page).toHaveScreenshot(snap(info, 'gallery', 'indexPopulated'), { fullPage: true });
});

test('gallery index filtered', async ({ page }, info) => {
  // Filter by testClass=ProfileViewTests — narrows 19 items to 6 and shows
  // the dropdown-partition behavior (available test/theme/lang values rise
  // to the top with counts; unrelated values fall below the "no matches"
  // divider as disabled).
  await gotoStableGallery(page, '/?filter_testClass=ProfileViewTests');
  await expect(page).toHaveScreenshot(snap(info, 'gallery', 'indexFiltered'), { fullPage: true });
});

// ───────── FOCUSED DETAIL ─────────

test('gallery focused', async ({ page }, info) => {
  // Open the first card. With gitscan's path-sorted ordering this is a
  // deterministic AddGroupFormViewTests item.
  await gotoStableGallery(page, '/');
  await page.locator('a.gcard').first().click();
  await page.waitForLoadState('networkidle');
  await awaitFonts(page);
  await expect(page).toHaveScreenshot(snap(info, 'gallery', 'focused'), { fullPage: true });
});

test('gallery focused missing variant', async ({ page }, info) => {
  // signInSheet only has theme=dark, lang=en. Asking for theme=light hits
  // the placeholder path — verifies the "── no PNG ──" divider, ✗ marker,
  // and the dashed-frame placeholder render.
  await gotoStableGallery(
    page,
    '/missing?ax_testClass=SignInViewTests&ax_test=signInSheet&ax_theme=light&ax_lang=en',
  );
  await expect(page).toHaveScreenshot(snap(info, 'gallery', 'focusedMissing'), { fullPage: true });
});

// ───────── MATRIX ─────────

test('gallery matrix', async ({ page }, info) => {
  // Land on a focused page first so the Matrix tab anchors to a real item;
  // pick a profileSheet item where the (theme × lang) grid is fully filled.
  await gotoStableGallery(page, '/');
  await page.locator('a.gcard').first().click();
  await page.waitForLoadState('networkidle');
  await page.locator('.view-tabs a').filter({ hasText: /Matrix/ }).click();
  await page.waitForLoadState('networkidle');
  await awaitFonts(page);
  await expect(page).toHaveScreenshot(snap(info, 'gallery', 'matrix'), { fullPage: true });
});
