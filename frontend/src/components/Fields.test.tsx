import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { SelectField } from './SelectField';
import { TextField } from './TextField';
import { Toggle } from './Toggle';

describe('form primitives', () => {
  it('TextField shows an inline error wired to the input', () => {
    render(<TextField label="Port" error="Port must be a whole number between 1024 and 65535" defaultValue="80" />);
    const input = screen.getByLabelText('Port');
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(input).toHaveAccessibleDescription('Port must be a whole number between 1024 and 65535');
  });
  it('SelectField is labelled and lists its options', () => {
    render(<SelectField label="Level" options={[{ value: 'INFO', label: 'Info' }, { value: 'DEBUG', label: 'Debug' }]} defaultValue="INFO" />);
    expect(screen.getByLabelText('Level')).toHaveValue('INFO');
    expect(screen.getAllByRole('option')).toHaveLength(2);
  });
  it('Toggle is a labelled switch operable by click and Space', async () => {
    function Host() {
      const [on, setOn] = useState(false);
      return <Toggle label="Dark theme" checked={on} onChange={setOn} />;
    }
    const user = userEvent.setup();
    render(<Host />);
    const sw = screen.getByRole('switch', { name: 'Dark theme' });
    await user.click(sw);
    expect(sw).toHaveAttribute('aria-checked', 'true');
    await user.keyboard(' ');
    expect(sw).toHaveAttribute('aria-checked', 'false');
  });
});

describe('form primitives fix round 1', () => {
  it('Toggle description is wired with aria-describedby', () => {
    render(<Toggle label="Dark" description="Uses the dark palette" checked={false} onChange={() => {}} />);
    expect(screen.getByRole('switch')).toHaveAccessibleDescription('Uses the dark palette');
  });
  it('TextField merges a caller aria-describedby with hint and error', () => {
    render(<><p id="extra">Extra help</p><TextField label="Port" hint="Hint" error="Bad" aria-describedby="extra" /></>);
    expect(screen.getByLabelText('Port')).toHaveAccessibleDescription('Extra help Hint Bad');
    expect(screen.getByRole('alert')).toHaveTextContent('Bad');
  });
  it('SelectField merges a caller aria-describedby with the error', () => {
    render(<><p id="extra">Extra help</p><SelectField label="L" options={[{ value: 'a', label: 'A' }]} error="Bad" aria-describedby="extra" /></>);
    expect(screen.getByLabelText('L')).toHaveAccessibleDescription('Extra help Bad');
  });
});

describe('TextField aria-invalid (P0-4 Task 9 fix round 2, minor 3)', () => {
  it('an error keeps aria-invalid even when the caller passes undefined or false', () => {
    render(
      <>
        <TextField label="One" error="Bad one" aria-invalid={undefined} />
        <TextField label="Two" error="Bad two" aria-invalid={false} />
        <TextField label="Three" aria-invalid />
        <TextField label="Four" />
      </>,
    );
    expect(screen.getByLabelText('One')).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByLabelText('Two')).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByLabelText('Three')).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByLabelText('Four')).not.toHaveAttribute('aria-invalid');
  });
});
