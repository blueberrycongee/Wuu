# Workspaces and projects

The workspace determines which local files wuu can operate on directly, which
directory commands run in, and where sessions appear in the sidebar. Before starting
a file task, confirming the current workspace matters more than tuning the prompt.

## Conversations and real projects

The desktop has two kinds of entries:

- **Conversations:** a shared area not bound to a user project directory. All
  project-less sessions use a Wuu-managed persistent scratch workspace, where you can
  create temporary files and open a terminal.
- **Projects:** bound to a local directory you choose, suited to reading and editing
  files, running commands, and reviewing Git changes.

If the answer shows the wrong directory, technology stack, or stale sessions, stop the
task and check the current project first.

## Add a workspace

In the **Workspaces** area of the sidebar, choose **Add workspace**:

- **Use existing folder:** register an existing project directory;
- **New blank project:** create and register a new empty directory.

Adding a workspace does not upload the project to wuu. Files stay in their original
location; when interacting with a model, only the prompts, file content, and tool
results the task needs are sent to the current model service.

## Switch between and organize projects

- Select a project name to enter its workspace and see the sessions that belong to it.
- You can collapse and reorder the project area, and pin frequently used sessions.
- Removing a project from the sidebar does not delete its files on disk. By default it
  only removes wuu's registration and keeps local state; you can then choose to clean
  up that project's sessions, goals, and artifacts, while memory is archived and kept.
- If a directory was moved or renamed, the project shows as unavailable. Use
  **Relocate** to choose the new directory and keep the original project identity and
  session ownership.

## Workspace boundary

The default `standard` permission mode restricts file tools to the current runtime
root, registered workspaces, system temporary directories, and explicitly attached
scopes — but it is not an operating-system sandbox. Commands that are allowed to run
still execute as the current logged-in user. Before working with untrusted
repositories or sensitive data, read [permission modes](../reference/permissions.md)
and the [security model](../reference/security-model.md).

## Next steps

- [Conversations and branches](conversations.md)
- [Files, changes, terminal, and browser](workspace-tools.md)
