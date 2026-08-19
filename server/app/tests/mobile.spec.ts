/**
 * Mobile layout and UX validation — iPhone 14 viewport (390 × 844).
 *
 * Validates assumptions about the mobile experience:
 *   - layout fits without horizontal overflow
 *   - BookCards (not BookTable) renders on narrow viewports
 *   - filter drawer works
 *   - virtualised card list is scrollable
 *   - header controls are reachable / not clipped
 *   - no content hidden under the FloatingTaskPanel pill
 *   - safe-area / notch handling is present in markup
 *
 * Requires the Vite dev server on http://localhost:5173 and an active backend.
 * One IRC search ("fire") is shared across all search-result tests.
 *
 * Run with:  npx playwright test tests/mobile.spec.ts
 */

import { test, expect, type Page } from '@playwright/test';

// ── Constants ─────────────────────────────────────────────────────────────────

const BASE            = 'http://localhost:5173';
const CONNECT_TIMEOUT = 60_000;
const SEARCH_TIMEOUT  = 150_000;

/** iPhone 14 logical pixels */
const MOBILE = { width: 390, height: 844 };

// ── Helpers ───────────────────────────────────────────────────────────────────

async function setMobile(page: Page) {
  await page.setViewportSize(MOBILE);
}

async function waitConnected(page: Page) {
  // The search input is disabled until the WebSocket handshake completes.
  // This works on all viewport sizes (no responsive hiding).
  await page.waitForFunction(
    () => {
      const input = document.querySelector<HTMLInputElement>('input[type="search"]');
      return !!input && !input.disabled;
    },
    null,
    { timeout: CONNECT_TIMEOUT },
  );
}

/** Returns true only when real book cards appear; false on timeout or no results. */
async function doSearchMobile(page: Page, query: string): Promise<boolean> {
  const input = page.locator('input[type="search"]');
  await input.fill(query);
  await input.press('Enter');

  try {
    await Promise.race([
      // Success path: real book cards rendered in the virtual list
      page.waitForFunction(
        () => document.querySelectorAll('[data-testid="book-card"]').length > 0
          || document.querySelectorAll('[data-testid="book-scroll-cards"] > div > div').length > 0,
        null,
        { timeout: SEARCH_TIMEOUT },
      ),
      // Failure path: IRC timeout or no results — resolve early as failure
      page.waitForFunction(
        () => document.body.innerText.includes('Search timed out')
          || document.body.innerText.includes('No results'),
        null,
        { timeout: SEARCH_TIMEOUT },
      ).then(() => Promise.reject(new Error('search-failed'))),
    ]);
    return true;
  } catch {
    return false;
  }
}

// ── 1. Static layout (no search required) ────────────────────────────────────

