import { useEffect, useRef, useState } from "react";
import { useAuth } from "../context/context";
import { getToken, get } from "../api/api";
import { getFriends, getMessages } from "../api/services";

export default function Chat() {
  const wsRef = useRef(null);
  const { user, logout } = useAuth();

  const [friends, setFriends] = useState([]);
  const [selectedFriend, setSelectedFriend] = useState(null);
  const [text, setText] = useState("");
  const [messages, setMessages] = useState([]);
  const [onlineUsers, setOnlineUsers] = useState(new Set());
  const [loadingHistory, setLoadingHistory] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const [messageOffset, setMessageOffset] = useState(0);

  useEffect(() => {
    loadFriends();
  }, []);

  useEffect(() => {
    if (!user?.id) return;

    const token = getToken();
    const ws = new WebSocket(`ws://localhost:8002/api/v1/ws?token=${token}`);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log("WS connected");
      ws.send(JSON.stringify({ event: "user_online", user_id: user.id }));
    };

    ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);

      switch (msg.event) {
        case "chat.message":
          if (
            msg.sender_id === selectedFriend?.id ||
            msg.receiver_id === selectedFriend?.id
          ) {
            setMessages((prev) => [...prev, msg]);
          }
          break;
        case "user_online":
          setOnlineUsers((prev) => new Set([...prev, msg.user_id]));
          break;
        case "user_offline":
          setOnlineUsers((prev) => {
            const next = new Set(prev);
            next.delete(msg.user_id);
            return next;
          });
          break;
      }
    };

    ws.onclose = () => {
      console.log("WS closed");
    };

    return () => {
      ws.send(JSON.stringify({ event: "user_offline", user_id: user.id }));
      ws.close();
    };
  }, [user?.id, selectedFriend?.id]);

  const loadFriends = async () => {
    try {
      const res = await get("/friends");
      setFriends(res.data.friends || []);
    } catch (err) {
      console.error("Failed to load friends:", err);
    }
  };

  const loadHistory = async (friend, offset = 0) => {
    if (offset === 0) {
      setSelectedFriend(friend);
      setLoadingHistory(true);
      setMessages([]);
    } else {
      setLoadingHistory(true);
    }

    try {
      const res = await getMessages(friend.id, 50, offset);
      const newMessages = (res.data.messages || []).reverse();
      
      if (offset === 0) {
        setMessages(newMessages);
        setMessageOffset(50);
      } else {
        setMessages((prev) => [...newMessages, ...prev]);
        setMessageOffset(offset + 50);
      }
      
      setHasMore(newMessages.length === 50);
    } catch (err) {
      console.error("Failed to load messages:", err);
    } finally {
      setLoadingHistory(false);
    }
  };

  const loadMoreMessages = () => {
    if (selectedFriend && hasMore && !loadingHistory) {
      loadHistory(selectedFriend, messageOffset);
    }
  };

  const sendMessage = () => {
    if (!text || !wsRef.current || !selectedFriend) return;

    const payload = {
      event: "chat.message",
      receiver_id: selectedFriend.id,
      receiver_type: "user",
      data: {
        text,
      },
    };

    wsRef.current.send(JSON.stringify(payload));

    const localMsg = {
      event: "chat.message",
      sender_id: user.id,
      receiver_id: selectedFriend.id,
      receiver_type: "user",
      data: { text },
      created_at: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, localMsg]);
    setText("");
  };

  const isOnline = selectedFriend && onlineUsers.has(selectedFriend.id);

  return (
    <div style={{ display: "flex", height: "100vh" }}>
      <div style={{ width: "250px", borderRight: "1px solid #ccc", padding: 10 }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 10 }}>
          <h3>Chats</h3>
          <button onClick={logout} style={{ fontSize: 12 }}>Logout</button>
        </div>
        <div>
          {friends.length === 0 ? (
            <p style={{ color: "#666" }}>No friends yet</p>
          ) : (
            friends.map((friend) => (
              <div
                key={friend.id}
                onClick={() => loadHistory(friend)}
                style={{
                  padding: "10px",
                  cursor: "pointer",
                  background: selectedFriend?.id === friend.id ? "#e0e0e0" : "transparent",
                  borderRadius: 4,
                  marginBottom: 4,
                  display: "flex",
                  alignItems: "center",
                  gap: 8,
                }}
              >
                <span
                  style={{
                    width: 8,
                    height: 8,
                    borderRadius: "50%",
                    backgroundColor: onlineUsers.has(friend.id) ? "green" : "#ccc",
                  }}
                />
                {friend.username || friend.email}
              </div>
            ))
          )}
        </div>
      </div>

      <div style={{ flex: 1, display: "flex", flexDirection: "column", padding: 20 }}>
        {selectedFriend ? (
          <>
            <div style={{ borderBottom: "1px solid #ccc", paddingBottom: 10, marginBottom: 10 }}>
              <strong>{selectedFriend.username || selectedFriend.email}</strong>
              <span
                style={{
                  marginLeft: 8,
                  fontSize: 12,
                  color: isOnline ? "green" : "#666",
                }}
              >
                {isOnline ? "● Online" : "○ Offline"}
              </span>
            </div>

            <div style={{ flex: 1, overflowY: "auto" }}>
              {loadingHistory ? (
                <p>Loading...</p>
              ) : hasMore && selectedFriend ? (
                <button 
                  onClick={loadMoreMessages}
                  style={{ margin: "10px auto", display: "block" }}
                >
                  Load More
                </button>
              ) : messages.length === 0 ? (
                <p style={{ color: "#666" }}>No messages yet</p>
              ) : null}

              {messages.map((m, i) => (
                  <div
                    key={i}
                    style={{
                      textAlign: m.sender_id === user.id ? "right" : "left",
                      marginBottom: 8,
                    }}
                  >
                    <span
                      style={{
                        display: "inline-block",
                        padding: "8px 12px",
                        background: m.sender_id === user.id ? "#007bff" : "#e0e0e0",
                        color: m.sender_id === user.id ? "#fff" : "#000",
                        borderRadius: 12,
                        maxWidth: "70%",
                      }}
                    >
                      {m.data?.text || m.body}
                    </span>
                  </div>
                )
              )}
            </div>

            <div style={{ display: "flex", gap: 8, marginTop: 10 }}>
              <input
                style={{ flex: 1, padding: 8 }}
                value={text}
                onChange={(e) => setText(e.target.value)}
                placeholder="Type message"
                onKeyDown={(e) => e.key === "Enter" && sendMessage()}
              />
              <button onClick={sendMessage}>Send</button>
            </div>
          </>
        ) : (
          <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100%" }}>
            <p style={{ color: "#666" }}>Select a conversation to start chatting</p>
          </div>
        )}
      </div>
    </div>
  );
}
