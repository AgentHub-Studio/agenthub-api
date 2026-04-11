// export_test.go exposes internal functions to the _test package for white-box testing.
package agentic

// ExportedMemoryStoreTool exposes the internal memoryStoreTool constructor for tests.
func ExportedMemoryStoreTool() LLMTool {
	return memoryStoreTool()
}