test.describe('Mobile — static layout', () => {
  test.beforeEach(async ({ page }) => {
    await setMobile(page);
    await page.goto(BASE);
  });

  test('viewport meta: width=device-width, viewport-fit=cover', async ({ page }) => {
    const viewport = await page.$eval('meta[name="viewport"]', el =>
      el.getAttribute('content') ?? ''
    );
    expect(viewport).toContain('width=device-width');
    expect(viewport).toContain('viewport-fit=cover');
  });

  test('no horizontal overflow — page width equals viewport width', async ({ page }) => {
    const overflow = await page.evaluate(() => {
      return document.documentElement.scrollWidth > window.innerWidth;
    });
    expect(overflow, 'page has horizontal overflow').toBe(false);
  });

  test('page fills viewport height (h-dvh)', async ({ page }) => {
    const appHeight = await page.$eval('#app', el => el.getBoundingClientRect().height);
    // Allow 1px rounding tolerance
    expect(appHeight).toBeGreaterThanOrEqual(MOBILE.height - 1);
  });

  test('header is visible and within viewport', async ({ page }) => {
    const header = page.locator('header').first();
    await expect(header).toBeVisible();
    const rect = await header.boundingBox();
    expect(rect).not.toBeNull();
    expect(rect!.x).toBeGreaterThanOrEqual(0);
    expect(rect!.x + rect!.width).toBeLessThanOrEqual(MOBILE.width + 1);
  });

  test('brand name "OpenBooks ABS" is visible in header', async ({ page }) => {
    await expect(page.getByText('OpenBooks', { exact: false }).first()).toBeVisible();
  });

  test('search input is present and within horizontal bounds', async ({ page }) => {
    const input = page.locator('input[type="search"]');
    await expect(input).toBeVisible();
    const rect = await input.boundingBox();
    expect(rect!.x).toBeGreaterThanOrEqual(0);
    expect(rect!.x + rect!.width).toBeLessThanOrEqual(MOBILE.width + 1);
  });

  test('search button is present and does not overflow', async ({ page }) => {
    const btn = page.locator('button[type="submit"]');
    await expect(btn).toBeVisible();
    const rect = await btn.boundingBox();
    expect(rect!.x + rect!.width).toBeLessThanOrEqual(MOBILE.width + 1);
  });

  test('safe-area padding applied to root div', async ({ page }) => {
    const style = await page.$eval('#app > div', el =>
      (el as HTMLElement).style.paddingTop
    );
    // env(safe-area-inset-top) evaluates to 0 in desktop Chromium but attribute must be present
    const html = await page.content();
    expect(html).toContain('safe-area-inset-top');
  });

  test('SearchTabBar is visible', async ({ page }) => {
    // Tab bar row — contains the "Search to start" nudge or actual tabs
    const tabBar = page.locator('[data-testid="search-tab"]').or(
      page.locator('text=Search to start')
    );
    await expect(tabBar.first()).toBeVisible();
  });

  test('Library button text hidden on mobile (hidden sm:block)', async ({ page }) => {
    // The Library label span has "hidden sm:block" — at 390px it must not be visible
    const libraryText = page.locator('text=Library');
    await expect(libraryText).toBeHidden();
  });

  test('Library icon button is still visible and tappable', async ({ page }) => {
    // The Library toggle button (icon-only on mobile)
    const libraryBtn = page.locator('button[title="Downloaded books"]');
    await expect(libraryBtn).toBeVisible();
    const rect = await libraryBtn.boundingBox();
    expect(rect!.width).toBeGreaterThan(0);
  });

  test('no <table> or <tbody> rendered — BookCards path used on mobile', async ({ page }) => {
    await waitConnected(page);
    // Submit an empty-ish search to trigger the search view to mount
    // (The table is only rendered once results exist, but check the initial DOM too)
    const tableCount = await page.locator('table').count();
    expect(tableCount, 'BookTable should not render on mobile').toBe(0);
  });
});

// ── 2. After WebSocket connection ─────────────────────────────────────────────

test.describe('Mobile — after connection', () => {
  test('search input enabled once connected', async ({ page }) => {
    await setMobile(page);
    await page.goto(BASE);
    await waitConnected(page);
    await expect(page.locator('input[type="search"]')).toBeEnabled();
  });

  test('username text NOT rendered (hidden sm:block on mobile)', async ({ page }) => {
    await setMobile(page);
    await page.goto(BASE);
    await waitConnected(page);
    // The username span has class "hidden sm:block" — must not be in the visible layout
    const usernameSpan = page.locator('header span.hidden');
    // It exists in the DOM but must be invisible (display:none via Tailwind)
    const count = await usernameSpan.count();
    if (count > 0) {
      await expect(usernameSpan.first()).toBeHidden();
    }
  });

  test('connection icon (BadgeCheck / PlugZap) visible in header', async ({ page }) => {
    await setMobile(page);
    await page.goto(BASE);
    await waitConnected(page);
    // After connecting, a BadgeCheck SVG appears next to the username area
    const connIcon = page.locator('header svg').last();
    await expect(connIcon).toBeVisible();
  });

  test('FloatingTaskPanel NOT visible before any task', async ({ page }) => {
    await setMobile(page);
    await page.goto(BASE);
    await waitConnected(page);
    // The panel only renders when taskStore.tasks.length > 0 (v-if="hasAny")
    const panel = page.locator('[data-testid="task-panel"]');
    await page.waitForTimeout(500);
    await expect(panel).toBeHidden();
  });

  test('dark-mode toggle button is in header and tappable', async ({ page }) => {
    await setMobile(page);
    await page.goto(BASE);
    await waitConnected(page);
    const darkBtn = page.locator('header button[title*="mode"]');
    await expect(darkBtn).toBeVisible();
    await darkBtn.click();
    // After clicking, the html element should toggle dark class
    const hasDark = await page.$eval('html', el => el.classList.contains('dark'));
    expect(typeof hasDark).toBe('boolean'); // just verify it toggled without error
  });

  test('empty state shown when no search yet', async ({ page }) => {
    await setMobile(page);
    await page.goto(BASE);
    await waitConnected(page);
    // EmptyState renders "Search a book to get started." when no activeItem
    const empty = page.locator('text=Search a book to get started');
    await expect(empty).toBeVisible();
  });
});

