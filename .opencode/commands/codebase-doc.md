---
description: Generate or refresh technical documentation from the current repository implementation.
agent: documentation
subtask: true
---

Documentation request:

$ARGUMENTS

Inspect the CURRENT repository thoroughly.

Use:

- project-architecture
- codebase-documentation
- api-contract when required
- websocket-realtime when required

Do not trust existing documentation without verifying it against source code.

Generate or update the requested Markdown documentation under docs/.

Document:

1. project overview
2. technology stack
3. repository structure
4. backend architecture
5. frontend architecture
6. database structure
7. authentication
8. REST APIs
9. WebSocket architecture
10. currently implemented features
11. partially implemented/scaffolded features
12. configuration
13. tests
14. deployment/runtime structure
15. current limitations

Clearly distinguish:

- implemented
- partial
- scaffolded
- missing

Do not modify application code.