---
title: "WebSocket Authentication"
description: "How to enable and use token authentication for the /stream WebSocket endpoint."
weight: 5
---

## WebSocket Token Authentication

To protect the `/stream` WebSocket endpoint from unauthorized access, you can require a shared authentication token.

### How to enable

- Add the flag `--ws-token <token>` when starting the server (any command with `--serve`).
- The token value is defined by you (example: `--ws-token SUPER_SECRET_TOKEN`).
- The same token must be sent by the client when connecting to the WebSocket.

### How to authenticate

The token can be sent in two ways:

- HTTP header: `Authorization: Bearer <token>`
- Query string: `?token=<token>`

Example using [wscat](https://github.com/websockets/wscat):

```bash
wscat -c ws://localhost:8080/stream -H "Authorization: Bearer SUPER_SECRET_TOKEN"
# or
wscat -c ws://localhost:8080/stream?token=SUPER_SECRET_TOKEN
```

If the token is incorrect or missing, the connection will be refused with 401 Unauthorized.

### Security notes

- The token acts as a shared "pre-password": any client must know the exact value to access the WebSocket.
- The token is **not generated automatically** — you define and manage the secret.
- In Kubernetes, inject the token via Secret/environment variable.
- If `--ws-token` is not set, the endpoint remains open (default behavior).

> **Note:** The web dashboard does not prompt for a token. This feature protects only the raw WebSocket API. To protect the dashboard, use a reverse proxy with authentication or contribute improvements to the UI.

---

For more details, see the or [Usage Guide](/docs/usage).
