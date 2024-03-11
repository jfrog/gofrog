package filestream

import (
	"errors"
	"fmt"
	ioutils "github.com/jfrog/gofrog/io"
	"io"
	"mime/multipart"
)

const (
	FileType = "file"
)

// The expected type of function that should be provided to the ReadFilesFromStream func, that returns the writer that should handle each file
type FileWriterFunc func(fileName string) (writer io.WriteCloser, err error)

func ReadFilesFromStream(multipartReader *multipart.Reader, fileWriterFunc FileWriterFunc) error {
	for {
		// Read the next file streamed from client
		fileReader, err := multipartReader.NextPart()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("failed to read file: %w", err)
		}
		err = readFile(fileReader, fileWriterFunc)
		if err != nil {
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
	defer ioutils.Close(fileWriter, &err)
	if _, err = io.Copy(fileWriter, fileReader); err != nil {
		return fmt.Errorf("failed writing '%s' file: %w", fileName, err)
	}
	return err
}

// The expected type of function that should be provided to the WriteFilesToStream func, that returns the reader that should handle each file
type FileReaderFunc func(fileName string) (writer io.ReadCloser, err error)

func WriteFilesToStream(multipartWriter *multipart.Writer, checksumsList []string, fileReaderFunc FileReaderFunc) (err error) {
	for _, fileChecksum := range checksumsList {
		if err = writeFile(multipartWriter, fileChecksum, fileReaderFunc); err != nil {
			return
		}
	}

	// Close finishes the multipart message and writes the trailing
	// boundary end line to the output.
	return multipartWriter.Close()
}

func writeFile(multipartWriter *multipart.Writer, fileChecksum string, fileReaderFunc FileReaderFunc) (err error) {
	fileReader, err := fileReaderFunc(fileChecksum)
	defer ioutils.Close(fileReader, &err)
	fileWriter, err := multipartWriter.CreateFormFile(FileType, fileChecksum)
	if err != nil {
		return fmt.Errorf("failed to CreateFormFile: %w", err)
	}
	_, err = io.Copy(fileWriter, fileReader)
	return err
}
