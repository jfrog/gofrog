package io

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAsyncMultiWriter(t *testing.T) {
	for _, limit := range []int{1, 2} {
		var buf1, buf2 bytes.Buffer
		multiWriter := AsyncMultiWriter(limit, &buf1, &buf2)

		data := []byte("test data")
		n, err := multiWriter.Write(data)
		assert.NoError(t, err)
		assert.Equal(t, len(data), n)

		// Check if data is correctly written to both writers
		assert.Equal(t, string(data), buf1.String())
		assert.Equal(t, string(data), buf2.String())
	}
}

func TestAsyncMultiWriter_Error(t *testing.T) {
	expectedErr := errors.New("write error")

	// Mock writer that always returns an error
	mockWriter := &mockWriter{writeErr: expectedErr}
	multiWriter := AsyncMultiWriter(2, mockWriter)

	_, err := multiWriter.Write([]byte("test data"))
	assert.Equal(t, expectedErr, err)
}

// Mock writer to simulate Write errors
type mockWriter struct {
	writeErr error
}

func (m *mockWriter) Write(p []byte) (int, error) {
	return 0, m.writeErr
}
