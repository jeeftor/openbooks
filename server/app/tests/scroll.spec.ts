import { test, expect } from '@playwright/test';

test('search for "fire" and verify results table is scrollable', async ({ page }) => {
  await page.goto('http://localhost:5173');

  // Wait for WebSocket connection — username is a random guest name,
  // so wait for the header span to be non-empty (not "Connecting…" / "Disconnected")
  await page.waitForFunction(
    () => {
      const el = document.querySelector('header span.truncate');
      return el && el.textContent && el.textContent.trim() !== '' &&
             el.textContent.trim() !== 'Connecting…' &&
             el.textContent.trim() !== 'Disconnected';
    },
    null,
    { timeout: 20000 }
  );

  // Type and submit search
  const input = page.locator('input[type="search"]');
  await input.fill('fire');
  await input.press('Enter');

  // Wait for results — tab badge changes from spinner to a number
  await page.waitForFunction(() => {
    // No animate-spin elements remaining (all searches settled)
    const spinners = document.querySelectorAll('.animate-spin');
    // And at least one result row exists
    const rows = document.querySelectorAll('tbody tr');
    return spinners.length === 0 && rows.length > 0;
  }, null, { timeout: 150000 });

  await page.screenshot({ path: '/tmp/pw_fire_results.png' });

  // Log all overflow-auto divs for diagnostics
  const allOverflowAuto = await page.evaluate(() =>
    [...document.querySelectorAll('div')].filter(el =>
      getComputedStyle(el).overflow === 'auto' || el.classList.contains('overflow-auto')
    ).map(el => ({
      testid: el.getAttribute('data-testid'),
      scrollHeight: el.scrollHeight,
      clientHeight: el.clientHeight,
      rect: el.getBoundingClientRect(),
    }))
  );
  console.log('All overflow-auto divs:', JSON.stringify(allOverflowAuto, null, 2));

  // Locate the virtualizer scroll container inside BookTable
  const scrollContainer = page.locator('[data-testid="book-scroll-container"]');
  await expect(scrollContainer).toBeVisible();

  const metrics = await scrollContainer.evaluate((el) => ({
    scrollHeight: el.scrollHeight,
    clientHeight: el.clientHeight,
    isScrollable: el.scrollHeight > el.clientHeight,
    rect: el.getBoundingClientRect(),
  }));

  console.log('Scroll container metrics:', metrics);
  console.log(`scrollHeight=${metrics.scrollHeight}, clientHeight=${metrics.clientHeight}`);

  // Must be scrollable (more content than visible area)
  expect(metrics.isScrollable).toBe(true);

  // Bottom of scroll container must not be clipped by viewport
  expect(metrics.rect.bottom).toBeLessThanOrEqual(page.viewportSize()!.height + 1);

  // Actually scroll down and verify position changes
  await scrollContainer.evaluate((el) => { el.scrollTop = 600; });
  await page.waitForTimeout(400);
  const scrollTop = await scrollContainer.evaluate((el) => el.scrollTop);
  console.log(`scrollTop after scroll: ${scrollTop}`);
  expect(scrollTop).toBeGreaterThan(0);

  await page.screenshot({ path: '/tmp/pw_fire_scrolled.png' });
});
