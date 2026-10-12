# AGENTS.md - Agent Guidelines for `uav-lab`

Project context, commands and workflow rules for AI coding agents working on this repository.
See `README.md` for the full architecture, design decisions and benchmarks.

---

## 📌 Project Overview

End-to-end telemetry pipeline for a simulated UAV fleet: a Raspberry Pi 5 edge node streams
Protobuf over UDP inside a WireGuard tunnel to a Go gateway on a Hetzner VPS, which persists
to TimescaleDB and serves a live React dashboard. Live at https://uav.manex.dev.

| Area | Location | Stack |
| :--- | :--- | :--- |
| CLI entrypoint (`uav gateway \| sim \| monitor \| db`) | `cmd/uav`, `internal/cli` | Go, cobra |
| Ingest, ledger, persistence, HTTP/WS | `internal/{ingest,ledger,database,server}` | Go, pgx, goose |
| Simulator, terminal UI | `internal/{sim,tui}` | Go, Bubble Tea |
| Telemetry contract | `proto/telemetry/v1` → generated into `gen/go` | Protobuf |
| Dashboard | `web/` | React, Vite, Tailwind, MapLibre (TS) |
| ROS 2 nodes (block 1) | `src/b1_core_nodes` | C++20, ROS 2 Jazzy (Docker) |
| Deploy | `docker-compose.{prod,edge}.yml`, `.github/workflows/deploy.yml` | Docker, GHCR, Watchtower |

---

## 🛠️ Commands

```bash
make db-up        # TimescaleDB in Docker (needed by integration tests)
make test         # go test -race ./...
make dev          # gateway: UDP :9876, HTTP/WS :8080
make swarm drones=20 hz=50
make web-dev      # Vite dev server; `npm --prefix web run lint` runs oxlint
make tidy         # go fmt + go mod tidy
make migration name=<name>      # new goose migration in internal/database/migrations
bash scripts/generate_proto.sh  # regenerate gen/go after editing .proto (needs protoc)
```

- Never edit `gen/` by hand; change the `.proto` and regenerate.
- Schema changes go through a new goose migration, never by editing an applied one.
- Every push to `main` deploys to production (GHCR + SSH to the VPS, Watchtower on the Pi).
  Make sure `make test` and `npm --prefix web run build` pass before pushing.
- `build/`, `install/`, `log/` are colcon artifacts from the ROS 2 container; ignore them.

---

## 🧠 Interaction & Pair Programming Style

- **When STARTING a new Task or Use Case:** follow a structured 4-step kickoff:
  1. **Goal & Context:** Briefly summarize the technical goal of the issue.
  2. **Alternatives & Trade-offs:** Present 2–3 implementation options or architectural choices with pros/cons.
  3. **Pedagogical Explanation:** Explain *why* a specific pattern or tool is standard in systems/robotics/networking engineering.
  4. **Action Plan:** Outline clear, step-by-step execution before generating code or commands.

- **During ONGOING CONVERSATION & Iterations:** Respond **naturally, concisely, and conversationally**. Do NOT repeat the structured 4-step template for follow-up questions, code reviews, quick debugging, or chat iterations.

---

## 📌 GitHub Project

- **Repository:** `Manex142/uav-lab`
- **GitHub Project (Kanban):** `UAV Lab`, number `5`, ID `PVT_kwHOB8jL-M4Bf1aZ`
  (https://github.com/users/Manex142/projects/5)

## 📋 GitHub Issues & Hierarchy Rules

Issues follow a 3-tier hierarchy: **`Epic` → `Use case` → `Task`**.

1. **Titles must be CLEAN and natural** — Do NOT add noisy prefixes like `[EPIC-01]` or `[TASK-01.1]`.
   - ✅ `Block 1: ROS2 Middleware Core & C++`
   - ✅ `Basic Pub/Sub Telemetry Pipeline`
   - ✅ `Implement C++20 telemetry publisher node at 100 Hz`
2. **Apply the corresponding type label:** `Epic`, `Use case`, `Task`.
3. **Always link newly created issues to GitHub Project `5` (`UAV Lab`).**
4. **Always link child issues to their parent issue:**
   - **Native GitHub Sub-Issues:** Every `Use case` must be a sub-issue of its parent `Epic`, and every `Task` a sub-issue of its parent `Use case`, via GitHub GraphQL (`addSubIssue` mutation through `gh api graphql`) so progress bars and hierarchy show up in the web UI.
   - **Markdown References:**
     - Child issues must declare their parent at the top of the body (e.g., `Parent Epic: #<number>` or `Parent Use case: #<number>`).
     - Parent issues must track their children in a markdown checklist (e.g., `- [ ] #<number> <Title>`).

---

## 🔀 Git & Commit Conventions

Follow **Conventional Commits** in English:

```text
<type>(<scope>): <short summary in present tense> (#<issue>)
```

- **Types:** `feat`, `fix`, `docs`, `refactor`, `style`, `test`, `perf`, `ci`, `chore`
- **Scopes:** the area touched, e.g. `ingest`, `ledger`, `db`, `server`, `sim`, `tui`, `web`, `proto`, `deploy`, `edge`, `ci`, `bench`, `b1` (ROS 2 nodes), `readme`, `agents`. Combine with commas when needed (`fix(web,server): ...`).
- Append the issue number when the commit closes or advances an issue.
- **Examples:**
  - `feat(server): implement Go WebSocket telemetry streaming hub and HTTP server (#55)`
  - `fix(web,server): resolve LIVE status flicker caused by microsecond clock skew`
  - `docs(readme): add architecture diagram, live demo and benchmark summary`
- **Language:** all code, docs, commits, PRs and GitHub issues MUST be in **English**. Conversation follows the developer's language.
