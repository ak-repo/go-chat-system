package model

type ApiResponse struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
	Error  any    `json:"error,omitempty"`
}
