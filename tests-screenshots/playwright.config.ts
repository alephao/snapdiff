import { defineConfig, devices } from '@playwright/test';

// Single shared snapdiff instance booted by global-setup against a
// deterministic fixture repo. URL handed off via SNAPDIFF_URL env var.
export default defineConfig({
  testDir: '.',
  testMatch: /.*\.spec\.ts$/,

  // Visual-regression suite must run deterministically: one worker,
  // strict in-order tests within each file.
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: 0,

  reporter: process.env.CI ? 'dot' : 'list',

  globalSetup: './global-setup.ts',
  globalTeardown: './global-teardown.ts',

  // Flat baselines directory, names like `index.populated.desktop.png`
  // so snapdiff's own axis_regex can group them by page/scenario/viewport.
  snapshotPathTemplate: '{testDir}/baselines/{arg}{ext}',

  expect: {
    // Strict pixel comparison; with fonts embedded and animations off,
    // captures are byte-stable in our test environment.
    toHaveScreenshot: {
      maxDiffPixelRatio: 0.001,
      animations: 'disabled',
      caret: 'hide',
    },
  },

  use: {
    baseURL: process.env.SNAPDIFF_URL || 'http://127.0.0.1:7777',
    actionTimeout: 5_000,
    navigationTimeout: 10_000,
  },

  projects: [
    {
      name: 'desktop',
      use: { ...devices['Desktop Chrome'], viewport: { width: 1280, height: 800 } },
    },
    {
      name: 'mobile',
      use: { ...devices['Pixel 7'], viewport: { width: 390, height: 844 } },
    },
  ],
});
