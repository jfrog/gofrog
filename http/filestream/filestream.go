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
	"github.com/schollz/progressbar/v3"
)

const (
	FileType  = "file"
	ErrorType = "error"
)

type MultipartError struct {
	FileName   string `json:"file_name"`
	ErrMessage string `json:"error_message"`
}

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
	return WriteFilesToStreamWithProgressBar(multipartWriter, filesList, nil)
}

func WriteFilesToStreamWithProgressBar(multipartWriter *multipart.Writer, filesList []*FileInfo, bar *progressbar.ProgressBar) (err error) {
	// Close finishes the multipart message and writes the trailing
	// boundary end line to the output, thereby marking the EOF.
	defer ioutils.Close(multipartWriter, &err)
	for _, file := range filesList {
		if err = writeFile(multipartWriter, file); err != nil {
			// Returning the error from writeFile with a possible error from the writeErr function
			return errors.Join(err, writeErr(multipartWriter, file, err))
		}
		if bar != nil {
			bar.Add(1)
		}
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

func writeErr(multipartWriter *multipart.Writer, file *FileInfo, writeFileErr error) error {
	fileWriter, err := multipartWriter.CreateFormField(ErrorType)
	if err != nil {
		return fmt.Errorf("failed to create form field: %w", err)
	}

	multipartErr := MultipartError{FileName: file.Name, ErrMessage: writeFileErr.Error()}
	multipartErrJSON, err := json.Marshal(multipartErr)
	if err != nil {
		return fmt.Errorf("failed to marshal multipart error for file %q: %w", file.Name, err)
	}

	_, err = io.Copy(fileWriter, bytes.NewReader(multipartErrJSON))
	return err
}
