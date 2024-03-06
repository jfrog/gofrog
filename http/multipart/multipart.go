package multipart

import (
	"fmt"
	fileutils "github.com/jfrog/gofrog/io"
	"io"
	"mime/multipart"
	"os"
)

type FileWriterFunc func(fileName string) (io.WriteCloser, error)

func ReadFromStream(multipartReader *multipart.Reader, fileWriterFunc FileWriterFunc) error {
	for {
		// Read the next file streamed from client
		fileReader, err := multipartReader.NextPart()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read file: %w", err)
		}
		fileName := fileReader.FileName()
		fileWriter, err := fileWriterFunc(fileName)
		if err != nil {
			return err
		}
		defer fileutils.Close(fileWriter, &err)

		// Stream file directly to disk
		if _, err = io.Copy(fileWriter, fileReader); err != nil {
			return fmt.Errorf("failed writing '%s' file: %w", fileName, err)
		}
	}
	return nil
}

func exampleFunc(filepath string) (fileWriter io.WriteCloser, err error) {
	fileWriter, err = os.Create(filepath)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	return
}

func myMain(filepath string) {
	ReadFromStream(nil, exampleFunc)
}
