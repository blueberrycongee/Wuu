// Package sdk is the application-embedding SDK for Wuu's UI-neutral agent
// host. It is separate from packages/plugin-go, the helper-side SDK used by
// plugin runtime processes; plugin authors do not need this package.
//
// A Runtime owns the provider clients, durable session mechanisms, tools,
// plugin generations, and recovery services for one workspace. Applications
// can either serve the versioned app-server protocol to another process or use
// Client and Session to drive that same protocol in-process. Neither path
// imports or replaces agent-loop, persistence, permission, or plugin internals.
package sdk
