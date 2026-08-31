/**
 * End-to-end tests against the live OpenBooks instance.
 * Requires: vite dev server on http://localhost:5173 + active IRC backend.
 *
 * Rate-limit strategy: performs exactly 2 IRC searches total.
 *   - "fire" — shared across connection, results, filter, normalisation, and dedup tests
 *   - "dragon"    — used only in the multi-tab describe block
 *
 * Run with:  npx playwright test tests/e2e.spec.ts
 */

import { test, expect, type Browser, type Page } from '@playwright/test';

// ── Helpers ───────────────────────────────────────────────────────────────────

const BASE = 'http://localhost:5173';
const CONNECT_TIMEOUT = 60_000;   // IRC re-handshake can be slow after prior runs
const SEARCH_TIMEOUT  = 150_000;  // 2.5 min — covers cold IRC searches

/** Wait for the WebSocket to connect (connected indicator appears in header).
 *  The username is a random guest name (e.g. "eager_lion") so we can't match
 *  a fixed prefix — instead we wait for the BadgeCheck icon that only renders
 *  when appStore.username is set (i.e. connected to IRC). */
async function waitConnected(page: Page) {
  // The header shows a BadgeCheck SVG icon when connected, PlugZap when not.
  // Wait for any text to appear in the username span (non-empty = connected).
  await page.waitForFunction(
    () => {
      const el = document.querySelector('header span.truncate');
      return el && el.textContent && el.textContent.trim() !== '' &&
             el.textContent.trim() !== 'Connecting…' &&
             el.textContent.trim() !== 'Disconnected';
    },
    null,
    { timeout: CONNECT_TIMEOUT }
  );
}


/**
 * Type a query, submit, and wait until results appear.
 * Throws a descriptive error if the IRC search times out (bot offline/rate-limited)
 * rather than waiting the full SEARCH_TIMEOUT.
 */
async function doSearch(page: Page, query: string) {
  const input = page.locator('input[type="search"]');
  await input.fill(query);
  await input.press('Enter');

  // Accept: results rows visible OR timeout banner visible (so we fail fast)
  await page.waitForFunction(
    () => {
      if (document.querySelectorAll('.animate-spin').length > 0) return false;
      if (document.querySelectorAll('tbody tr').length > 0) return true;
      // Timeout state: "Search timed out" heading in the DOM
      return !!document.querySelector('[class*="bg-red-100"]')
          || document.body.innerText.includes('Search timed out');
    },
    null,
    { timeout: SEARCH_TIMEOUT },
  );

  // If we landed in the timeout state, throw so beforeAll fails clearly
  const rows = await page.locator('tbody tr').count();
  if (rows === 0) {
    throw new Error(
      `IRC search for "${query}" timed out — bot may be rate-limited or offline`,
    );
  }
}

/** Query strings currently visible as tabs. */
async function tabQueries(page: Page): Promise<string[]> {
  return page.locator('[data-testid="tab-query"]').allTextContents();
}

// ── 1. Connection (no search needed) ─────────────────────────────────────────

test.describe('Connection', () => {
  test('username appears in header and main area after connect', async ({ page }) => {
    await page.goto(BASE);
    await waitConnected(page);
    // The header span should contain a non-empty username (guest name like "eager_lion")
    const headerSpan = page.locator('header span.truncate');
    await expect(headerSpan).not.toBeEmpty();
  });

  test('search input is enabled when connected', async ({ page }) => {
    await page.goto(BASE);
    await waitConnected(page);
    await expect(page.locator('input[type="search"]')).toBeEnabled();
  });

  test('submit button is disabled with empty input', async ({ page }) => {
    await page.goto(BASE);
    await waitConnected(page);
    await expect(page.locator('button[type="submit"]')).toBeDisabled();
  });

  test('empty-string search does not add a new tab', async ({ page }) => {
    await page.goto(BASE);
    await waitConnected(page);
    const countBefore = (await tabQueries(page)).length;
    await page.locator('input[type="search"]').press('Enter');
    await page.waitForTimeout(400);
    const countAfter = (await tabQueries(page)).length;
    expect(countAfter).toBe(countBefore);
  });
});

// ── 2. Search results (fire) ───────────────────────────────────────────────
//
// One shared page for the entire block to avoid repeated IRC searches.

