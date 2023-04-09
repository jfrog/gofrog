package fanout

import (
	"fmt"
	"github.com/pkg/errors"
	"io"
	"sync"
)

// A reader that emits its read to multiple consumers using a ReadAll(p []byte) ([]interface{}, error) func
type ReadAllReader struct {
	reader      io.Reader
	consumers   []ReadAllConsumer
	pipeReaders []*io.PipeReader
	pipeWriters []*io.PipeWriter
	results     chan *readerResult
	errs        chan error
}

type ReadAllConsumer interface {
	ReadAll(io.Reader) (interface{}, error)
}

type ReadAllConsumerFunc func(io.Reader) (interface{}, error)

func (f ReadAllConsumerFunc) ReadAll(r io.Reader) (interface{}, error) {
	return f(r)
}

type readerResult struct {
	data interface{}
	pos  int
}

/*
[inr]--r--
          |--w--[pw]--|--[pr]--r
          |--w--[pw]--|--[pr]--r
          |--w--[pw]--|--[pr]--r
*/

func NewReadAllReader(reader io.Reader, consumers ...ReadAllConsumer) *ReadAllReader {
	procLen := len(consumers)
	pipeReaders := make([]*io.PipeReader, procLen)
	pipeWriters := make([]*io.PipeWriter, procLen)
	done := make(chan *readerResult, procLen)
	errs := make(chan error, procLen)
	//Create pipe r/w for each reader
	for i := 0; i < procLen; i++ {
		pr, pw := io.Pipe()
		pipeReaders[i] = pr
		pipeWriters[i] = pw
	}
	return &ReadAllReader{reader, consumers, pipeReaders, pipeWriters, done, errs}
}

func toWriters(pipeWriters []*io.PipeWriter) (writers []io.Writer) {
	// Convert to an array of io.Writers so it can be taken by a variadic func
	// See: https://groups.google.com/forum/#!topic/golang-nuts/zU3BqD5mKs8
	writers = make([]io.Writer, len(pipeWriters))
	for i, w := range pipeWriters {
		writers[i] = w
	}
	return
}

func (r *ReadAllReader) GetReader(i int) io.Reader {
	return r.pipeReaders[i]
}

func (r *ReadAllReader) ReadAll() ([]interface{}, error) {
	defer close(r.results)
	defer close(r.errs)

	for i, sr := range r.consumers {
		go func(sr ReadAllConsumer, pos int) {
			reader := r.pipeReaders[pos]
			// The reader might stop but the writer hasn't done
			// Closing the pipe will cause an error to the writer which will cause all readers to stop as well
			defer reader.Close()
			ret, perr := sr.ReadAll(reader)
			if perr != nil {
				r.errs <- errors.WithStack(perr)
				return
			}
			r.results <- &readerResult{ret, pos}
		}(sr, i)
	}
	var multiWriterError error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer r.close()
		mw := io.MultiWriter(toWriters(r.pipeWriters)...)
		_, err := io.Copy(mw, r.reader)
		if err != nil {
			// probably caused due to closed pipe reader
			multiWriterError = fmt.Errorf("fanout multiwriter error: %v ", err)
		}
	}()
	wg.Wait()
	return getAllReadersResult(r, multiWriterError)
}

func (r *ReadAllReader) close() {
	for _, pw := range r.pipeWriters {
		_ = pw.Close()
	}
}

func getAllReadersResult(r *ReadAllReader, err error) ([]interface{}, error) {
	results := make([]interface{}, len(r.consumers))
	lastError := err
	for range r.consumers {
		select {
		case e := <-r.errs:
			lastError = e
		case result := <-r.results:
			results[result.pos] = result.data
		}
	}
	if lastError != nil {
		return nil, lastError
	}
	return results, nil
}
