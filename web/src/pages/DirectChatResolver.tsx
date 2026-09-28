import { useEffect, useState } from 'react';
import { Navigate, useParams } from 'react-router-dom';
import { createOrGetConversation } from '../api/conversations';

export default function DirectChatResolver() {
  const { userId } = useParams(); const [id, setId] = useState(''); const [error, setError] = useState('');
  useEffect(() => { if (!userId) return; void createOrGetConversation(userId).then((result) => { if (result.success && result.data) setId(result.data.id); else setError(result.error || 'Conversation unavailable'); }).catch(() => setError('Conversation unavailable')); }, [userId]);
  if (id) return <Navigate to={`/conversations/${id}`} replace />;
  return <div className="app-shell flex min-h-screen items-center justify-center text-slate-300">{error || 'Opening conversation...'}</div>;
}
