package httpserver

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is set at build time with -ldflags "-X lynx-mcp/internal/httpserver.version=...".
var version = "dev"

func (s *Server) mcpHandler() http.Handler {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "lynx-mcp",
		Version: version,
	}, nil)

	searchObjectsTool, searchObjectsHandler := s.newSearchObjectsTool()
	mcp.AddTool(server, searchObjectsTool, searchObjectsHandler)

	getObjectTool, getObjectHandler := s.newGetObjectTool()
	mcp.AddTool(server, getObjectTool, getObjectHandler)

	listSchedulesTool, listSchedulesHandler := s.newListSchedulesTool()
	mcp.AddTool(server, listSchedulesTool, listSchedulesHandler)

	listAutomationsTool, listAutomationsHandler := s.newListAutomationsTool()
	mcp.AddTool(server, listAutomationsTool, listAutomationsHandler)

	controlFunctionTool, controlFunctionHandler := s.newControlFunctionTool()
	mcp.AddTool(server, controlFunctionTool, controlFunctionHandler)

	getHistoryTool, getHistoryHandler := s.newGetHistoryTool()
	mcp.AddTool(server, getHistoryTool, getHistoryHandler)

	getCurrentValuesTool, getCurrentValuesHandler := s.newGetCurrentValuesTool()
	mcp.AddTool(server, getCurrentValuesTool, getCurrentValuesHandler)

	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		JSONResponse: true,
	})
}
