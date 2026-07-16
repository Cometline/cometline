// Package tools is the registry assembler for CometMind built-in tools.
//
// Domain layout (deepening #6):
//
//	tools/fs            — FileWorkspace: path resolve, mounts, locks
//	tools/diffartifact  — edit_file DiffArtifact wire contract
//	tools/sandbox       — path escape checks
//	tools/*.go          — tool adapters + registry + ToolSurface policy
//	  surface.go        — CodingCapability / ToolSurface
//	  edit/read/write/run — filesystem tool adapters
//	  spawngeneral      — in-process subagents
//	  delegatecoding    — optional ACP harness adapter
//	  job_*, memory_*, mcp_* — other domains (still co-located; thin seams)
package tools
