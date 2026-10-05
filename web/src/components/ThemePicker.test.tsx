import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ThemeProvider } from '../context/ThemeContext';
import { applyTheme, readTheme } from '../context/theme';
import ThemePicker from './ThemePicker';
beforeEach(() => { localStorage.clear(); document.documentElement.removeAttribute('data-theme'); });
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
describe('themes', () => {
  it('defaults to light and persists dark across remounts', () => {
    applyTheme(readTheme());
    render(<ThemeProvider><ThemePicker /></ThemeProvider>);
    expect(screen.getByRole('button', { name: 'Light' })).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(screen.getByRole('button', { name: 'Dark' }));
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark');
    expect(document.documentElement.style.colorScheme).toBe('dark');
    expect(localStorage.getItem('chat-theme')).toBe('dark');
    cleanup();
    render(<ThemeProvider><ThemePicker /></ThemeProvider>);
    expect(screen.getByRole('button', { name: 'Dark' })).toHaveAttribute('aria-pressed', 'true');
  });
  it('ignores invalid saved themes and works when storage is unavailable', () => {
    localStorage.setItem('chat-theme', 'invalid');
    expect(readTheme()).toBe('light');
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('Unavailable'); });
    render(<ThemeProvider><ThemePicker /></ThemeProvider>);
    fireEvent.click(screen.getByRole('button', { name: 'Dark' }));
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark');
  });
});
