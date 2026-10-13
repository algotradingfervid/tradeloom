import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { NumberCommitField } from './SettingRow';

describe('NumberCommitField draft (Task 19: no mount-time reset)', () => {
  const field = (value: number) => <NumberCommitField label="Port" value={value} onCommit={vi.fn(async () => null)} />;

  it('a re-render with the same value keeps what the user typed', () => {
    const { rerender } = render(field(7420));
    const input = screen.getByRole('textbox', { name: 'Port' });
    fireEvent.change(input, { target: { value: '80' } });
    rerender(field(7420));
    expect(input).toHaveValue('80');
  });

  it('a new value from outside replaces the draft', () => {
    const { rerender } = render(field(7420));
    const input = screen.getByRole('textbox', { name: 'Port' });
    fireEvent.change(input, { target: { value: '80' } });
    rerender(field(8080));
    expect(input).toHaveValue('8080');
  });
});
