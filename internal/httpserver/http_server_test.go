package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lynx-mcp/internal/config"
)

func TestServer_Routes(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenMethod string
		givenPath   string
		givenBody   string
		wantStatus  int
	}{
		"MCP endpoint initializes a session": {
			givenMethod: http.MethodPost, givenPath: "/mcp",
			givenBody:  `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`,
			wantStatus: http.StatusOK,
		},
		"root is not routed":                         {givenMethod: http.MethodGet, givenPath: "/", wantStatus: http.StatusNotFound},
		"MCP subpaths are not routed":                {givenMethod: http.MethodGet, givenPath: "/mcp/unknown", wantStatus: http.StatusNotFound},
		"unsupported method reaches the MCP handler": {givenMethod: http.MethodPut, givenPath: "/mcp", wantStatus: http.StatusMethodNotAllowed},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := &Server{Service: &mcpServiceMock{}}
			handler := s.routes()
			request := httptest.NewRequestWithContext(t.Context(), tt.givenMethod, tt.givenPath, strings.NewReader(tt.givenBody))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			assert.Equal(t, tt.wantStatus, recorder.Code)

			if tt.wantStatus == http.StatusOK {
				assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
				sessionID := recorder.Header().Get("Mcp-Session-Id")
				require.NotEmpty(t, sessionID)
				deleteRequest := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/mcp", nil)
				deleteRequest.Header.Set("Mcp-Session-Id", sessionID)
				deleteRecorder := httptest.NewRecorder()
				handler.ServeHTTP(deleteRecorder, deleteRequest)
				assert.Equal(t, http.StatusNoContent, deleteRecorder.Code)
			}
		})
	}
}

func TestServer_Open(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenListen   string
		givenCanceled bool
		wantErr       bool
	}{
		"listen failure is returned":                 {givenListen: "127.0.0.1:invalid", wantErr: true},
		"context cancellation shuts the server down": {givenListen: "127.0.0.1:0", givenCanceled: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.givenCanceled {
				cancel()
			}
			s := &Server{Service: &mcpServiceMock{}}
			result := make(chan error, 1)
			go func() {
				result <- s.Open(ctx, config.HTTPConfig{Listen: tt.givenListen, ShutdownTimeout: time.Second})
			}()

			select {
			case err := <-result:
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("HTTP server did not return")
			}
		})
	}
}
