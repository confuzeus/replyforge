package model

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDisplayIDGenerator_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		gen := NewDisplayIDGenerator()
		assert.NotNil(t, gen)
		assert.NotNil(t, gen.s)
	})
}

func TestGenerate_VariousInputs(t *testing.T) {
	gen := NewDisplayIDGenerator()

	tests := []struct {
		name string
		id   int64
	}{
		{"zero", 0},
		{"one", 1},
		{"small positive", 42},
		{"medium", 9999},
		{"large", 999999},
		{"max int32", math.MaxInt32},
		{"near max int64", math.MaxInt64 - 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := gen.Generate(tt.id)
			require.NoError(t, err)
			assert.NotEmpty(t, result)
			assert.GreaterOrEqual(t, len(result), 6)
		})
	}
}

func TestGenerate_MinLength(t *testing.T) {
	gen := NewDisplayIDGenerator()

	for i := int64(0); i < 1000; i++ {
		id, err := gen.Generate(i)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(id), 6, "display ID for %d should be at least 6 chars", i)
	}
}

func TestGenerate_Deterministic(t *testing.T) {
	gen := NewDisplayIDGenerator()

	first, err := gen.Generate(12345)
	require.NoError(t, err)

	second, err := gen.Generate(12345)
	require.NoError(t, err)

	assert.Equal(t, first, second)
}

func TestGenerate_Unique(t *testing.T) {
	gen := NewDisplayIDGenerator()

	seen := make(map[string]bool)
	for i := int64(0); i < 1000; i++ {
		id, err := gen.Generate(i)
		require.NoError(t, err)
		assert.False(t, seen[id], "duplicate display ID: %s for input %d", id, i)
		seen[id] = true
	}
}

func TestGenerate_SequentialNotTrivial(t *testing.T) {
	gen := NewDisplayIDGenerator()

	id1, err := gen.Generate(1)
	require.NoError(t, err)
	id2, err := gen.Generate(2)
	require.NoError(t, err)
	id3, err := gen.Generate(3)
	require.NoError(t, err)

	assert.NotEqual(t, id1, id2)
	assert.NotEqual(t, id2, id3)
	assert.NotEqual(t, id1, id3)
}

func TestGenerate_MaxInt64(t *testing.T) {
	gen := NewDisplayIDGenerator()

	result, err := gen.Generate(math.MaxInt64)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.GreaterOrEqual(t, len(result), 6)
}
