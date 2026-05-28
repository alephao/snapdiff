import { test, expect } from '@playwright/test';
import { gotoStable, snap, postVerdict } from './helpers';

// All tests live in this single file so we can rely on declaration
// order. Tests that don't mutate verdict state run first; tests that
// do come last and are explicit about which IDs they touch.
//
// Fixture IDs (gitscan sorts by path; see fixtures/build_fixture):
//   0–7   AddGroupFormViewTests / formViewSignedIn{,Excluded} × 4 (modified)
//   8–11  ProfileViewTests / profileSheet × 4 (modified)
//   12–13 ProfileViewTests / profileSheetDevMode × 2 (added)
//   14    SignInViewTests / signInSheet.dark.en (deleted)
//   15–18 GroupListViewTests / populatedStateOnline × 4 (modified)

test.describe.configure({ mode: 'serial' });

// ───────── INDEX (read-only states) ─────────

test('index populated', async ({ page }, info) => {
  await gotoStable(page, '/');
  await expect(page).toHaveScreenshot(snap(info, 'index', 'populated'), { fullPage: true });
});

test('index filtered by theme dark', async ({ page }, info) => {
  await gotoStable(page, '/');
  // Activate the "dark" chip in the theme axis.
  await page.locator('.chip[data-axis="theme"][data-val="dark"]').click({ force: true });
  await page.waitForTimeout(80);
  await expect(page).toHaveScreenshot(snap(info, 'index', 'filteredDark'), { fullPage: true });
});

test('index group collapsed', async ({ page }, info) => {
  await gotoStable(page, '/');
  // Collapse the first group.
  await page.locator('.group .group-head').first().click({ force: true });
  await page.waitForTimeout(80);
  await expect(page).toHaveScreenshot(snap(info, 'index', 'collapsed'), { fullPage: true });
});

// ───────── DIFF MODES (read-only) ─────────

test('diff side modified', async ({ page }, info) => {
  await gotoStable(page, '/diff/0');
  await expect(page).toHaveScreenshot(snap(info, 'diff', 'sideModified'), { fullPage: true });
});

// Modes are now driven by keyboard (1=side, 2=swipe, 3=toggle, 4=pixel,
// 5=onion); the mode-button bar was removed in commit 982548e.

test('diff swipe modified', async ({ page }, info) => {
  await gotoStable(page, '/diff/0');
  await page.keyboard.press('2');
  await page.waitForTimeout(80);
  await expect(page).toHaveScreenshot(snap(info, 'diff', 'swipeModified'), { fullPage: true });
});

test('diff toggle modified', async ({ page }, info) => {
  await gotoStable(page, '/diff/0');
  await page.keyboard.press('3');
  await page.waitForTimeout(80);
  await expect(page).toHaveScreenshot(snap(info, 'diff', 'toggleModified'), { fullPage: true });
});

test('diff pixel modified', async ({ page }, info) => {
  await gotoStable(page, '/diff/0');
  await page.keyboard.press('4');
  await page.waitForTimeout(150);
  await expect(page).toHaveScreenshot(snap(info, 'diff', 'pixelModified'), { fullPage: true });
});

test('diff onion modified', async ({ page }, info) => {
  await gotoStable(page, '/diff/0');
  await page.keyboard.press('5');
  await page.waitForTimeout(80);
  await expect(page).toHaveScreenshot(snap(info, 'diff', 'onionModified'), { fullPage: true });
});

test('diff added', async ({ page }, info) => {
  // ID 12 = profileSheetDevMode.dark.en.png (added)
  await gotoStable(page, '/diff/12');
  await expect(page).toHaveScreenshot(snap(info, 'diff', 'added'), { fullPage: true });
});

test('diff deleted', async ({ page }, info) => {
  // ID 14 = signInSheet.dark.en.png (deleted)
  await gotoStable(page, '/diff/14');
  await expect(page).toHaveScreenshot(snap(info, 'diff', 'deleted'), { fullPage: true });
});

// ───────── VERDICT MUTATIONS (run last) ─────────

test('diff verdict approved', async ({ page, baseURL }, info) => {
  // Set ID 4 to approved.
  await postVerdict(baseURL!, '4', 'approved');
  await gotoStable(page, '/diff/4');
  await expect(page).toHaveScreenshot(snap(info, 'diff', 'verdictApproved'), { fullPage: true });
});

test('diff verdict rejected', async ({ page, baseURL }, info) => {
  // Set ID 5 to rejected with a comment.
  await postVerdict(baseURL!, '5', 'rejected', 'logo cropped on pt-BR');
  await gotoStable(page, '/diff/5');
  await expect(page).toHaveScreenshot(snap(info, 'diff', 'verdictRejected'), { fullPage: true });
});

test('index after verdicts', async ({ page }, info) => {
  // By now IDs 4 and 5 have verdicts; the index shows mixed state.
  await gotoStable(page, '/');
  await expect(page).toHaveScreenshot(snap(info, 'index', 'mixedVerdicts'), { fullPage: true });
});
