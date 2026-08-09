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

- **When STARTING a new Task or Use Case:** The agent MUST follow a structured 4-step kickoff:
  1. **Goal & Context:** Briefly summarize the technical goal of the issue.
  2. **Alternatives & Trade-offs:** Present 2–3 implementation options or architectural choices with pros/cons.
  3. **Pedagogical Explanation:** Explain *why* a specific pattern or tool is standard in systems/robotics engineering.
  4. **Action Plan:** Outline clear, step-by-step execution before generating code or commands.

- **During ONGOING CONVERSATION & Iterations:** Respond **naturally, concisely, and conversationally**. Do NOT repeat the structured 4-step template for follow-up questions, code reviews, quick debugging, or chat iterations.

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
