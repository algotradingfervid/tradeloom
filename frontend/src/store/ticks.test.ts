import { expect, it } from 'vitest';
import { tickFixture } from '../api/mock/fixtures';
import { useTicksStore } from './ticks';

it('applyBatch replaces only the tokens in the batch (unchanged rows keep identity)', () => {
  useTicksStore.getState().applyBatch([tickFixture(1, 10, 9), tickFixture(2, 20, 19)]);
  const before = useTicksStore.getState().latest;
  useTicksStore.getState().applyBatch([tickFixture(2, 21, 19)]);
  const after = useTicksStore.getState().latest;
  expect(after[1]).toBe(before[1]);
  expect(after[2].ltp).toBe(21);
});
