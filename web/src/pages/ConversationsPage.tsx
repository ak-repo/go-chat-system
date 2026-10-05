import Icon from '../components/Icon';
import { useConversations } from '../context/ConversationContext';
import { appName } from '../config/app';
export default function ConversationsPage() {
  const { setNewChat } = useConversations();
  return <main className="chat-welcome"><div className="welcome-art" aria-hidden="true"><div className="welcome-screen"><Icon name="chats" width={88} height={88} /><span /><span /></div><div className="welcome-badge"><Icon name="check" width={32} height={32} /></div></div><h1>{appName} for desktop</h1><p>Keep your conversations close.<br />Choose a chat or start a new one with a friend.</p><button className="primary-button" onClick={() => setNewChat(true)}>Start a new chat</button><span className="welcome-footer">Your personal chat workspace</span></main>;
}
