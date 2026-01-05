# go-chat-system

“A real chat system is not about sending messages; it is about guaranteeing delivery under failure.”

```text
chat-app/
├── cmd/                         # 🟢 ENTRY POINTS (Main applications)
│   └── server/
│       └── main.go              # App entrypoint. Initializes Config, DB, Redis, HTTP & WS servers.
│
├── internal/                    # 🟢 PRIVATE APPLICATION CODE (The Core)
│   ├── config/                  # Configuration loading (Env vars, Flags)
│   │
│   ├── domain/                  # 🧠 PURE LOGIC (Interfaces & Models ONLY)
│   │   ├── user.go              # User entity & repository interface
│   │   ├── message.go           # Message entity & repository interface
│   │   ├── conversation.go      # Group / One-on-One conversation rules
│   │   ├── call.go              # WebRTC signaling domain models
│   │   └── errors.go            # Standardized domain errors
│   │
│   ├── service/                 # 🧠 USE CASES (Application logic)
│   │   ├── auth_service.go      # Login, Registration, JWT issuing
│   │   ├── chat_service.go      # Message validation & routing logic
│   │   ├── presence_service.go  # Online / Offline status logic
│   │   └── signaling_service.go # WebRTC offer / answer orchestration
│   │
│   ├── repository/              # 💾 DATA STORAGE IMPLEMENTATION
│   │   ├── postgres/            # PostgreSQL implementations
│   │   │   ├── user_repo.go
│   │   │   └── message_repo.go
│   │   └── redis/               # Redis implementations
│   │       ├── presence_repo.go # Online user tracking
│   │       └── cache_repo.go    # Generic caching
│   │
│   └── transport/               # 🔌 INPUT ADAPTERS (How data enters the system)
│       ├── http/                # REST API
│       │   ├── handler.go       # Router setup (Gin / Chi / net-http)
│       │   ├── auth.go          # Auth endpoints
│       │   └── middleware.go    # CORS, JWT validation
│       │
│       └── websocket/           # Real-Time Engine
│           ├── hub.go            # Connection registry & broadcasting
│           ├── client.go        # Per-user read/write pumps
│           └── handler.go       # WS events → service calls
│
├── pkg/                         # 🟢 PUBLIC / REUSABLE LIBRARIES
│   ├── logger/                  # Structured logging wrapper
│   └── utils/                   # Small shared helpers (Time, IDs)
│
├── migrations/                  # 🟢 DATABASE MIGRATIONS
│   ├── 000001_create_users.up.sql
│   └── 000002_create_messages.up.sql
│
├── deploy/                      # 🟢 DEPLOYMENT CONFIGS
│   ├── docker-compose.yml       # Local dev (App + Postgres + Redis)
│   └── k8s/                     # Future Kubernetes manifests
│
├── Dockerfile                   # Production build instructions
├── Makefile                     # Task runner (build, run, migrate)
├── go.mod                       # Go dependencies
└── README.md                    # Project documentation
```

# Flow : Outside World → Transport → Service → Domain → Repository → DB

https://github.com/ak-repo/go-chat-system.git
