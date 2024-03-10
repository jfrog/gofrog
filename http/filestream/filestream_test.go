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

func TestWriteFilesToStreamAndReadFilesFromStream(t *testing.T) {
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
	assert.NoError(t, ReadFilesFromStream(multipartReader, fileHandlerWithHashValidation))

	// Validate file 1 transferred successfully
	file1 = filepath.Join(targetDir, "test1.txt")
	assert.FileExists(t, file1)
	content, err := os.ReadFile(file1)
	assert.NoError(t, err)
	assert.Equal(t, file1Content, content)

	// Validate file 2 transferred successfully
	file2 = filepath.Join(targetDir, "test2.txt")
	assert.FileExists(t, file2)
	content, err = os.ReadFile(file2)
	assert.NoError(t, err)
	assert.Equal(t, file2Content, content)
}

func simpleFileHandler(fileName string) (fileWriter io.Writer, err error) {
	return os.OpenFile(filepath.Join(targetDir, fileName), os.O_RDWR|os.O_CREATE, 0777)
}

func fileHandlerWithHashValidation(fileName string) (fileWriter io.Writer, err error) {
	fileWriter, err = simpleFileHandler(fileName)
	if err != nil {
		return
	}
	// GetExpectedHashFromLockFile(fileName)
	lockFileMock := map[string]string{
		"file1": "070afab2066d3b16",
		"file2": "070afab2066d3b16",
	}
	return io.MultiWriter(
		fileWriter,
		&HashValidator{hash: xxh3.New(), actualChecksum: lockFileMock[fileName]},
	), nil
}

type HashValidator struct {
	hash           hash.Hash64
	actualChecksum string
}

func (hw *HashValidator) Write(p []byte) (n int, err error) {
	n, err = hw.hash.Write(p)
	sd := fmt.Sprintf("%x", hw.hash.Sum(nil))
	if sd != hw.actualChecksum {
		err = errors.New("checksum mismatch")
	}
	return
}
