import { useConversations } from '../context/ConversationContext';
import { useEffect, useEffectEvent, useState } from 'react';
import { Navigate, useParams } from 'react-router-dom';
import { createOrGetConversation } from '../api/conversations';

export default function DirectChatResolver() {
  const { refresh } = useConversations();
  const refreshList = useEffectEvent(refresh);
  const { userId } = useParams(); const [id, setId] = useState(''); const [error, setError] = useState('');
  useEffect(() => { if (!userId) return; void createOrGetConversation(userId).then((result) => { if (result.success && result.data) { refreshList(); setId(result.data.id); } else setError(result.error || 'Conversation unavailable'); }).catch(() => setError('Conversation unavailable')); }, [userId]);
  if (id) return <Navigate to={`/conversations/${id}`} replace />;
  return <div className="app-shell flex min-h-screen items-center justify-center text-secondary">{error || 'Opening conversation...'}</div>;
}
