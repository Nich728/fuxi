# ADR-0017: Use Task instead of make for developer commands

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-11-12 |
| **Related** | SYSTEM_DESIGN §13, §14 |

## Context

SYSTEM_DESIGN assumed `make up`, `make eval` and similar. The owner develops on Windows, where `make` isn't installed by default and Makefiles depend on a Unix shell. CI runs on Linux. The same commands must work in both places.

## Decision

Keep developer commands in `Taskfile.yml` and run them with [Task](https://taskfile.dev) (`task up`, `task test`, `task lint`). Task is a single binary on Windows, macOS and Linux and runs its commands through a built-in POSIX-style shell, so one file works everywhere. Wherever the design docs say `make <x>`, read `task <x>`.

## Alternatives considered

- **make.** Standard on Unix, but needs MSYS2, Git Bash or WSL on Windows, and Makefile syntax (tabs, shell quoting) breaks easily there.
- **PowerShell scripts.** Native on Windows, but a second set of scripts would be needed for CI and macOS.
- **Plain README instructions.** No tooling, but long commands get mistyped and drift from what CI runs.

## Consequences

- Contributors install one extra tool (`winget install Task.Task`, `brew install go-task`).
- CI calls the underlying commands directly, so it doesn't depend on Task.
