package io

import (
	"errors"
	"io"

	"golang.org/x/sync/errgroup"
)

var ErrShortWrite = errors.New("the number of bytes written is less than the length of the input")

type asyncMultiWriter struct {
	writers []io.Writer
	limit   int
}

// AsyncMultiWriter creates a writer that duplicates its writes to all the
// provided writers asynchronous
func AsyncMultiWriter(limit int, writers ...io.Writer) io.Writer {
	w := make([]io.Writer, len(writers))
	copy(w, writers)
	return &asyncMultiWriter{writers: w, limit: limit}
}

// Writes data asynchronously to each writer and waits for all of them to complete.
// In case of an error, the writing will not complete.
func (amw *asyncMultiWriter) Write(p []byte) (int, error) {
	eg := errgroup.Group{}
	eg.SetLimit(amw.limit)
	for _, w := range amw.writers {
		currentWriter := w
		eg.Go(func() error {
			n, err := currentWriter.Write(p)
			if err != nil {
				return err
			}
			if n != len(p) {
				return ErrShortWrite
			}
			return nil
		})
	}

	return len(p), eg.Wait()
}
