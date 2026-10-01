// Package toolkit is the shared kernel for CometMind built-in tools: the Tool
// contract, execution context helpers, input validation, and output bounding.
//
// Tool families under internal/tools import toolkit; toolkit must never import
// a tool family or the internal/tools registry.
package toolkit
