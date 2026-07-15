package websocket

import (
	"encoding/json"
	"testing"
)

func TestExtractMessageTextSupportsTextAndContent(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "text field",
			data: `{"text":"hello"}`,
			want: "hello",
		},
		{
			name: "content field",
			data: `{"content":"hello"}`,
			want: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractMessageText(json.RawMessage(tt.data))
			if err != nil {
				t.Fatalf("extractMessageText returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
