package httpserver

import (
	"fmt"
	"testing"

	"github.com/IoTOpen/go-lynx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lynx-mcp/internal/domain"
)

func TestServer_NewSearchObjectsTool(t *testing.T) {
	t.Parallel()

	output := &domain.SearchObjectsResult{
		SearchResult: lynx.SearchResult{Total: 1, Limit: 10, Offset: 2},
		Results:      []domain.SearchObjectEntry{{SearchEntry: lynx.SearchEntry{Type: "function", ID: 7}, ModelType: "temperature"}},
	}
	tests := map[string]struct {
		givenInput  searchObjectsInput
		givenOutput *domain.SearchObjectsResult
		givenErr    error
		wantOptions lynx.SearchOptions
		wantOutput  domain.SearchObjectsResult
		wantErr     bool
	}{
		"global query and options are forwarded": {
			givenInput:  searchObjectsInput{Query: "temperature", Types: []string{"function"}, Limit: 10, Offset: 2, Metadata: map[string]string{"floor": "2"}},
			givenOutput: output, wantOutput: *output,
			wantOptions: lynx.SearchOptions{Query: "temperature", Types: []string{"function"}, Limit: 10, Offset: 2, Metadata: lynx.Meta{"floor": "2"}},
		},
		"installation search leaves optional query unset": {
			givenInput: searchObjectsInput{InstallationID: 42}, givenOutput: output, wantOutput: *output,
		},
		"service error": {
			givenInput: searchObjectsInput{Query: "*"}, givenErr: fmt.Errorf("mock error"), wantErr: true,
			wantOptions: lynx.SearchOptions{Query: "*"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			service := &mcpServiceMock{SearchObjectsFunc: func(id int64, options lynx.SearchOptions) (*domain.SearchObjectsResult, error) {
				assert.Equal(t, tt.givenInput.InstallationID, id)
				assert.Equal(t, tt.wantOptions, options)
				return tt.givenOutput, tt.givenErr
			}}
			s := &Server{Service: service}
			tool, handler := s.newSearchObjectsTool()

			gotResult, gotOutput, err := handler(t.Context(), nil, tt.givenInput)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Nil(t, gotResult)
			assert.Equal(t, tt.wantOutput, gotOutput)
			assert.Len(t, service.SearchObjectsCalls(), 1)
			require.NotNil(t, tool)
			assert.Equal(t, "iot.search_objects", tool.Name)
			require.NotNil(t, tool.Annotations)
			assert.True(t, tool.Annotations.ReadOnlyHint)
			assert.True(t, tool.Annotations.IdempotentHint)
		})
	}
}
