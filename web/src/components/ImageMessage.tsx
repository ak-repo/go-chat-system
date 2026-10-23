import { useEffect, useRef, useState } from 'react';
import { imageURL, type Attachment } from '../api/attachments';

export default function ImageMessage({ messageId, attachment }: { messageId: string; attachment: Attachment }) {
  const [thumbnail, setThumbnail] = useState('');
  const [full, setFull] = useState('');
  const [error, setError] = useState('');
  const [visible, setVisible] = useState(false);
  const target = useRef<HTMLDivElement>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const opener = useRef<HTMLButtonElement>(null);
  const refreshed = useRef(false);
  const fullRefreshed = useRef(false);
  const [fullError, setFullError] = useState(false);
  useEffect(() => {
    if (!target.current || typeof IntersectionObserver === 'undefined') return;
    const observer = new IntersectionObserver(([entry]) => { if (entry.isIntersecting) { setVisible(true); observer.disconnect(); } });
    observer.observe(target.current);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    if (!visible && typeof IntersectionObserver !== 'undefined') return;
    let cancelled = false;
    void imageURL(messageId, attachment.id, 'thumbnail').then((url) => { if (!cancelled) { refreshed.current = false; setError(''); setThumbnail(url); } }).catch(() => { if (!cancelled) setError('Image unavailable'); });
    return () => { cancelled = true; };
  }, [messageId, attachment.id, visible]);
  const refresh = async () => {
    try { setError(''); setThumbnail(await imageURL(messageId, attachment.id, 'thumbnail', false, true)); }
    catch { setError('Image unavailable'); }
  };
  const view = async () => {
    fullRefreshed.current = false;
    setFullError(false);
    try { setFull(await imageURL(messageId, attachment.id, 'full')); dialog.current?.showModal(); }
    catch { setError('Image unavailable'); }
  };
  const download = async () => {
    try { const url = await imageURL(messageId, attachment.id, 'full', true); const link = document.createElement('a'); link.href = url; link.download = attachment.filename; link.click(); }
    catch { setError('Download unavailable'); }
  };
  return <div ref={target} className="image-message">
    <button ref={opener} type="button" aria-label={`View image: ${attachment.filename}`} onClick={() => void view()} style={{ aspectRatio: `${attachment.width} / ${attachment.height}` }}>
      {thumbnail ? <img src={thumbnail} alt={attachment.filename} width={attachment.width} height={attachment.height} loading="lazy" onError={() => { if (!refreshed.current) { refreshed.current = true; void refresh(); } else setError('Image unavailable'); }} /> : <span>Image</span>}
    </button>
    {error && <p role="status">{error} <button type="button" onClick={() => { refreshed.current = false; void refresh(); }}>Retry</button></p>}
    <button type="button" className="image-download" onClick={() => void download()}>Download image</button>
    <dialog ref={dialog} className="image-viewer" aria-label={attachment.filename} onClose={() => opener.current?.focus()} onClick={(event) => { if (event.target === event.currentTarget) dialog.current?.close(); }}>
      <button autoFocus type="button" className="soft-button" onClick={() => dialog.current?.close()}>Close image</button>
      {fullError && <p role="status">Image unavailable <button type="button" onClick={() => {
        fullRefreshed.current = false;
        setFullError(false);
        void imageURL(messageId, attachment.id, 'full', false, true).then(setFull).catch(() => setFullError(true));
      }}>Retry full image</button></p>}
      {full && !fullError && <img src={full} alt={attachment.filename} onError={() => {
        if (fullRefreshed.current) { setFullError(true); return; }
        fullRefreshed.current = true;
        void imageURL(messageId, attachment.id, 'full', false, true).then(setFull).catch(() => setFullError(true));
      }} />}
      <button type="button" className="soft-button" onClick={() => void download()}>Download image</button>
    </dialog>
  </div>;
}
