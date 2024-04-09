package filestream

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"

	ioutils "github.com/jfrog/gofrog/io"
)

const (
	FileType  = "file"
	ErrorType = "error"
)

// The expected type of function that should be provided to the ReadFilesFromStream func, that returns the writer that should handle each file
type FileWriterFunc func(fileName string) (writers []io.WriteCloser, err error)

func ReadFilesFromStream(multipartReader *multipart.Reader, fileWritersFunc FileWriterFunc) error {
	for {
		// Read the next file streamed from client
		fileReader, err := multipartReader.NextPart()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("failed to read file: %w", err)
		}
		if err = readFile(fileReader, fileWritersFunc); err != nil {
			return err
		}

	}
	return nil
}

func readFile(fileReader *multipart.Part, fileWriterFunc FileWriterFunc) (err error) {
	fileName := fileReader.FileName()
	fileWriter, err := fileWriterFunc(fileName)
	if err != nil {
		return err
	}
	var writers []io.Writer
	for _, writer := range fileWriter {
		defer ioutils.Close(writer, &err)
		// Create a multi writer that will write the file to all the provided writers
		// We read multipart once and write to multiple writers, so we can't use the same multipart writer multiple times
		writers = append(writers, writer)
	}
	if _, err = io.Copy(ioutils.AsyncMultiWriter(10, writers...), fileReader); err != nil {
		return fmt.Errorf("failed writing '%s' file: %w", fileName, err)
	}
	return nil
}

type FileInfo struct {
	Name string
	Path string
}

func WriteFilesToStream(multipartWriter *multipart.Writer, filesList []*FileInfo) (err error) {
	var isContentWritten bool
	defer func() {
		// The multipartWriter.Close() function automatically writes the closing boundary to the underlying writer,
		// regardless of whether any content was written to it. Therefore, if no content was written
		// (i.e., no parts were created using the multipartWriter), there is no need to explicitly close the
		// multipartWriter. The closing boundary will be correctly handled by calling multipartWriter.Close()
		// when it goes out of scope or when explicitly called, ensuring the proper termination of the multipart request.
		if isContentWritten {
			err = errors.Join(err, multipartWriter.Close())
		}
	}()
	for _, file := range filesList {
		if err = writeFile(multipartWriter, file); err != nil {
			isContentWritten, err = writeErrPart(multipartWriter, file, err)
			return err
		}
		isContentWritten = true
	}

	return nil
}

func writeFile(multipartWriter *multipart.Writer, file *FileInfo) (err error) {
	fileReader, err := os.Open(file.Path)
	if err != nil {
		return fmt.Errorf("failed opening file %q: %w", file.Name, err)
	}
	defer ioutils.Close(fileReader, &err)
	fileWriter, err := multipartWriter.CreateFormFile(FileType, file.Name)
	if err != nil {
		return fmt.Errorf("failed to create form file for %q: %w", file.Name, err)
	}
	_, err = io.Copy(fileWriter, fileReader)
	return err
}

func writeErrPart(multipartWriter *multipart.Writer, file *FileInfo, writeFileErr error) (bool, error) {
	var isPartWritten bool
	fileWriter, err := multipartWriter.CreateFormField(ErrorType)
	if err != nil {
		return isPartWritten, fmt.Errorf("failed to create form field: %w", err)
	}
	isPartWritten = true
	multipartErr := NewMultipartError(file.Name, writeFileErr.Error())
	multipartErrJSON, err := json.Marshal(multipartErr)
	if err != nil {
		return isPartWritten, fmt.Errorf("failed to marshal multipart error for file %q: %w", file.Name, err)
	}

	_, err = io.Copy(fileWriter, bytes.NewReader(multipartErrJSON))
	return isPartWritten, err
}
