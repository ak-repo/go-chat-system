import { useEffect, useRef, useState } from "react";
import { useAuth } from "../context/context";

export default function Chat() {
  const wsRef = useRef(null);
  const { user } = useAuth();
  const [to, setTo] = useState("432e3286-ad69-4221-9194-a1bf10e6e96e");
  const [text, setText] = useState("");
  const [messages, setMessages] = useState([]);

  useEffect(() => {
    const ws = new WebSocket(`ws://localhost:8002/ws`);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log("WebSocket connected");
    };

    ws.onmessage = (event) => {
      console.log("data: ", event);
      const msg = JSON.parse(event.data);
      setMessages((prev) => [...prev, msg]);
    };

    ws.onclose = () => {
      console.log("WebSocket closed");
    };

    return () => ws.close();
  }, [user?.id]);

  const sendMessage = () => {
    if (!text) return;

    const msg = {
      to,
      body: text,
    };

    wsRef.current.send(JSON.stringify(msg));
    setText("");
  };

  return (
    <div style={{ padding: 20 }}>
      <h3>Minimal WebSocket Chat</h3>

      <div>
        <label>Your ID: </label>
        <input value={user?.id} />
      </div>

      <div>
        <label>Send To: </label>
        <input value={to} onChange={(e) => setTo(e.target.value)} />
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
            {console.log(m)}
            <strong>{m.from}:</strong> {m.text}
          </li>
        ))}
      </ul>
    </div>
  );
}
