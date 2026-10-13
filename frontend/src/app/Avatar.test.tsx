import { fireEvent, render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { demoSeed } from '../api/mock/fixtures';
import { Avatar } from './ProfileMenu';

const withUrl = (avatarUrl: string) => ({ ...demoSeed().profile, avatarUrl });

describe('Avatar (Ruling 90 M3: remote avatar offline or blocked by a CSP)', () => {
  it('shows the remote picture while it loads', () => {
    const { container } = render(<Avatar profile={withUrl('https://example.invalid/a.png')} />);
    expect(container.querySelector('img.avatar')).toHaveAttribute('src', 'https://example.invalid/a.png');
  });
  it('falls back to the initials when the picture fails to load, never a broken image', () => {
    const { container } = render(<Avatar profile={withUrl('https://example.invalid/a.png')} />);
    fireEvent.error(container.querySelector('img.avatar')!);
    expect(container.querySelector('img')).toBeNull();
    expect(container.querySelector('.avatar')).toHaveTextContent('AR');
  });
  it('tries again when the profile brings a different picture', () => {
    const { container, rerender } = render(<Avatar profile={withUrl('https://example.invalid/a.png')} />);
    fireEvent.error(container.querySelector('img.avatar')!);
    rerender(<Avatar profile={withUrl('https://example.invalid/b.png')} />);
    expect(container.querySelector('img.avatar')).toHaveAttribute('src', 'https://example.invalid/b.png');
  });
});
