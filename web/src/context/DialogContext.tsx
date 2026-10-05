/* eslint-disable react-refresh/only-export-components */
import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import Icon from '../components/Icon';
type Request = { title: string; initial?: string; resolve: (value: string | null) => void };
const DialogContext = createContext<{ confirm: (title: string) => Promise<boolean>; askText: (title: string, initial: string) => Promise<string | null> } | null>(null);
export function DialogProvider({ children }: { children: ReactNode }) {
  const [request, setRequest] = useState<Request | null>(null);
  const [value, setValue] = useState('');
  const panel = useRef<HTMLDivElement>(null);
  const pending = useRef<Request | null>(null);
  const ask = useCallback((title: string, initial?: string) => new Promise<string | null>((resolve) => {
    if (pending.current) { resolve(null); return; }
    const next = { title, initial, resolve };
    pending.current = next;
    setValue(initial ?? '');
    setRequest(next);
  }), []);
  const finish = useCallback((answer: string | null) => {
    pending.current?.resolve(answer);
    pending.current = null;
    setRequest(null);
  }, []);
  useEffect(() => () => pending.current?.resolve(null), []);
  useEffect(() => {
    if (!request) return;
    const previous = document.activeElement as HTMLElement | null;
    const target = panel.current;
    const controls = () => [...target!.querySelectorAll<HTMLElement>('button, textarea, input, a[href]')].filter((element) => !element.hasAttribute('disabled'));
    (target?.querySelector<HTMLElement>('textarea') ?? controls()[0])?.focus();
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { event.preventDefault(); finish(null); }
      if (event.key === 'Tab') {
        const items = controls();
        const first = items[0], last = items.at(-1);
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
      }
    };
    document.addEventListener('keydown', onKey);
    return () => { document.removeEventListener('keydown', onKey); previous?.focus(); };
  }, [request, finish]);
  const confirm = useCallback(async (title: string) => (await ask(title)) !== null, [ask]);
  const askText = useCallback((title: string, initial: string) => ask(title, initial), [ask]);
  return <DialogContext.Provider value={{ confirm, askText }}>{children}{request && createPortal(<div className="dialog-backdrop" onClick={(event) => { if (event.target === event.currentTarget) finish(null); }}><div ref={panel} className="dialog-panel" role="dialog" aria-modal="true" aria-labelledby="dialog-title"><header><h2 id="dialog-title">{request.title}</h2><button className="icon-button" aria-label="Close dialog" onClick={() => finish(null)}><Icon name="close" /></button></header><form onSubmit={(event) => { event.preventDefault(); finish(request.initial !== undefined ? value : 'confirmed'); }}>{request.initial !== undefined && <textarea aria-label={request.title} className="app-input" value={value} onChange={(event) => setValue(event.target.value)} rows={4} required />}<footer><button type="button" className="soft-button" onClick={() => finish(null)}>Cancel</button><button className="primary-button">{request.initial !== undefined ? 'Save' : 'Confirm'}</button></footer></form></div></div>, document.body)}</DialogContext.Provider>;
}
export function useDialogs() {
  const context = useContext(DialogContext);
  if (!context) throw new Error('DialogProvider is required');
  return context;
}
