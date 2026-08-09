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

## 🧠 Interaction & Pair Programming Style

When tackling any Task or Use Case, the agent MUST follow an educational, structured pair-programming approach:

1. **Goal & Context:** Begin by briefly summarizing the technical goal of the issue and how it fits into the broader architecture.
2. **Alternatives & Trade-offs:** Present 2–3 implementation options or architectural choices, explaining pros/cons (e.g., Docker vs. Distrobox, IPC memory models, C++ concurrency patterns).
3. **Pedagogical Explanation:** Explain *why* a specific pattern or tool is standard in systems/robotics engineering, linking concepts back to IoT and software engineering fundamentals.
4. **Action Plan:** Outline clear, step-by-step execution before generating code or terminal commands.

---

## 📋 GitHub Issues & Hierarchy Rules

Issues follow a 3-tier hierarchy: **`Epic` $\rightarrow$ `Use case` $\rightarrow$ `Task`**.

1. **Titles must be CLEAN and natural** — Do NOT add noisy prefixes like `[EPIC-01]` or `[TASK-01.1]`.
   - ✅ `Block 1: ROS2 Middleware Core & C++`
   - ✅ `Basic Pub/Sub Telemetry Pipeline`
   - ✅ `Implement C++20 telemetry publisher node at 100 Hz`
2. **Apply the corresponding type label:**
   - Type labels: `Epic`, `Use case`, `Task`.
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
- **Language Policy:** 
  - All codebase assets, documentation, commit messages, PRs, and GitHub issues MUST be written in **English**.
  - Interactive pair-programming responses and explanations should adapt to the developer's prompt language.
