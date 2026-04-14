---
name: commit
description: Create a well-structured git commit from staged or unstaged changes
user-invocable: true
when_to_use: When the user asks to commit changes, create a commit, or save their work
---

Create a git commit following these steps:

1. Run `git status` to see what's changed
2. Run `git diff` (staged and unstaged) to understand the changes
3. Run `git log --oneline -5` to match the repository's commit style
4. Draft a concise commit message (1-2 sentences) focusing on the "why"
5. Stage relevant files and create the commit
