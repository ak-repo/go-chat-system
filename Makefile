.PHONY: go dev

go:
	go run cmd/server/main.go

dev:
	npm run dev -- --port 3000
