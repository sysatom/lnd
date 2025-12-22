# LND (Linux Network Diagnoser) - Copilot Instructions

You are an expert Golang developer and System Architect working on **LND**, a TUI-based network diagnostic tool.

## Project Context
- **Goal**: Create a "Swiss Army Knife" for Linux network diagnostics in a single static binary.
- **Stack**: Go 1.24+, Bubble Tea (TUI), Lipgloss (Styling).
- **Build Constraint**: `CGO_ENABLED=0` (Must be statically linked, no libc dependencies).

## Architecture & Boundaries
1.  **`internal/collector/` (Data Layer)**:
    - **Responsibility**: Pure data gathering. Returns Go structs.
    - **Constraint**: NO UI code, NO `lipgloss`, NO `tea.Msg`.
    - **Method**: Prefer parsing `/proc`, `/sys`, or using `netlink` over `exec.Command`.
2.  **`internal/ui/` (View Layer)**:
    - **Responsibility**: Rendering strings and styling.
    - **Constraint**: NO business logic, NO I/O operations.
3.  **`internal/app/` (Control Layer)**:
    - **Responsibility**: Bubble Tea `Model`, `Update` loop, and State management.
    - **Constraint**: Orchestrates data flow. Triggers Collectors via `tea.Cmd`.

## Critical Development Rules
1.  **Zero Panic Policy**:
    - The tool is for production diagnostics. It must **NEVER** panic.
    - Handle all errors (permissions, timeouts, missing files) and return them to be displayed as "N/A" or error messages in the UI.
2.  **Concurrency & Performance**:
    - The TUI `Update()` function must remain non-blocking.
    - Heavy tasks (Ping, DNS, Netlink) **MUST** run in `tea.Cmd` (goroutines).
    - Implement **Loading States** (flags) to prevent spawning thousands of goroutines if a collector is slow.
3.  **Root & Permissions**:
    - Detect Root at startup.
    - If running as non-root, **Degrade Gracefully**. Do not crash. Show "Restricted Mode" warnings for features like `ethtool` or `inet_diag`.
4.  **Compatibility**:
    - Target Linux (AMD64/ARM64).
    - Avoid `syscall` constants that vary by architecture unless handled safely.
5.  **Language**:
    - All code comments, documentation, and commit messages must be in **English**.

## Preferred Libraries
- **TUI**: `github.com/charmbracelet/bubbletea`
- **Style**: `github.com/charmbracelet/lipgloss`
- **Procfs**: `github.com/prometheus/procfs` (Read /proc/net/snmp)
- **Netlink**: `github.com/vishvananda/netlink` (Routes, Interfaces)
- **System**: `github.com/shirou/gopsutil/v3`
