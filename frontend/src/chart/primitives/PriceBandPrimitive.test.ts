import { describe, expect, it, vi } from 'vitest';
import { PriceBandPrimitive } from './PriceBandPrimitive';

describe('PriceBandPrimitive', () => {
  it('fills the band between the two price coordinates in bitmap space', () => {
    const p = new PriceBandPrimitive({ from: 10, to: 20, color: 'band' });
    const requestUpdate = vi.fn();
    p.attached({ series: { priceToCoordinate: (x: number) => x }, requestUpdate } as never);
    const fillRect = vi.fn();
    const ctx = { fillRect, fillStyle: '' };
    const target = {
      useBitmapCoordinateSpace: (f: (s: unknown) => void) =>
        f({ context: ctx, bitmapSize: { width: 100, height: 80 }, verticalPixelRatio: 2, horizontalPixelRatio: 2, mediaSize: { width: 50, height: 40 } }),
    };
    p.paneViews()[0].renderer()!.draw(target as never);
    expect(ctx.fillStyle).toBe('band');
    expect(fillRect).toHaveBeenCalledWith(0, 20, 100, 20);
    p.update({ to: 30 });
    expect(requestUpdate).toHaveBeenCalledTimes(1);
    expect(p.paneViews()[0].zOrder?.()).toBe('bottom');
  });
});
