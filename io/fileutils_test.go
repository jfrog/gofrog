package io

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClose(t *testing.T) {
	var err error
	f, err := os.Create(filepath.Join(t.TempDir(), "test"))
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

	nilErr := new(error)
	Close(f, nilErr)
	assert.NotNil(t, nilErr)
}

func getNilErr() error {
	return nil
}
