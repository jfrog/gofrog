package fanout

import (
        "io"
        "github.com/pkg/errors"
        log "github.com/Sirupsen/logrus"
)

//A reader that emits its read to multiple pipes
type FanoutReader struct {
        reader        io.Reader
        singleReaders []SingleReader
        pipeReaders   []*io.PipeReader
        pipeWriters   []*io.PipeWriter
        results       chan *readerResult
        errs          chan error
}

type SingleReader interface {
        ReadAll(io.Reader) (interface{}, error)
}

type ReaderFunc func(io.Reader) (interface{}, error)

func (f ReaderFunc) ReadAll(r io.Reader) (interface{}, error) {
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

func NewFanoutReader(reader io.Reader, singleReaders ... SingleReader) *FanoutReader {
        procLen := len(singleReaders)
        pipeReaders := make([]*io.PipeReader, procLen)
        pipeWriters := make([]*io.PipeWriter, procLen)
        done := make(chan *readerResult)
        errs := make(chan error)
        //Create pipe r/w for each reader
        for i := 0; i < procLen; i++ {
                pr, pw := io.Pipe()
                pipeReaders[i] = pr
                pipeWriters[i] = pw
        }
        return &FanoutReader{reader, singleReaders, pipeReaders, pipeWriters, done, errs}
}

func (r *FanoutReader) writers() (writers []io.Writer) {
        //Convert to an array of io.Writers so it can be taken by a variadic func
        //See: https://groups.google.com/forum/#!topic/golang-nuts/zU3BqD5mKs8
        writers = make([]io.Writer, len(r.pipeWriters))
        for i, w := range r.pipeWriters {
                writers[i] = w
        }
        return
}

func (r *FanoutReader) GetReader(i int) io.Reader {
        return r.pipeReaders[i]
}

func (r *FanoutReader) ReadAll() ([]interface{}, error) {
        defer close(r.results)
        defer close(r.errs)

        for i, sr := range r.singleReaders {
                go func(sr SingleReader, pos int) {
                        ret, perr := sr.ReadAll(r.pipeReaders[pos])
                        if perr != nil {
                                r.errs <- errors.WithStack(perr)
                                //panic(perr)
                                return
                        }
                        r.results <- &readerResult{ret, pos}
                }(sr, i)
        }
        go func() {
                defer r.Close()
                mw := io.MultiWriter(r.writers()...)
                _, err := io.Copy(mw, r.reader)
                if err != nil {
                        //panic(err)
                        r.errs <- errors.WithStack(err)
                }
        }()
        results := make([]interface{}, len(r.singleReaders))
        for range r.singleReaders {
                select {
                case err := <-r.errs:
                        log.Error("ERROR: ", err)
                        return nil, err
                case result := <-r.results:
                        results[result.pos] = result.data
                        log.Debugf("Doen fanout single reader: %d", result.pos)
                }
        }
        return results, nil
}

func (r *FanoutReader) Close() {
        for _, pw := range r.pipeWriters {
                pw.Close()
        }
}

/*func (r *MultiPipeReader) Read(b []byte) (n int, err error) {
        n, err = r.reader.Read(b)
        if n > 0 {
                for i, pw := range r.pipeWriters {
                        go func() {
                                fmt.Print("*,")
                                if _, werr := pw.Write(b[:n]); werr != nil {
                                        if werr == io.EOF {
                                                //r.done <- true
                                        }
                                        if werr == io.ErrClosedPipe {
                                                log.Printf("Reader %d closed", i)
                                                //r.done <- true
                                        } else {
                                                r.errs <- werr
                                                //panic(werr)
                                                return
                                        }
                                }
                                r.done <- true
                        }()
                }
                for range r.pipeWriters {
                        select {
                        case err := <-r.errs:
                                fmt.Println("ERR!!!")
                                return 0, err
                        case <-r.done:
                        }
                }
        }
        return
}*/