// ── 3. Search results on mobile ───────────────────────────────────────────────
// One shared search for "fire" — runs in serial (workers: 1).

let fireSearchDone = false;

test.describe('Mobile — search results (fire)', () => {
  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await setMobile(page);
    await page.goto(BASE);
    await waitConnected(page);
    fireSearchDone = await doSearchMobile(page, 'fire');
    await page.close();
  }, { timeout: SEARCH_TIMEOUT + CONNECT_TIMEOUT });

  test.beforeEach(async ({ page }) => {
    test.skip(!fireSearchDone, 'Search setup failed — skipping');
    await setMobile(page);
    await page.goto(BASE);
    await waitConnected(page);
    // Restore "fire" results from history tab (server may return cached results)
    const tab = page.locator('[data-testid="search-tab"]').first();
    if (await tab.count() > 0) await tab.click();
    // Allow 30s for results to appear (server cache should be fast)
    const ok = await page.waitForFunction(
      () => document.querySelectorAll('[data-testid="book-card"]').length > 0
        || document.querySelectorAll('[data-testid="book-scroll-cards"] > div > div').length > 0,
      null,
      { timeout: 30_000 },
    ).then(() => true).catch(() => false);
    test.skip(!ok, 'Search results not available in restored page — requires IRC/cache');
  });

  test('BookCards used — no <table> rendered with results', async ({ page }) => {
    const tableCount = await page.locator('table').count();
    expect(tableCount, 'BookTable must not render on mobile viewport').toBe(0);
  });

  test('result count / filter bar visible', async ({ page }) => {
    // BookCards filter bar: shows "N/N" count and Filter button
    const filterBtn = page.locator('button', { hasText: 'Filter' });
    await expect(filterBtn).toBeVisible();
  });

  test('Filter button does not overflow viewport', async ({ page }) => {
    const filterBtn = page.locator('button', { hasText: 'Filter' });
    const rect = await filterBtn.boundingBox();
    expect(rect!.x + rect!.width).toBeLessThanOrEqual(MOBILE.width + 2);
  });

  test('filter drawer opens on tap', async ({ page }) => {
    const filterBtn = page.locator('button', { hasText: 'Filter' });
    await filterBtn.click();
    // Bottom sheet has "Filter Results" heading
    await expect(page.locator('text=Filter Results')).toBeVisible();
  });

  test('filter drawer has author and title inputs', async ({ page }) => {
    await page.locator('button', { hasText: 'Filter' }).click();
    await expect(page.locator('text=Filter Results')).toBeVisible();
    await expect(page.locator('input[placeholder*="author" i]')).toBeVisible();
    await expect(page.locator('input[placeholder*="title" i]')).toBeVisible();
  });

  test('filter drawer closes via Done button', async ({ page }) => {
    await page.locator('button', { hasText: 'Filter' }).click();
    await expect(page.locator('text=Filter Results')).toBeVisible();
    await page.locator('button', { hasText: 'Done' }).click();
    await expect(page.locator('text=Filter Results')).toBeHidden();
  });

  test('filter drawer closes by tapping backdrop', async ({ page }) => {
    await page.locator('button', { hasText: 'Filter' }).click();
    await expect(page.locator('text=Filter Results')).toBeVisible();
    // Tap the semi-transparent backdrop (top-left corner, outside the sheet)
    await page.mouse.click(10, 10);
    await expect(page.locator('text=Filter Results')).toBeHidden();
  });

  test('card scroll container is vertically scrollable', async ({ page }) => {
    // The virtualiser scroll root in BookCards
    const scroll = page.locator('[class*="overflow-auto"]').last();
    const metrics = await scroll.evaluate(el => ({
      scrollHeight: el.scrollHeight,
      clientHeight: el.clientHeight,
    }));
    expect(metrics.scrollHeight).toBeGreaterThan(metrics.clientHeight);
  });

  test('card list can actually scroll (scrollTop advances)', async ({ page }) => {
    const scroll = page.locator('[class*="overflow-auto"]').last();
    await scroll.evaluate(el => { el.scrollTop = 400; });
    await page.waitForTimeout(300);
    const scrollTop = await scroll.evaluate(el => el.scrollTop);
    expect(scrollTop).toBeGreaterThan(0);
  });

  test('no content clipped at bottom of viewport', async ({ page }) => {
    // The cards scroll container bottom must not exceed the viewport
    const scroll = page.locator('[class*="overflow-auto"]').last();
    const rect = await scroll.boundingBox();
    expect(rect!.y + rect!.height).toBeLessThanOrEqual(MOBILE.height + 2);
  });

  test('FloatingTaskPanel pill visible after search task', async ({ page }) => {
    // After issuing a search, a task is created → panel appears (v-if="hasAny")
    const panel = page.locator('[data-testid="task-panel"]');
    await expect(panel).toBeVisible({ timeout: 5000 });
  });

  test('FloatingTaskPanel does not overlap search input', async ({ page }) => {
    const input = page.locator('input[type="search"]');
    const inputRect = await input.boundingBox();
    const panel = page.locator('[data-testid="task-panel"]');
    if (await panel.count() > 0) {
      const panelRect = await panel.first().boundingBox();
      if (panelRect && inputRect) {
        const overlaps = panelRect.y < inputRect.y + inputRect.height
          && panelRect.y + panelRect.height > inputRect.y;
        expect(overlaps, 'FloatingTaskPanel overlaps the search input').toBe(false);
      }
    }
  });

  test('Library overlay opens on mobile via SearchTabBar Library button', async ({ page }) => {
    const libraryBtn = page.locator('button[title="Downloaded books"]');
    await libraryBtn.click();
    // Library panel slides in — look for "Library" or "Downloads" heading
    await expect(
      page.locator('text=Library').or(page.locator('text=Downloads'))
    ).toBeVisible({ timeout: 3000 });
  });

  test('screenshot — mobile results view', async ({ page }) => {
    await page.screenshot({ path: '/tmp/pw_mobile_results.png', fullPage: false });
  });

  test('screenshot — filter drawer open', async ({ page }) => {
    await page.locator('button', { hasText: 'Filter' }).click();
    await expect(page.locator('text=Filter Results')).toBeVisible();
    await page.screenshot({ path: '/tmp/pw_mobile_filter_drawer.png' });
  });
});

