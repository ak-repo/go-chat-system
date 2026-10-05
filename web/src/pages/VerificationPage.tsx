import { useState, type FormEvent } from 'react';
import { Link, useLocation, useSearchParams } from 'react-router-dom';
import { confirmVerification, requestVerification } from '../api/auth';

export default function VerificationPage() {
  const [params] = useSearchParams();
  const location = useLocation();
  const [email, setEmail] = useState(() => params.get('email') ?? '');
  const [token, setToken] = useState(() => params.get('token') ?? '');
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const submit = async (event: FormEvent) => { event.preventDefault(); setLoading(true); setError(''); try { const result = await requestVerification(email); if (result.success) setMessage(result.data?.message || 'If the account exists, verification instructions were sent.'); else setError(result.error || 'Could not request verification'); } catch { setError('Could not request verification'); } finally { setLoading(false); } };
  const verify = async (event: FormEvent) => { event.preventDefault(); setLoading(true); setError(''); try { const result = await confirmVerification(token); if (result.success) setMessage('Account verified.'); else setError(result.error || 'Invalid or expired verification token'); } catch { setError('Invalid or expired verification token'); } finally { setLoading(false); } };
  return <div className="app-shell flex min-h-screen items-center justify-center p-4"><div className="app-card w-full max-w-md p-7 sm:p-9"><h1 className="mb-7 text-2xl font-bold text-primary">Verify account</h1>{error && <div className="mb-4 rounded-xl border border-danger bg-danger-soft p-3 text-danger">{error}</div>}{message && <div className="mb-4 rounded-xl border border-accent bg-accent-soft p-3 text-accent">{message}</div>}<form onSubmit={submit} className="space-y-4"><input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required className="app-input w-full px-3 py-3" placeholder="Email" /><button disabled={loading} className="primary-button w-full py-3 font-semibold disabled:opacity-50">Send verification instructions</button></form><form onSubmit={verify} className="space-y-4 border-t border-theme mt-8 pt-8"><input value={token} onChange={(e) => setToken(e.target.value)} required className="app-input w-full px-3 py-3" placeholder="Verification token" /><button disabled={loading} className="w-full rounded-xl bg-raised py-3 font-semibold text-primary hover-surface disabled:opacity-50">Verify account</button></form><Link to="/login" state={location.state} className="mt-5 block text-sm font-semibold text-accent hover-accent">Back to login</Link></div></div>;
}