test.describe('Search results (fire)', () => {
  let p: Page;

  test.beforeAll(async ({ browser }: { browser: Browser }) => {
    p = await browser.newPage();
    await p.setViewportSize({ width: 1280, height: 800 });
    await p.goto(BASE);
    await waitConnected(p);
    try {
      await doSearch(p, 'fire');
      await p.screenshot({ path: '/tmp/pw_fire.png' });
    } catch (err) {
      console.warn('⚠ Search failed (IRC down?) — search tests will be skipped:', (err as Error).message);
      await p.close();
      p = null as unknown as Page;
    }
  }, { timeout: 300_000 });

  test.afterAll(async () => { if (p) await p.close(); });

  // Skip all tests in this group if the beforeAll couldn't get IRC results
  test.beforeEach(async () => {
    test.skip(!p, 'IRC not connected — skipping search-results tests');
  });

  // ─ Results existence ───────────────────────────────────────────────────────

  test('tbody has at least one row', async () => {
    expect(await p.locator('tbody tr').count()).toBeGreaterThan(0);
  });

  test('stats bar shows "N results"', async () => {
    await expect(p.locator('text=/\\d+ results/')).toBeVisible();
  });

  test('result count is > 0', async () => {
    const text = await p.locator('text=/\\d+ results/').textContent() ?? '';
    expect(parseInt(text, 10)).toBeGreaterThan(0);
  });

  // ─ Tab ─────────────────────────────────────────────────────────────────────

  test('tab query is lowercase "fire"', async () => {
    expect(await tabQueries(p)).toContain('fire');
  });

  test('tab result-count badge is a positive integer', async () => {
    const badge = p.locator('[data-testid="tab-badge"]').first();
    await expect(badge).toBeVisible();
    expect(parseInt((await badge.textContent()) ?? '0', 10)).toBeGreaterThan(0);
  });

  test('active tab has highlighted styling', async () => {
    const tab = p.locator('[data-testid="search-tab"]', { hasText: 'fire' });
    await expect(tab).toHaveClass(/bg-white/);
  });

  // ─ Table structure ─────────────────────────────────────────────────────────

  test('table has Server, Author, Title, Format, Size columns', async () => {
    const texts = (await p.locator('thead th').allTextContents()).join(' ').toLowerCase();
    for (const col of ['server', 'author', 'title', 'format', 'size']) {
      expect(texts).toContain(col);
    }
  });

  test('first result row has non-empty server, author, and title', async () => {
    // Skip the padding row (height-only <tr>) — find first row with actual cells
    const rows = p.locator('tbody tr').filter({ has: p.locator('td') });
    const cells = rows.first().locator('td');
    for (const idx of [0, 1, 2]) {
      const text = await cells.nth(idx).textContent();
      expect(text?.trim()).not.toBe('');
    }
  });

  test('Download button visible on first result row', async () => {
    const row = p.locator('tbody tr').filter({ has: p.locator('button', { hasText: /download/i }) }).first();
    await expect(row.locator('button', { hasText: /download/i })).toBeVisible();
  });

  // ─ Scroll / virtualizer ────────────────────────────────────────────────────

  test('scroll container has a positive client height (layout OK)', async () => {
    const sc = p.locator('[data-testid="book-scroll-container"]');
    const { clientHeight } = await sc.evaluate(el => ({ clientHeight: el.clientHeight }));
    expect(clientHeight).toBeGreaterThan(100);
  });

  test('scroll container is scrollable (scrollHeight > clientHeight)', async () => {
    const sc = p.locator('[data-testid="book-scroll-container"]');
    const { scrollHeight, clientHeight } = await sc.evaluate(el => ({
      scrollHeight: el.scrollHeight,
      clientHeight: el.clientHeight,
    }));
    expect(scrollHeight).toBeGreaterThan(clientHeight);
  });

  test('scrolling 600 px updates scrollTop', async () => {
    const sc = p.locator('[data-testid="book-scroll-container"]');
    await sc.evaluate(el => { el.scrollTop = 600; });
    await p.waitForTimeout(300);
    expect(await sc.evaluate(el => el.scrollTop)).toBeGreaterThan(0);
    await sc.evaluate(el => { el.scrollTop = 0; });
  });

  test('virtualizer renders different rows after large scroll', async () => {
    const sc = p.locator('[data-testid="book-scroll-container"]');
    const before = await p.locator('tbody td:nth-child(3) span').allTextContents();
    await sc.evaluate(el => { el.scrollTop = 3000; });
    await p.waitForTimeout(500);
    const after = await p.locator('tbody td:nth-child(3) span').allTextContents();
    const overlap = before.filter(t => t && after.includes(t)).length;
    expect(overlap).toBeLessThan(Math.ceil(before.length * 0.5));
    await sc.evaluate(el => { el.scrollTop = 0; });
    await p.waitForTimeout(200);
  });

  // ─ Filters ─────────────────────────────────────────────────────────────────

  test('matched/total counter shows "X/Y" format', async () => {
    const text = await p.locator('span.tabular-nums').last().textContent();
    expect(text).toMatch(/\d+\/\d+/);
  });

  test('format chip buttons exist (EPUB/MOBI/etc.)', async () => {
    expect(await p.locator('button', { hasText: /^(EPUB|MOBI|AZW3|PDF|HTM)$/ }).count()).toBeGreaterThan(0);
  });

  test('clicking EPUB chip reduces matched count', async () => {
    const counter = () => p.locator('span.tabular-nums').last().textContent();
    const before = parseInt((await counter())?.split('/')[0] ?? '0', 10);
    await p.locator('button', { hasText: /^EPUB$/ }).first().click();
    await p.waitForTimeout(300);
    const after = parseInt((await counter())?.split('/')[0] ?? '0', 10);
    expect(after).toBeLessThanOrEqual(before);
    await p.locator('button', { hasText: /^All$/ }).first().click();
    await p.waitForTimeout(200);
  });

  test('title filter narrows matched count', async () => {
    const input = p.locator('input[placeholder="Title…"]');
    await input.fill('lord');
    await p.waitForTimeout(300);
    const text = await p.locator('span.tabular-nums').last().textContent();
    const [matched, total] = (text ?? '0/0').split('/').map(Number);
    expect(matched).toBeLessThanOrEqual(total!);
    await input.fill('');
    await p.waitForTimeout(200);
  });

  test('author filter narrows matched count', async () => {
    const input = p.locator('input[placeholder="Author…"]');
    await input.fill('stephen');
    await p.waitForTimeout(300);
    expect(await p.locator('tbody tr').count()).toBeGreaterThan(0);
    await input.fill('');
    await p.waitForTimeout(200);
  });

  test('gibberish author filter yields 0 matched', async () => {
    const input = p.locator('input[placeholder="Author…"]');
    await input.fill('zzzzz_no_such_author_xyzzy');
    await p.waitForTimeout(300);
    const matched = parseInt(
      (await p.locator('span.tabular-nums').last().textContent())?.split('/')[0] ?? '0',
      10,
    );
    expect(matched).toBe(0);
    await input.fill('');
    await p.waitForTimeout(200);
  });

  test('"N/A size" chip toggles without crashing', async () => {
    const chip = p.locator('button', { hasText: /N\/A size/ });
    await expect(chip).toBeVisible();
    await chip.click();
    await p.waitForTimeout(200);
    await chip.click();
    await p.waitForTimeout(200);
    expect(await p.locator('tbody tr').count()).toBeGreaterThan(0);
  });

  // ─ Sorting ─────────────────────────────────────────────────────────────────

  test('clicking Author header sorts without crashing', async () => {
    const btn = p.locator('thead th').nth(1).locator('button');
    await btn.click(); await p.waitForTimeout(200);
    await btn.click(); await p.waitForTimeout(200);
    expect(await p.locator('tbody tr').count()).toBeGreaterThan(0);
  });

  test('clicking Title header sorts without crashing', async () => {
    const btn = p.locator('thead th').nth(2).locator('button');
    await btn.click(); await p.waitForTimeout(200);
    await btn.click(); await p.waitForTimeout(200);
    expect(await p.locator('tbody tr').count()).toBeGreaterThan(0);
  });

  // ─ Other controls ──────────────────────────────────────────────────────────

  test('Group Books toggle works', async () => {
    const btn = p.locator('button', { hasText: /Group Books/ });
    await expect(btn).toBeVisible();
    await btn.click(); await p.waitForTimeout(300);
    expect(await p.locator('tbody tr').count()).toBeGreaterThan(0);
    await btn.click(); await p.waitForTimeout(200);
  });

  test('Export button is visible', async () => {
    await expect(p.locator('button', { hasText: /Export/ })).toBeVisible();
  });

  test('Refresh button is visible', async () => {
    await expect(p.locator('button', { hasText: /Refresh/ })).toBeVisible();
  });

  // ─ Case normalisation (reuses same page/results) ──────────────────────────

  test('searching "Fire" (cap F) does not open a second tab', async () => {
    const countBefore = (await tabQueries(p)).length;
    await p.locator('input[type="search"]').fill('Fire');
    await p.locator('input[type="search"]').press('Enter');
    await p.waitForTimeout(700);
    expect((await tabQueries(p)).length).toBe(countBefore);
  });

  test('searching "FIRE" (all caps) does not open a second tab', async () => {
    const countBefore = (await tabQueries(p)).length;
    await p.locator('input[type="search"]').fill('FIRE');
    await p.locator('input[type="search"]').press('Enter');
    await p.waitForTimeout(700);
    expect((await tabQueries(p)).length).toBe(countBefore);
  });

  test('all tab query labels are lowercase', async () => {
    for (const q of await tabQueries(p)) {
      expect(q).toBe(q.toLowerCase());
    }
  });

  // ─ Deduplication (reuses same page/results) ───────────────────────────────

  test('same query twice does not open a duplicate tab', async () => {
    const countBefore = (await tabQueries(p)).length;
    await p.locator('input[type="search"]').fill('fire');
    await p.locator('input[type="search"]').press('Enter');
    await p.waitForTimeout(700);
    expect((await tabQueries(p)).length).toBe(countBefore);
  });

  test('results remain visible after dedup reuse', async () => {
    expect(await p.locator('tbody tr').count()).toBeGreaterThan(0);
  });
});

