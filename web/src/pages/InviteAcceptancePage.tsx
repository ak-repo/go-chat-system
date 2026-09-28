import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { acceptGroupInvite } from '../api/conversations';

export default function InviteAcceptancePage() {
  const { token } = useParams();
  const [conversationId, setConversationId] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    if (!token) return;
    void acceptGroupInvite(token).then((result) => {
      if (result.data) setConversationId(result.data.conversation_id);
      else setError(result.error || 'This invite is unavailable');
    }).catch(() => setError('This invite is unavailable'));
  }, [token]);

  if (!token) return <main className="app-shell flex min-h-screen items-center justify-center text-red-300">Invalid invite link</main>;

  return <main className="app-shell flex min-h-screen items-center justify-center p-4"><section className="app-card w-full max-w-md p-6 text-center"><h1 className="text-2xl font-bold text-white">Group invitation</h1>{conversationId ? <><p className="mt-3 text-slate-300">You joined the group.</p><Link className="primary-button mt-5 inline-block px-4 py-2" to={`/conversations/${conversationId}`}>Open group</Link></> : error ? <><p className="mt-3 text-red-300">{error}</p><Link className="soft-button mt-5 inline-block px-4 py-2" to="/conversations">Back to conversations</Link></> : <p className="mt-3 text-slate-300">Accepting invite...</p>}</section></main>;
}
