package websocket


type Room struct {
	ID      string
	Members map[string]bool
}

func (h *Hub) CreateRoom(roomID string, members []string) {
	room := &Room{
		ID:      roomID,
		Members: make(map[string]bool),
	}

	for _, id := range members {
		room.Members[id] = true
	}

	h.rooms[roomID] = room
}

func (h *Hub) sendToGroup(msg *WSMessage) {
	room, ok := h.rooms[msg.ReceiverID]
	if !ok {
		return
	}

	for userID := range room.Members {
		conns, ok := h.clients[userID]
		if !ok {
			continue
		}

		for c := range conns {
			select {
			case c.send <- msg:
			default:
				close(c.send)
				delete(conns, c)
			}
		}
	}
}
