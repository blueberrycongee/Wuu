// Package sdk embeds Wuu's UI-neutral agent host in Go applications.
//
// A Runtime owns the provider clients, durable session mechanisms, tools,
// plugin generations, and recovery services for one workspace. Shells attach
// through the versioned app-server protocol instead of importing agent-loop or
// persistence internals.
package sdk
