import { afterEach, describe, expect, it, vi } from 'vitest';
import { applyTheme, readTheme } from '../context/theme';
afterEach(() => { localStorage.clear(); vi.restoreAllMocks(); });
describe('dark workspace', () => {
  it('ignores a previously saved light preference', () => {
    localStorage.setItem('chat-theme', 'light');
    expect(readTheme()).toBe('dark');
    applyTheme('light');
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(document.documentElement.style.colorScheme).toBe('dark');
  });
  it('works without browser storage', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('Unavailable'); });
    expect(readTheme()).toBe('dark');
  });
});
