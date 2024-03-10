package filestream

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/zeebo/xxh3"
	"hash"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var targetDir string

func TestReadFilesFromStream(t *testing.T) {
	sourceDir := t.TempDir()

	// Create 2 file to be transferred via our multipart stream
	file1 := filepath.Join(sourceDir, "test1.txt")
	file2 := filepath.Join(sourceDir, "test2.txt")
	file1Content := []byte("test content1")
	file2Content := []byte("test content2")
	assert.NoError(t, os.WriteFile(file1, file1Content, 0600))
	assert.NoError(t, os.WriteFile(file2, file2Content, 0600))

	// Create the multipart writer that will stream our files
	responseWriter := httptest.NewRecorder()
	assert.NoError(t, WriteFilesToStream(responseWriter, []string{file1, file2}))

	// Create local temp dir that will store our files
	targetDir = t.TempDir()

	// Get boundary hash from writer
	boundary := strings.Split(responseWriter.Header().Get(contentType), "boundary=")[1]
	// Create the multipart reader that will read the files from the stream
	multipartReader := multipart.NewReader(responseWriter.Body, boundary)
	assert.NoError(t, ReadFilesFromStream(multipartReader, simpleFileHandler))

	// Validate file 1 transferred successfully
	content, err := os.ReadFile(filepath.Join(targetDir, "test1.txt"))
	assert.NoError(t, err)
	assert.Equal(t, file1Content, content)

	// Validate file 2 transferred successfully
	content, err = os.ReadFile(filepath.Join(targetDir, "test2.txt"))
	assert.NoError(t, err)
	assert.Equal(t, file2Content, content)

}

func simpleFileHandler(fileName string) (fileWriter io.Writer, err error) {
	return os.Create(filepath.Join(targetDir, fileName))
}

func fileHandlerWithHash(fileName string) (fileWriter io.Writer, err error) {
	fileWriter, err = simpleFileHandler(fileName)
	if err != nil {
		return
	}
	// GetExpectedHashFromLockFile(fileName)
	expectedHash := "SDFDSFSDFSDFDSF"
	return io.MultiWriter(fileWriter, NewHashWrapper(expectedHash)), nil
}

type HashWrapper struct {
	hash           hash.Hash64
	actualChecksum string
}

func NewHashWrapper(actualChecksum string) *HashWrapper {
	return &HashWrapper{hash: xxh3.New(), actualChecksum: actualChecksum}
}

func (hw *HashWrapper) Write(p []byte) (n int, err error) {
	n, err = hw.hash.Write(p)
	if fmt.Sprintf("%x", hw.hash.Sum(nil)) != hw.actualChecksum {
		err = errors.New("checksum mismatch")
	}
	return
}
