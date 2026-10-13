import { expect, test } from '@playwright/test';

test('FR-SHELL-001 AC2: at 1280×720 no horizontal scrollbar; status, quick links and avatar fully visible', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto('/');
  await expect(page.getByRole('navigation', { name: 'Main navigation' })).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBe(0);
  const targets = [
    page.getByRole('button', { name: /^Market data status/ }),
    page.getByRole('link', { name: 'Order Book' }),
    page.getByRole('link', { name: 'Positions' }),
    page.getByRole('button', { name: 'Account menu' }),
  ];
  // The bell (P3) joins this list when it ships.
  for (const t of targets) {
    const box = await t.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.y).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(1280);
  }
  // P0-5 Task 12: the three index ticker pills. .topbar__ticker clips its overflow and boundingBox() ignores clipping,
  // so each pill must lie inside the ticker container's box, not merely inside the window.
  const ticker = await page.getByTestId('topbar-ticker').boundingBox();
  expect(ticker).not.toBeNull();
  for (const name of [/^NIFTY 50,/, /^NIFTY BANK,/, /^SENSEX,/]) {
    const box = await page.getByRole('button', { name }).boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(ticker!.x);
    expect(box!.x + box!.width).toBeLessThanOrEqual(ticker!.x + ticker!.width);
    expect(box!.x + box!.width).toBeLessThanOrEqual(1280);
  }
});

test('P0-5 Ruling 113 I1: a keyboard-focused ticker pill shows its focus ring inside the clipping ticker strip', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto('/');
  const ticker = page.getByTestId('topbar-ticker');
  for (const name of [/^NIFTY 50,/, /^NIFTY BANK,/, /^SENSEX,/]) {
    const pill = page.getByRole('button', { name });
    await page.keyboard.press('Shift'); // a keyboard interaction first, so programmatic focus matches :focus-visible
    await pill.focus();
    expect(await pill.evaluate((el) => el.matches(':focus-visible'))).toBe(true);
    const ring = await pill.evaluate((el) => {
      const s = getComputedStyle(el);
      return { style: s.outlineStyle, width: parseFloat(s.outlineWidth), offset: parseFloat(s.outlineOffset) };
    });
    expect(ring.style).not.toBe('none');
    expect(ring.width).toBeGreaterThan(0);
    // The ring is drawn from the border edge outwards by `offset`; with offset ≤ -width it lies inside the pill.
    expect(ring.offset).toBeLessThanOrEqual(-ring.width);
    const pillBox = (await pill.boundingBox())!;
    const tickerBox = (await ticker.boundingBox())!;
    expect(pillBox.x).toBeGreaterThanOrEqual(tickerBox.x);
    expect(pillBox.x + pillBox.width).toBeLessThanOrEqual(tickerBox.x + tickerBox.width);
    expect(pillBox.y).toBeGreaterThanOrEqual(tickerBox.y);
    expect(pillBox.y + pillBox.height).toBeLessThanOrEqual(tickerBox.y + tickerBox.height);
  }
});
