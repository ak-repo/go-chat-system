/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useState, type ReactNode } from 'react';
import { applyTheme, readTheme, THEME_KEY, type Theme } from './theme';
const ThemeContext = createContext<{ theme: Theme; setTheme: (theme: Theme) => void } | null>(null);
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, updateTheme] = useState(readTheme);
  function setTheme(next: Theme) {
    applyTheme(next);
    updateTheme(next);
    try { localStorage.setItem(THEME_KEY, next); } catch { /* still works without storage */ }
  }
  return <ThemeContext.Provider value={{ theme, setTheme }}>{children}</ThemeContext.Provider>;
}
export function useTheme() {
  const context = useContext(ThemeContext);
  if (!context) throw new Error('ThemeProvider is required');
  return context;
}
