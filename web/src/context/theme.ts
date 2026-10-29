export type Theme = 'light' | 'dark';
export const THEME_KEY = 'chat-theme';
export function readTheme(): Theme {
  return 'dark';
}
export function applyTheme(_theme: Theme) {
  void _theme;
  document.documentElement.dataset.theme = 'dark';
  document.documentElement.style.colorScheme = 'dark';
}
