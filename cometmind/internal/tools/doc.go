// Package tools is the composition root that assembles CometMind built-in tools.
// Families live in subpackages and must not import this package.
//
//	tools/toolkit       — Tool, Result, Workspace, session/progress context
//	tools/fs            — FileWorkspace: path resolve, mounts, locks
//	tools/fsops         — read, write, edit, glob, grep, list, run
//	tools/web           — web_fetch, web_search
//	tools/media         — generate, present, and capture image/video
//	tools/jobs          — job queue and scheduled job tools
//	tools/mcp           — MCP server management and proxied tools
//	tools/memory        — agent memory tools
//	tools/settings      — settings tools
//	tools/subagent      — spawn, wait, and coding-harness delegation
//	tools/skills        — skill discovery and draft tools
//	tools/inbox         — leave-message tool
//	tools/diffartifact  — edit_file DiffArtifact wire contract
//	tools/sandbox       — path escape checks
package tools
