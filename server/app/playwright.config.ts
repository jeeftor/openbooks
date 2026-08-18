import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 360_000,       // per-test + beforeAll timeout (covers cold IRC searches)
  workers: 1,             // run serially — IRC rate-limit protection
  use: {
    baseURL: 'http://localhost:5173',
    headless: true,
    viewport: { width: 1280, height: 800 },
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
});
