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

func WriteFilesToStream(multipartWriter *multipart.Writer, filesList []*FileInfo) error {
	for _, file := range filesList {
		if err := writeFile(multipartWriter, file); err != nil {
			return writeErrPart(multipartWriter, file, err)
		}
	}

	// Close finishes the multipart message and writes the trailing
	// boundary end line to the output.
	// We don't use defer for this because the multipart.Writer's Close() method writes regardless of whether there was an error or if writing hadn't started at all
	return multipartWriter.Close()
}

func writeFile(multipartWriter *multipart.Writer, file *FileInfo) (err error) {
	fileReader, err := os.Open(file.Path)
	if err != nil {
		return fmt.Errorf("failed opening %q: %w", file.Name, err)
	}
	defer ioutils.Close(fileReader, &err)
	fileWriter, err := multipartWriter.CreateFormFile(FileType, file.Name)
	if err != nil {
		return fmt.Errorf("failed to CreateFormFile: %w", err)
	}
	_, err = io.Copy(fileWriter, fileReader)
	return err
}

func writeErrPart(multipartWriter *multipart.Writer, file *FileInfo, writeFileErr error) error {
	fileWriter, err := multipartWriter.CreateFormField(ErrorType)
	if err != nil {
		return fmt.Errorf("failed to CreateFormField: %w", err)
	}

	multipartErr := NewMultipartError(file.Name, writeFileErr.Error())
	multipartErrJSON, err := json.Marshal(multipartErr)
	if err != nil {
		return fmt.Errorf("failed to marshal multipart error: %w", err)
	}

	_, err = io.Copy(fileWriter, bytes.NewReader(multipartErrJSON))
	return err
}