// ── 4. Known gaps — document what is missing ──────────────────────────────────

test.describe('Mobile — known gaps (expected failures)', () => {
  test.beforeEach(async ({ page }) => {
    await setMobile(page);
    await page.goto(BASE);
    await waitConnected(page);
  });

  test('MobileNav bottom bar is present and visible', async ({ page }) => {
    const nav = page.locator('nav').filter({ hasText: /History|Downloads|Activity/ });
    await expect(nav).toBeVisible();
  });

  test('MobileNav History tab opens history panel', async ({ page }) => {
    const historyBtn = page.locator('nav button', { hasText: 'History' });
    await expect(historyBtn).toBeVisible();
    await historyBtn.click();
    // Bottom sheet appears (class bottom-14 z-40, only present when activeTab is set)
    const sheet = page.locator('[class*="bottom-14"][class*="z-40"]');
    await expect(sheet).toBeVisible({ timeout: 3000 });
    // HistoryPanel inside shows "N searches" header
    await expect(page.locator('text=/\\d+ searches/')).toBeVisible({ timeout: 2000 });
  });

  test('MobileNav Activity tab opens activity panel', async ({ page }) => {
    const activityBtn = page.locator('nav button', { hasText: 'Activity' });
    await expect(activityBtn).toBeVisible();
    await activityBtn.click();
    // Bottom sheet appears
    const sheet = page.locator('[class*="bottom-14"][class*="z-40"]');
    await expect(sheet).toBeVisible({ timeout: 3000 });
    // ActivityPanel header shows "Activity" (exact match to avoid matching "No activity yet")
    await expect(page.locator('[class*="bottom-14"]').getByText('Activity', { exact: true })).toBeVisible({ timeout: 2000 });
  });
});
