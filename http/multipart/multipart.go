package multipart

import (
	"errors"
	"fmt"
	"github.com/cespare/xxhash"
	ioutils "github.com/jfrog/gofrog/io"
	"hash"
	"io"
	"mime/multipart"
	"os"
)

type FileWriterFunc func(fileName string) (io.Writer, error)

func ReadFilesFromStream(multipartReader *multipart.Reader, fileWriterFunc FileWriterFunc) error {
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

		if _, err = io.Copy(fileWriter, fileReader); err != nil {
			return fmt.Errorf("failed writing '%s' file: %w", fileName, err)
		}
	}
	return nil
}

func WriteFilesToStream(writer io.Writer, filesList []string) error {
	multipartWriter := multipart.NewWriter(writer)
	for _, filePath := range filesList {
		fileReader, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer ioutils.Close(fileReader, &err)
		fileWriter, err := multipartWriter.CreateFormFile("fieldname", filePath)
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

func getFileWriter(fileName string) (fileWriter io.Writer, err error) {
	lockfileMock := map[string]string{
		"SDFDSFSDFSDFDSF":   "file1",
		"XXSDFDSFSDFSDFDSF": "file2",
	}
	realFileName := lockfileMock[fileName]
	fileWriter, err = os.Create(realFileName)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	// Currently we are using the file name as the hash
	fileHash := fileName
	return io.MultiWriter(fileWriter, NewHashWrapper(fileHash)), nil
}

type HashWrapper struct {
	hash           hash.Hash64
	actualChecksum string
}

func NewHashWrapper(actualChecksum string) *HashWrapper {
	return &HashWrapper{hash: xxhash.New(), actualChecksum: actualChecksum}
}

func (hw *HashWrapper) Write(p []byte) (n int, err error) {
	n, err = hw.hash.Write(p)
	if fmt.Sprintf("%x", hw.hash.Sum(nil)) != hw.actualChecksum {
		err = errors.New("checksum mismatch")
	}
	return
}

func myMain(filepath string) {
	ReadFilesFromStream(nil, getFileWriter)
}
