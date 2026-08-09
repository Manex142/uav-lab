# 🤖 AGENTS.md - Antigravity Agent Guidelines for `uav-lab`

This document defines project metadata, standards, and workflow rules for AI assistant agents working on the `uav-lab` repository.

---

## 📌 Project Metadata & URLs

- **GitHub Repository:** `Manex142/uav-lab`
- **Repository URL:** `https://github.com/Manex142/uav-lab`
- **GitHub Project (Kanban):** `UAV Lab`
  - **Project Number:** `5`
  - **Project ID:** `PVT_kwHOB8jL-M4Bf1aZ`
  - **Project URL:** `https://github.com/users/Manex142/projects/5`

---

## 📋 GitHub Issues & Hierarchy Rules

Issues follow a 3-tier hierarchy: **`Epic` $\rightarrow$ `Use Case` $\rightarrow$ `Task`**.

1. **Titles must be CLEAN and natural** — Do NOT add noisy prefixes like `[EPIC-01]` or `[TASK-01.1]`.
   - ✅ `Block 1: ROS2 Middleware Core & C++`
   - ✅ `Basic Pub/Sub Telemetry Pipeline`
   - ✅ `Implement C++20 telemetry publisher node at 100 Hz`
2. **Apply the corresponding type label:**
   - Type labels: `epic`, `use-case`, `task`.
3. **Always link newly created issues to GitHub Project Number `5` (`UAV Lab`).**

---

## 🔀 Git & Commit Conventions

Follow **Conventional Commits** in English:

```text
<type>(<scope>): <short summary in present tense>
```

- **Types:** `feat`, `fix`, `docs`, `refactor`, `style`, `chore`
- **Examples:**
  - `feat(b1): add C++20 telemetry publisher node at 100 Hz`
  - `fix(b1): resolve QoS durability mismatch in subscriber node`
  - `docs(readme): update block 1 milestones`
- **Language Policy:** **ALL** code, docstrings, commit messages, PRs, and GitHub issues MUST be written in **English**.
