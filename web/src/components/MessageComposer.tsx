import { useLayoutEffect, useRef, type FormEventHandler, type KeyboardEventHandler } from 'react';
import Icon from './Icon';
import type { ImageDraft } from '../hooks/useImageUpload';
interface Props {
 image?: ImageDraft | null; onImage?: (file: File) => void; onCancelImage?: () => void; onRetryImage?: () => void;
  value: string; disabled: boolean; onChange: (value: string) => void;
  onSubmit: FormEventHandler<HTMLFormElement>; onBlur: () => void;
  onKeyDown?: KeyboardEventHandler<HTMLTextAreaElement>;
  mentionCount?: number; activeMention?: number;
}
export default function MessageComposer({ value, disabled, onChange, onSubmit, onBlur, onKeyDown, mentionCount = 0, activeMention = 0, image, onImage, onCancelImage, onRetryImage }: Props) {
  const input = useRef<HTMLTextAreaElement>(null);
  useLayoutEffect(() => {
    if (!input.current) return;
    input.current.style.height = 'auto';
    input.current.style.height = `${Math.max(40, Math.min(160, input.current.scrollHeight))}px`;
  }, [value]);
  const canSend = !disabled && (image ? image.phase === "ready" : !!value.trim());
  return <><div className="image-draft" hidden={!image}>{image && <>
 {image.preview && <img src={image.preview} alt="Selected image" />}
 <div><p>{image.file.name}</p>{image.phase === "uploading" && <><progress aria-label="Image upload progress" max={100} value={image.progress} /><span>{image.progress}%</span></>}{image.phase === "processing" && <p role="status">Processing image…</p>}{image.phase === "ready" && <p role="status">Image ready</p>}{image.error && <p role="alert">{image.error}</p>}{image.phase === "failed" && <button type="button" onClick={onRetryImage}>Retry upload</button>}<button type="button" onClick={onCancelImage}>Remove image</button></div>
 </>}</div><form onSubmit={onSubmit} className="composer-form">
    <textarea ref={input} id="message-input" aria-label="Message" value={value} onChange={(event) => onChange(event.target.value)} onKeyDown={(event) => {
      onKeyDown?.(event);
      if (event.defaultPrevented || event.nativeEvent.isComposing) return;
      if (event.key === 'Enter' && !event.shiftKey) {
        event.preventDefault();
        if (canSend) event.currentTarget.form?.requestSubmit();
      }
    }} onBlur={onBlur} rows={1} role={mentionCount ? 'combobox' : undefined} aria-autocomplete={mentionCount ? 'list' : undefined} aria-expanded={mentionCount ? true : undefined} aria-controls={mentionCount ? 'mention-suggestions' : undefined} aria-activedescendant={mentionCount ? `mention-option-${activeMention}` : undefined} placeholder={disabled ? 'Reconnect to send' : 'Type a message'} disabled={disabled} />
    {onImage && <label className="image-picker" aria-disabled={disabled}><span>Image</span><input aria-label="Attach image" type="file" accept="image/jpeg,image/png,image/webp" disabled={disabled} onChange={(event) => { const file = event.target.files?.[0]; if (file) onImage(file); event.target.value = ""; }} /></label>}
    <button className="send-button" disabled={!canSend} aria-label="Send message" title="Send message"><Icon name="send" width={22} height={22} /></button>
  </form></>;
}
