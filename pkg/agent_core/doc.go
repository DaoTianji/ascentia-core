// Package agent_core is a tenant-scoped general agent engine: memory side-query,
// modular prompts, explicit planner/todos, ReAct loop with circuit breaker, and
// optional thinking stream demux. Callers inject MemoryProvider, SkillCatalog,
// ModelClient, etc.; this package does not implement HTTP, SQL, or auth.
package agent_core
