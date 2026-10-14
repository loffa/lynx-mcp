package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePagination(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenLimit  int
		givenOffset int
		wantLimit   int
		wantErr     bool
	}{
		"default limit":                      {wantLimit: 100},
		"minimum limit":                      {givenLimit: 1, wantLimit: 1},
		"maximum limit":                      {givenLimit: 10_000, wantLimit: 10_000},
		"offset is accepted":                 {givenLimit: 25, givenOffset: 50, wantLimit: 25},
		"negative limit":                     {givenLimit: -1, wantErr: true},
		"limit above maximum":                {givenLimit: 10_001, wantErr: true},
		"negative offset":                    {givenLimit: 25, givenOffset: -1, wantErr: true},
		"negative offset with default limit": {givenOffset: -1, wantErr: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gotLimit, err := normalizePagination(tt.givenLimit, tt.givenOffset)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantLimit, gotLimit)
		})
	}
}

func TestPageBounds(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenTotal  int
		givenLimit  int
		givenOffset int
		wantStart   int
		wantEnd     int
	}{
		"empty results":                 {givenLimit: 10},
		"first page":                    {givenTotal: 25, givenLimit: 10, wantEnd: 10},
		"middle page":                   {givenTotal: 25, givenLimit: 10, givenOffset: 10, wantStart: 10, wantEnd: 20},
		"partial last page":             {givenTotal: 25, givenLimit: 10, givenOffset: 20, wantStart: 20, wantEnd: 25},
		"offset equals total":           {givenTotal: 25, givenLimit: 10, givenOffset: 25, wantStart: 25, wantEnd: 25},
		"offset exceeds total":          {givenTotal: 25, givenLimit: 10, givenOffset: 100, wantStart: 25, wantEnd: 25},
		"large limit does not overflow": {givenTotal: math.MaxInt, givenLimit: math.MaxInt, givenOffset: 1, wantStart: 1, wantEnd: math.MaxInt},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gotStart, gotEnd := pageBounds(tt.givenTotal, tt.givenLimit, tt.givenOffset)
			assert.Equal(t, tt.wantStart, gotStart)
			assert.Equal(t, tt.wantEnd, gotEnd)
		})
	}
}
