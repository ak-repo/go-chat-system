/* eslint-disable react-hooks/set-state-in-effect */
import { useEffect, useRef, useState } from 'react';
import { cancelImage, completeImage, imageStatus, initiateImage, uploadImage, type Attachment } from '../api/attachments';

export interface ImageDraft { file: File; preview: string; progress: number; phase: 'uploading' | 'processing' | 'ready' | 'failed'; attachment?: Attachment; error?: string }
function delay(signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const abort = () => { window.clearTimeout(timer); reject(new DOMException('Cancelled', 'AbortError')); };
    const timer = window.setTimeout(() => { signal.removeEventListener('abort', abort); resolve(); }, 2000);
    signal.addEventListener('abort', abort, { once: true });
    if (signal.aborted) abort();
  });
}
export function useImageUpload(conversationId: string) {
  const [draft, setDraft] = useState<ImageDraft | null>(null);
  const active = useRef<{ controller: AbortController; id?: string; preview: string } | null>(null);
  const clear = (discard = true) => {
    const current = active.current;
    active.current = null;
    current?.controller.abort();
    if (current) URL.revokeObjectURL(current.preview);
    if (discard && current?.id) void cancelImage(current.id).catch(() => { /* server cleanup reclaims abandoned uploads */ });
    setDraft(null);
  };
  useEffect(() => {
    setDraft(null);
    return () => {
      const current = active.current;
      active.current = null;
      current?.controller.abort();
      if (current) URL.revokeObjectURL(current.preview);
      if (current?.id) void cancelImage(current.id).catch(() => {});
    };
  }, [conversationId]);
  const select = async (file: File) => {
    clear();
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type) || file.size === 0 || file.size > 10 * 1024 * 1024) {
      setDraft({ file, preview: '', progress: 0, phase: 'failed', error: 'Choose a JPEG, PNG or WebP image up to 10 MiB.' });
      return;
    }
    const current = { controller: new AbortController(), preview: URL.createObjectURL(file), id: undefined as string | undefined };
    active.current = current;
    setDraft({ file, preview: current.preview, progress: 0, phase: 'uploading' });
    const update = (patch: Partial<ImageDraft>) => { if (active.current === current) setDraft((previous) => previous ? { ...previous, ...patch } : previous); };
    try {
      const ticket = await initiateImage(conversationId, file, current.controller.signal);
      current.id = ticket.attachment.id;
      if (active.current !== current) { void cancelImage(current.id).catch(() => {}); return; }
      await uploadImage(ticket, file, current.controller.signal, (progress) => update({ progress }));
      update({ phase: 'processing', progress: 100 });
      let attachment = await completeImage(current.id, current.controller.signal);
      while (attachment.state === 'queued' || attachment.state === 'processing') {
        await delay(current.controller.signal);
        attachment = await imageStatus(current.id, current.controller.signal);
      }
      if (attachment.state !== 'ready') throw new Error(attachment.error || 'Image processing failed. Try another image.');
      update({ phase: 'ready', attachment });
    } catch (error) {
      if (!current.controller.signal.aborted) update({ phase: 'failed', error: error instanceof Error ? error.message : 'Image upload failed' });
    }
  };
  return { draft, select, cancel: () => clear(), consume: () => clear(false), retry: () => { if (draft) void select(draft.file); } };
}
