import { CandlestickSeries, HistogramSeries, LineSeries, createChart } from 'lightweight-charts';

/** Injection seam so adapter tests run against a fake library (no canvas in jsdom). */
export interface LwcLib {
  createChart: typeof createChart;
  CandlestickSeries: typeof CandlestickSeries;
  HistogramSeries: typeof HistogramSeries;
  LineSeries: typeof LineSeries;
}

export const realLwc: LwcLib = { createChart, CandlestickSeries, HistogramSeries, LineSeries };
