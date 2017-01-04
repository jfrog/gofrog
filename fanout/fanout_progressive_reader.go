package fanout

import (
	"io"
)

//A reader that emits its read to multiple consumers using an io.Reader Read(p []byte) (int, error) func
type FanoutProgressiveReader struct {
	reader      io.Reader
	consumers   []ProgressiveConsumer
	pipeReaders []*io.PipeReader
	pipeWriters []*io.PipeWriter
	multiWriter io.Writer
}

type ProgressiveConsumer interface {
	Read([]byte) error
}

type ProgressiveConsumerFunc func([]byte) error

func (f ProgressiveConsumerFunc) Read(p []byte) error {
	return f(p)
}

type progressiveReaderResult struct {
	data interface{}
	pos  int
}

func NewProgressiveFanoutReader(reader io.Reader, consumers ...ProgressiveConsumer) *FanoutProgressiveReader {
	procLen := len(consumers)
	pipeReaders := make([]*io.PipeReader, procLen)
	pipeWriters := make([]*io.PipeWriter, procLen)
	//Create pipe r/w for each reader
	for i := 0; i < procLen; i++ {
		pr, pw := io.Pipe()
		pipeReaders[i] = pr
		pipeWriters[i] = pw
	}
	multiWriter := io.MultiWriter(toWriters(pipeWriters)...)
	return &FanoutProgressiveReader{reader: reader, consumers: consumers, pipeReaders: pipeReaders,
		pipeWriters:                    pipeWriters, multiWriter: multiWriter}
}

func (r *FanoutProgressiveReader) Read(p []byte) (int, error) {
	errs := make(chan error)
	done := make(chan bool)

	var n int
	var e error

	//TODO: EOF is EOF - no point in reading the single readers
	go func() {
		//Read from reader and fan out to the writers
		n, err := r.reader.Read(p)
		if err != nil {
			//Do not wrap the read err or EOF will not be handled
			errs <- err
		} else {
			_, err = r.multiWriter.Write(p[:n])
			if err != nil {
				errs <- err
			}
		}
	}()

	for i, sr := range r.consumers {
		go func(sr ProgressiveConsumer, pos int) {
			buf := make([]byte, len(p))
			l, perr := r.pipeReaders[pos].Read(buf)
			if perr != nil {
				errs <- perr
				return
			}
			rerr := sr.Read(buf[:l])
			if rerr != nil {
				errs <- rerr
				return
			}
			done <- true
		}(sr, i)
	}

	for range r.consumers {
		select {
		case e = <-errs:
			return n, e
		case <-done:
		}
	}
	return n, nil
}

func (r *FanoutProgressiveReader) Close() (err error) {
	for _, pw := range r.pipeWriters {
		e := pw.Close()
		if err != nil {
			err = e
		}
	}
	return
}
