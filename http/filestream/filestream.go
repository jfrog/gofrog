package filestream

import (
	"errors"
	"fmt"
	ioutils "github.com/jfrog/gofrog/io"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

const (
	contentType = "Content-Type"
	FileType    = "file"
)

// The expected type of function that should be provided to the ReadFilesFromStream func, that returns the writer that should handle each file
type FileHandlerFunc func(fileName string) (writer io.WriteCloser, err error)

func ReadFilesFromStream(multipartReader *multipart.Reader, fileHandlerFunc FileHandlerFunc) error {
	for {
		// Read the next file streamed from client
		fileReader, err := multipartReader.NextPart()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("failed to read file: %w", err)
		}
		fileName := fileReader.FileName()
		fileWriter, err := fileHandlerFunc(fileName)
		if err != nil {
			return err
		}
		if _, err = io.Copy(fileWriter, fileReader); err != nil {
			return fmt.Errorf("failed writing '%s' file: %w", fileName, err)
		}
		err = fileWriter.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func WriteFilesToStream(responseWriter http.ResponseWriter, filePaths []string) (err error) {
	multipartWriter := multipart.NewWriter(responseWriter)
	responseWriter.Header().Set(contentType, multipartWriter.FormDataContentType())

	for _, filePath := range filePaths {
		if err = writeFile(multipartWriter, filePath); err != nil {
			return
		}
	}

	// Close finishes the multipart message and writes the trailing
	// boundary end line to the output.
	return multipartWriter.Close()
}

func writeFile(multipartWriter *multipart.Writer, filePath string) (err error) {
	fileReader, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer ioutils.Close(fileReader, &err)
	fileWriter, err := multipartWriter.CreateFormFile(FileType, filePath)
	if err != nil {
		return fmt.Errorf("failed to CreateFormFile: %w", err)
	}
	_, err = io.Copy(fileWriter, fileReader)
	return err
}
