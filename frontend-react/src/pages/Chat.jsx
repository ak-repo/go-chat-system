import { useEffect, useRef, useState } from "react";
import { useAuth } from "../context/context";

export default function Chat() {
  const wsRef = useRef(null);
  const { user } = useAuth();

  const [receiverId, setReceiverId] = useState(
    "4ce4f257-edcd-48b6-b72f-a605771d8647"
  );
  const [text, setText] = useState("");
  const [messages, setMessages] = useState([]);

  useEffect(() => {
    if (!user?.id) return;

    const ws = new WebSocket("ws://localhost:8002/api/v1/ws");
    wsRef.current = ws;

    ws.onopen = () => {
      console.log("WS connected");
    };

    ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);

      // minimal safety check
      if (msg.event === "chat.message") {
        setMessages((prev) => [...prev, msg]);
      }
    };

    ws.onclose = () => {
      console.log("WS closed");
    };

    return () => ws.close();
  }, [user?.id]);

  const sendMessage = () => {
    if (!text || !wsRef.current) return;

    const payload = {
      event: "chat.message",
      receiver_id: receiverId,
      receiver_type: "user", // or "group"
      data: {
        text,
      },
    };

    wsRef.current.send(JSON.stringify(payload));
    setText("");
  };

  return (
    <div style={{ padding: 20 }}>
      <h3>Minimal WebSocket Chat</h3>

      <div>
        <label>Your ID:</label>
        <input value={user?.id || ""} readOnly />
      </div>

      <div>
        <label>Send To:</label>
        <input
          value={receiverId}
          onChange={(e) => setReceiverId(e.target.value)}
        />
      </div>

      <hr />

      <div>
        <input
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="Type message"
        />
        <button onClick={sendMessage}>Send</button>
      </div>

      <ul>
        {messages.map((m, i) => (
          <li key={i}>
            <strong>{m.sender_id}:</strong> {m.data?.text}
          </li>
        ))}
      </ul>
    </div>
  );
}
