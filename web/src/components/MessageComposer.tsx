import { useLayoutEffect, useRef, type FormEventHandler, type KeyboardEventHandler } from 'react';
import Icon from './Icon';
interface Props {
  value: string; disabled: boolean; onChange: (value: string) => void;
  onSubmit: FormEventHandler<HTMLFormElement>; onBlur: () => void;
  onKeyDown?: KeyboardEventHandler<HTMLTextAreaElement>;
  mentionCount?: number; activeMention?: number;
}
export default function MessageComposer({ value, disabled, onChange, onSubmit, onBlur, onKeyDown, mentionCount = 0, activeMention = 0 }: Props) {
  const input = useRef<HTMLTextAreaElement>(null);
  useLayoutEffect(() => {
    if (!input.current) return;
    input.current.style.height = 'auto';
    input.current.style.height = `${Math.max(40, Math.min(160, input.current.scrollHeight))}px`;
  }, [value]);
  return <form onSubmit={onSubmit} className="composer-form">
    <textarea ref={input} id="message-input" aria-label="Message" value={value} onChange={(event) => onChange(event.target.value)} onKeyDown={(event) => {
      onKeyDown?.(event);
      if (event.defaultPrevented || event.nativeEvent.isComposing) return;
      if (event.key === 'Enter' && !event.shiftKey) {
        event.preventDefault();
        if (!disabled && value.trim()) event.currentTarget.form?.requestSubmit();
      }
    }} onBlur={onBlur} rows={1} role={mentionCount ? 'combobox' : undefined} aria-autocomplete={mentionCount ? 'list' : undefined} aria-expanded={mentionCount ? true : undefined} aria-controls={mentionCount ? 'mention-suggestions' : undefined} aria-activedescendant={mentionCount ? `mention-option-${activeMention}` : undefined} placeholder={disabled ? 'Reconnect to send' : 'Type a message'} disabled={disabled} />
    <button className="send-button" disabled={disabled || !value.trim()} aria-label="Send message" title="Send message"><Icon name="send" width={22} height={22} /></button>
  </form>;
}
