import { useTheme } from '../context/ThemeContext';
import Icon from './Icon';
export default function ThemePicker() {
  const { theme, setTheme } = useTheme();
  return <section className="theme-settings" aria-labelledby="theme-heading"><h2 id="theme-heading">Appearance</h2><p>Choose your chat theme.</p><div className="theme-options">{(['light', 'dark'] as const).map((value) => <button key={value} type="button" aria-pressed={theme === value} onClick={() => setTheme(value)}><Icon name={value === 'light' ? 'sun' : 'moon'} />{value === 'light' ? 'Light' : 'Dark'}{theme === value && <Icon name="check" />}</button>)}</div></section>;
}
