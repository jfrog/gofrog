package io

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"strings"
	"testing"
)

func TestClose(t *testing.T) {
	var err error
	t.TempDir()
	f, err := os.Create(path.Join(t.TempDir(), "test"))
	assert.NoError(t, err)

	Close(f, &err)
	assert.NoError(t, err)

	// Try closing the same file again and expect error
	Close(f, &err)
	assert.Error(t, err)

	// Check that both errors are aggregated
	err = errors.New("original error")
	Close(f, &err)
	assert.Len(t, strings.Split(err.Error(), "\n"), 2)
}
