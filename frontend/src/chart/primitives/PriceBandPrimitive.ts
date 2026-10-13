import type {
  IPrimitivePaneRenderer, IPrimitivePaneView, ISeriesApi, ISeriesPrimitive, SeriesAttachedParameter, SeriesType, Time,
} from 'lightweight-charts';

type Target = Parameters<IPrimitivePaneRenderer['draw']>[0];
export interface PriceBand { from: number; to: number; color: string }

class BandRenderer implements IPrimitivePaneRenderer {
  constructor(private readonly y1: number | null, private readonly y2: number | null, private readonly color: string) {}
  draw(target: Target): void {
    const { y1, y2 } = this;
    if (y1 === null || y2 === null) return;
    target.useBitmapCoordinateSpace((scope) => {
      const top = Math.round(Math.min(y1, y2) * scope.verticalPixelRatio);
      const height = Math.round(Math.abs(y2 - y1) * scope.verticalPixelRatio);
      scope.context.fillStyle = this.color;
      scope.context.fillRect(0, top, scope.bitmapSize.width, height);
    });
  }
}

class BandView implements IPrimitivePaneView {
  constructor(private readonly source: PriceBandPrimitive) {}
  zOrder() { return 'bottom' as const; }
  renderer(): IPrimitivePaneRenderer { return this.source.renderer(); }
}

/** S0-5 "custom primitive": a horizontal price band (later reused for band fills, ARCHITECTURE §8.5). */
export class PriceBandPrimitive implements ISeriesPrimitive<Time> {
  private series: Pick<ISeriesApi<SeriesType, Time>, 'priceToCoordinate'> | null = null;
  private requestUpdate: (() => void) | null = null;
  private readonly views: readonly IPrimitivePaneView[] = [new BandView(this)];

  constructor(private band: PriceBand) {}

  attached(param: SeriesAttachedParameter<Time>): void {
    this.series = param.series;
    this.requestUpdate = param.requestUpdate;
  }

  detached(): void {
    this.series = null;
    this.requestUpdate = null;
  }

  paneViews(): readonly IPrimitivePaneView[] { return this.views; }

  update(patch: Partial<PriceBand>): void {
    this.band = { ...this.band, ...patch };
    this.requestUpdate?.();
  }

  renderer(): IPrimitivePaneRenderer {
    const s = this.series;
    return new BandRenderer(s?.priceToCoordinate(this.band.from) ?? null, s?.priceToCoordinate(this.band.to) ?? null, this.band.color);
  }
}
