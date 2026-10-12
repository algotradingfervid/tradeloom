import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Tabs } from './Tabs';

const tabs = [{ id: 'a', label: 'A' }, { id: 'b', label: 'B' }];

describe('Tabs (Ruling 105 M4)', () => {
  it('sets aria-controls only when the owner gives idPrefix (and so renders the panel)', () => {
    const { unmount } = render(<Tabs label="Kinds" tabs={tabs} value="a" onChange={() => {}} />);
    expect(screen.getByRole('tab', { name: 'A' })).not.toHaveAttribute('aria-controls');
    unmount();
    render(<Tabs label="Kinds" tabs={tabs} value="a" onChange={() => {}} idPrefix="p1" />);
    expect(screen.getByRole('tab', { name: 'A' })).toHaveAttribute('aria-controls', 'p1-panel');
    expect(screen.getByRole('tab', { name: 'B' })).toHaveAttribute('aria-controls', 'p1-panel');
  });
});
