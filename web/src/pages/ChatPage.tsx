import { useState, useEffect, useRef, useCallback, type FormEvent } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { useSocket } from '../context/SocketContext';
import { getMessages, type Message } from '../api';

export default function ChatPage() {
  const { userId } = useParams<{ userId: string }>();
  const navigate = useNavigate();
  const { user, logout } = useAuth();
  const { sendMessage, sendTyping, onMessage, onTyping, isConnected } = useSocket();

  const [messages, setMessages] = useState<Message[]>([]);
  const [inputMessage, setInputMessage] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [partnerTyping, setPartnerTyping] = useState(false);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const typingTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const loadMessages = useCallback(async () => {
    if (!userId) return;
    setLoading(true);
    try {
      const response = await getMessages(userId, 50, 0);
      if (response.success && response.data) {
        setMessages(response.data.messages);
      }
    } catch {
      setError('Failed to load messages');
    } finally {
      setLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    if (!user) {
      navigate('/login', { replace: true });
      return;
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect
    loadMessages();
  }, [user, userId, navigate, loadMessages]);

  // Set up WebSocket listeners
  useEffect(() => {
    const unsubscribeMessage = onMessage((data) => {
      const msg = data as { message_id: string; content: string; timestamp: string };
      const newMsg: Message = {
        id: msg.message_id,
        sender_id: userId || '',
        receiver_id: user?.id || '',
        body: msg.content,
        is_group: false,
        created_at: msg.timestamp,
        modified_at: msg.timestamp,
      };
      setMessages((prev) => [...prev, newMsg]);
    });

    const unsubscribeTyping = onTyping((data) => {
      const typing = data as { is_typing: boolean };
      setPartnerTyping(typing.is_typing);
    });

    return () => {
      unsubscribeMessage();
      unsubscribeTyping();
    };
  }, [userId, user?.id, onMessage, onTyping]);

  // Auto-scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const handleSend = (e: FormEvent) => {
    e.preventDefault();
    if (!inputMessage.trim() || !userId) return;

    sendMessage(userId, inputMessage.trim());
    setInputMessage('');

    // Send "not typing" indicator
    sendTyping(userId, false);
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setInputMessage(e.target.value);

    // Send typing indicator
    if (userId) {
      sendTyping(userId, true);

      // Clear previous timeout
      if (typingTimeoutRef.current) {
        clearTimeout(typingTimeoutRef.current);
      }

      // Set timeout to stop typing indicator
      typingTimeoutRef.current = setTimeout(() => {
        sendTyping(userId, false);
      }, 2000);
    }
  };

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const formatTime = (dateStr: string) => {
    const date = new Date(dateStr);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-lg">Loading...</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      {/* Header */}
      <header className="bg-white shadow">
        <div className="max-w-4xl mx-auto px-4 py-4 flex justify-between items-center">
          <div className="flex items-center gap-4">
            <Link
              to="/friends"
              className="text-purple-600 hover:text-purple-700"
            >
              ← Back
            </Link>
            <h1 className="text-xl font-bold">Chat</h1>
          </div>
          <div className="flex items-center gap-4">
            <span
              className={`w-2 h-2 rounded-full ${
                isConnected ? 'bg-green-500' : 'bg-red-500'
              }`}
              title={isConnected ? 'Connected' : 'Disconnected'}
            />
            <span className="text-gray-600">{user?.username}</span>
            <button
              onClick={handleLogout}
              className="text-sm text-red-600 hover:underline"
            >
              Logout
            </button>
          </div>
        </div>
      </header>

      {error && (
        <div className="max-w-4xl mx-auto px-4 mt-4 p-3 bg-red-100 text-red-700 rounded">
          {error}
        </div>
      )}

      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4">
        <div className="max-w-4xl mx-auto space-y-4">
          {messages.length === 0 ? (
            <div className="text-center text-gray-500 py-8">
              No messages yet. Start the conversation!
            </div>
          ) : (
            messages.map((msg) => {
              const isOwn = msg.sender_id === user?.id;
              return (
                <div
                  key={msg.id}
                  className={`flex ${isOwn ? 'justify-end' : 'justify-start'}`}
                >
                  <div
                    className={`max-w-xs md:max-w-md px-4 py-2 rounded-lg ${
                      isOwn
                        ? 'bg-purple-600 text-white'
                        : 'bg-gray-200 text-gray-800'
                    }`}
                  >
                    <div>{msg.body}</div>
                    <div
                      className={`text-xs mt-1 ${
                        isOwn ? 'text-purple-200' : 'text-gray-500'
                      }`}
                    >
                      {formatTime(msg.created_at)}
                    </div>
                  </div>
                </div>
              );
            })
          )}
          {partnerTyping && (
            <div className="text-gray-500 text-sm italic">
              Partner is typing...
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>
      </div>

      {/* Input */}
      <div className="bg-white border-t p-4">
        <form
          onSubmit={handleSend}
          className="max-w-4xl mx-auto flex gap-2"
        >
          <input
            type="text"
            value={inputMessage}
            onChange={handleInputChange}
            placeholder="Type a message..."
            className="flex-1 px-4 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-purple-500"
          />
          <button
            type="submit"
            disabled={!inputMessage.trim()}
            className="px-6 py-2 bg-purple-600 text-white rounded-md hover:bg-purple-700 disabled:opacity-50"
          >
            Send
          </button>
        </form>
      </div>
    </div>
  );
}