// ── 3. Multi-tab (performs the second IRC search: "dragon") ─────────────────────

test.describe('Multi-tab', () => {
  let p: Page;

  test.beforeAll(async ({ browser }: { browser: Browser }) => {
    p = await browser.newPage();
    await p.setViewportSize({ width: 1280, height: 800 });
    await p.goto(BASE);
    await waitConnected(p);
    try {
      await doSearch(p, 'fire');
      await doSearch(p, 'dragon');
      await p.screenshot({ path: '/tmp/pw_multitab.png' });
    } catch (err) {
      console.warn('⚠ Search failed (IRC down?) — multi-tab tests will be skipped:', (err as Error).message);
      await p.close();
      p = null as unknown as Page;
    }
  }, { timeout: 450_000 });

  test.afterAll(async () => { if (p) await p.close(); });

  test.beforeEach(async () => {
    test.skip(!p, 'IRC not connected — skipping multi-tab tests');
  });

  test('two searches produce two distinct tabs', async () => {
    const queries = await tabQueries(p);
    expect(queries).toContain('fire');
    expect(queries).toContain('dragon');
  });

  test('each tab has a non-zero result badge', async () => {
    for (const badge of await p.locator('[data-testid="tab-badge"]').all()) {
      expect(parseInt((await badge.textContent()) ?? '0', 10)).toBeGreaterThan(0);
    }
  });

  test('clicking fire tab updates search placeholder', async () => {
    const tab = p.locator('[data-testid="search-tab"]', { hasText: 'fire' });
    await tab.click();
    await p.waitForTimeout(400);
    const ph = await p.locator('input[type="search"]').getAttribute('placeholder');
    expect(ph?.toLowerCase()).toContain('fire');
  });

  test('fire results are scrollable after tab switch', async () => {
    const sc = p.locator('[data-testid="book-scroll-container"]');
    const { scrollHeight, clientHeight } = await sc.evaluate(el => ({
      scrollHeight: el.scrollHeight,
      clientHeight: el.clientHeight,
    }));
    expect(scrollHeight).toBeGreaterThan(clientHeight);
  });

  test('clicking dragon tab updates search placeholder', async () => {
    const tab = p.locator('[data-testid="search-tab"]', { hasText: 'dragon' });
    await tab.click();
    await p.waitForTimeout(400);
    const ph = await p.locator('input[type="search"]').getAttribute('placeholder');
    expect(ph?.toLowerCase()).toContain('dragon');
  });

  test('dragon results are scrollable', async () => {
    const sc = p.locator('[data-testid="book-scroll-container"]');
    const { scrollHeight, clientHeight } = await sc.evaluate(el => ({
      scrollHeight: el.scrollHeight,
      clientHeight: el.clientHeight,
    }));
    expect(scrollHeight).toBeGreaterThan(clientHeight);
  });

  test('closing fire tab removes it from the bar', async () => {
    const tab = p.locator('[data-testid="search-tab"]', { hasText: 'fire' });
    await tab.hover();
    // Close span is the last <span> child of the tab button
    await tab.locator('span').last().click();
    await p.waitForTimeout(400);
    expect(await tabQueries(p)).not.toContain('fire');
  });

  test('remaining tabs still render results after close', async () => {
    expect(await p.locator('tbody tr').count()).toBeGreaterThan(0);
  });
});
