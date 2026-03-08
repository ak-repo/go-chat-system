import { useCallback, useEffect, useRef, useState } from "react";
import { useAuth } from "../context/context";
import { getAuthToken } from "../api/api";
import {
  getChatsService,
  getChatMessagesService,
  getOrCreateDMChatService,
} from "../api/services";

const WS_BASE = "ws://localhost:8002/api/v1/ws";
const MESSAGES_PAGE_SIZE = 30;
const MAX_MESSAGES_IN_STATE = 200;

export default function Chat() {
  const wsRef = useRef(null);
  const messagesContainerRef = useRef(null);
  const { user } = useAuth();

  const [chats, setChats] = useState([]);
  const [selectedChat, setSelectedChat] = useState(null);
  const [messages, setMessages] = useState([]);
  const [text, setText] = useState("");
  const [loadingChats, setLoadingChats] = useState(false);
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMoreHistory, setHasMoreHistory] = useState(true);
  const [newDmUserId, setNewDmUserId] = useState("");

  const loadChats = useCallback(async () => {
    if (!user?.id) return;
    setLoadingChats(true);
    try {
      const res = await getChatsService();
      setChats(res?.data?.chats ?? []);
    } catch (e) {
      console.error("Failed to load chats", e);
    } finally {
      setLoadingChats(false);
    }
  }, [user?.id]);

  useEffect(() => {
    loadChats();
  }, [loadChats]);

  useEffect(() => {
    if (!selectedChat?.id) {
      setMessages([]);
      setHasMoreHistory(true);
      return;
    }
    let cancelled = false;
    setLoadingMessages(true);
    setHasMoreHistory(true);
    getChatMessagesService(selectedChat.id, { limit: MESSAGES_PAGE_SIZE })
      .then((res) => {
        if (cancelled) return;
        const list = res?.data?.messages ?? [];
        setMessages(list.reverse());
        setHasMoreHistory(list.length >= MESSAGES_PAGE_SIZE);
      })
      .catch((e) => console.error("Failed to load messages", e))
      .finally(() => {
        if (!cancelled) setLoadingMessages(false);
      });
    return () => { cancelled = true; };
  }, [selectedChat?.id]);

  const loadMoreMessages = useCallback(() => {
    if (!selectedChat?.id || loadingMore || !hasMoreHistory || messages.length === 0) return;
    const oldestId = messages[0]?.id;
    if (!oldestId) return;
    setLoadingMore(true);
    getChatMessagesService(selectedChat.id, { limit: MESSAGES_PAGE_SIZE, before: oldestId })
      .then((res) => {
        const list = res?.data?.messages ?? [];
        if (list.length < MESSAGES_PAGE_SIZE) setHasMoreHistory(false);
        setMessages((prev) => {
          const combined = [...list.reverse(), ...prev];
          if (combined.length > MAX_MESSAGES_IN_STATE) {
            return combined.slice(0, MAX_MESSAGES_IN_STATE);
          }
          return combined;
        });
      })
      .catch((e) => console.error("Failed to load more messages", e))
      .finally(() => setLoadingMore(false));
  }, [selectedChat?.id, loadingMore, hasMoreHistory, messages]);

  const handleScroll = useCallback(() => {
    const el = messagesContainerRef.current;
    if (!el || !hasMoreHistory || loadingMore) return;
    if (el.scrollTop < 80) loadMoreMessages();
  }, [hasMoreHistory, loadingMore, loadMoreMessages]);

  useEffect(() => {
    if (!user?.id) return;
    const token = getAuthToken();
    if (!token) return;

    const url = `${WS_BASE}?token=${encodeURIComponent(token)}`;
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log("WS connected");
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.event !== "chat.message") return;
        const data = msg.data;
        const chatId = typeof data === "object" && data !== null ? data.chat_id : null;
        if (chatId && selectedChat?.id === chatId) {
          setMessages((prev) => {
            const m = normalizeWsMessage(msg);
            if (prev.some((x) => x.id === m.id)) return prev;
            return [...prev, m];
          });
        }
      } catch (_) {}
    };

    ws.onclose = () => {
      console.log("WS closed");
    };

    return () => ws.close();
  }, [user?.id, selectedChat?.id]);

  const startDm = async () => {
    const otherId = newDmUserId.trim();
    if (!otherId) return;
    try {
      const res = await getOrCreateDMChatService(otherId);
      const chat = res?.data?.chat;
      if (chat) {
        await loadChats();
        setSelectedChat({ id: chat.id, otherUserId: otherId });
        setNewDmUserId("");
      }
    } catch (e) {
      console.error("Failed to start DM", e);
    }
  };

  const sendMessage = () => {
    if (!text || !wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;
    if (!selectedChat?.otherUserId) return;

    const payload = {
      event: "chat.message",
      receiver_id: selectedChat.otherUserId,
      receiver_type: "user",
      data: { text },
    };

    wsRef.current.send(JSON.stringify(payload));
    setText("");
  };

  const normalizedChats = chats.map((c) => {
    const otherUserId = (c.member_ids || []).find((id) => id !== user?.id);
    return { ...c, otherUserId };
  });

  return (
    <div style={{ display: "flex", height: "80vh", padding: 16 }}>
      <aside style={{ width: 260, borderRight: "1px solid #eee", paddingRight: 16 }}>
        <h3 style={{ marginTop: 0 }}>Conversations</h3>
        {loadingChats ? (
          <p>Loading...</p>
        ) : (
          <ul style={{ listStyle: "none", padding: 0 }}>
            {normalizedChats.map((c) => (
              <li key={c.id}>
                <button
                  type="button"
                  onClick={() =>
                    setSelectedChat({ id: c.id, otherUserId: c.otherUserId })
                  }
                  style={{
                    width: "100%",
                    textAlign: "left",
                    padding: "8px 12px",
                    marginBottom: 4,
                    background:
                      selectedChat?.id === c.id ? "#e0e0e0" : "transparent",
                    border: "1px solid #ddd",
                    borderRadius: 4,
                    cursor: "pointer",
                  }}
                >
                  Chat {c.otherUserId ? c.otherUserId.slice(0, 8) + "…" : c.id.slice(0, 8)}
                </button>
              </li>
            ))}
          </ul>
        )}
        <div style={{ marginTop: 12 }}>
          <input
            value={newDmUserId}
            onChange={(e) => setNewDmUserId(e.target.value)}
            placeholder="Other user ID for new DM"
            style={{ width: "100%", padding: 6, marginBottom: 4 }}
          />
          <button type="button" onClick={startDm} style={{ width: "100%" }}>
            Start DM
          </button>
        </div>
        {!loadingChats && normalizedChats.length === 0 && (
          <p style={{ color: "#666", marginTop: 8 }}>No conversations yet. Enter a user ID above to start a DM.</p>
        )}
      </aside>

      <main style={{ flex: 1, display: "flex", flexDirection: "column", marginLeft: 16 }}>
        {selectedChat ? (
          <>
            <div style={{ marginBottom: 12 }}>
              <strong>Chat</strong> (to: {selectedChat.otherUserId?.slice(0, 8)}…)
            </div>
            <div
              ref={messagesContainerRef}
              onScroll={handleScroll}
              style={{
                flex: 1,
                overflow: "auto",
                border: "1px solid #eee",
                borderRadius: 4,
                padding: 12,
                marginBottom: 12,
              }}
            >
              {loadingMessages ? (
                <p>Loading messages...</p>
              ) : (
                <>
                  {hasMoreHistory && (
                    <div style={{ textAlign: "center", padding: 8 }}>
                      {loadingMore ? (
                        <span>Loading older messages...</span>
                      ) : (
                        <button type="button" onClick={loadMoreMessages} style={{ fontSize: 12 }}>
                          Load older messages
                        </button>
                      )}
                    </div>
                  )}
                  <ul style={{ listStyle: "none", padding: 0 }}>
                  {messages.map((m) => (
                    <li
                      key={m.id}
                      style={{
                        marginBottom: 8,
                        textAlign: m.sender_id === user?.id ? "right" : "left",
                      }}
                    >
                      <span style={{ fontWeight: 600 }}>
                        {m.sender_id === user?.id ? "You" : m.sender_id?.slice(0, 8)}
                      </span>
                      : {m.content}
                      <span style={{ fontSize: 12, color: "#666", marginLeft: 8 }}>
                        {m.created_at
                          ? new Date(m.created_at).toLocaleTimeString()
                          : ""}
                      </span>
                    </li>
                  ))}
                </ul>
                </>
              )}
            </div>
            <div style={{ display: "flex", gap: 8 }}>
              <input
                value={text}
                onChange={(e) => setText(e.target.value)}
                placeholder="Type message"
                style={{ flex: 1, padding: 8 }}
                onKeyDown={(e) => e.key === "Enter" && sendMessage()}
              />
              <button type="button" onClick={sendMessage}>
                Send
              </button>
            </div>
          </>
        ) : (
          <p style={{ color: "#666" }}>Select a conversation or start a new DM.</p>
        )}
      </main>
    </div>
  );
}

function normalizeWsMessage(msg) {
  const data = msg.data;
  if (typeof data === "object" && data !== null && data.id) {
    return {
      id: data.id,
      chat_id: data.chat_id,
      sender_id: msg.sender_id,
      content: data.content ?? data.text,
      created_at: data.created_at,
    };
  }
  return {
    id: `tmp-${Date.now()}`,
    chat_id: null,
    sender_id: msg.sender_id,
    content: data?.text ?? data?.content ?? "",
    created_at: new Date().toISOString(),
  };
}
