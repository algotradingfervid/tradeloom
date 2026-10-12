import { expect, it } from 'vitest';
import golden from '../../../../internal/settings/testdata/shortcuts.golden.json';
import { shortcutFixture } from './fixtures';

// internal/settings TestDefaultShortcuts_Golden keeps the golden file equal to the Go registry, so the mock
// fixture cannot drift from DefaultShortcuts() (P0-4 Ruling 83-F I3).
it('shortcutFixture mirrors the Go shortcut registry exactly (ids, labels, scopes, defaults, order)', () => {
  expect(shortcutFixture).toEqual(golden);
});
