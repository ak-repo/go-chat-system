import { useEffect, useState } from 'react';
import { getNotificationPreferences, updateNotificationPreferences, type NotificationPreferences } from '../api/notifications';
export default function NotificationPreferencesPanel() {
 const [prefs,setPrefs]=useState<NotificationPreferences | null>(null); const [error,setError]=useState(''); const [busy,setBusy]=useState(false);
 useEffect(() => { let active=true; void getNotificationPreferences().then(r => { if(active) { if(r.data) setPrefs(r.data); else setError(r.error || 'Could not load preferences'); } }).catch(() => { if(active) setError('Could not load preferences'); }); return () => {active=false;}; },[]);
 async function toggle(key:keyof NotificationPreferences) { if(!prefs) return; setBusy(true); setError(''); try { const r=await updateNotificationPreferences({[key]:!prefs[key]}); if(r.data) setPrefs(r.data); else setError(r.error || 'Could not save preferences'); } catch { setError('Could not save preferences'); } finally {setBusy(false);} }
 return <section className="preferences-panel"><h2>Notification preferences</h2>{error && <p role="alert">{error}</p>}{!prefs && !error && <p role="status">Loading preferences…</p>}{prefs && (['message_enabled','reply_enabled','mention_enabled'] as const).map(key => <label key={key}><input type="checkbox" disabled={busy} checked={prefs[key]} onChange={() => void toggle(key)}/>{key.replace('_enabled','')} notifications</label>)}</section>;
}
