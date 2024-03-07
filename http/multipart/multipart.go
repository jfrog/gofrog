package multipart

import (
	"errors"
	"fmt"
	ioutils "github.com/jfrog/gofrog/io"
	"io"
	"mime/multipart"
	"os"
)

type FileWriterFunc func(fileName string) (io.WriteCloser, error)

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
		fileName := fileReader.FileName()
		fileWriter, err := fileWriterFunc(fileName)
		if err != nil {
			return err
		}

		if _, err = io.Copy(fileWriter, fileReader); err != nil {
			return fmt.Errorf("failed writing '%s' file: %w", fileName, err)
		}
		ioutils.Close(fileWriter, &err)
	}
	return nil
}

func WriteFilesToStream(multipartWriter *multipart.Writer, filePaths []string) (err error) {
	defer ioutils.Close(multipartWriter, &err)

	for _, filePath := range filePaths {
		fileReader, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer ioutils.Close(fileReader, &err)
		fileWriter, err := multipartWriter.CreateFormFile("file", filePath)
		if err != nil {
			return fmt.Errorf("failed to CreateFormFile: %w", err)
		}
		_, err = io.Copy(fileWriter, fileReader)
		if err != nil {
			return err
		}
	}
	return nil
}
