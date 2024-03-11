package filestream

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
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
	body := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(body)
	assert.NoError(t, WriteFilesToStream(multipartWriter, []string{file1, file2}))

	// Create local temp dir that will store our files
	targetDir = t.TempDir()

	// Create the multipart reader that will read the files from the stream
	multipartReader := multipart.NewReader(body, multipartWriter.Boundary())
	assert.NoError(t, ReadFilesFromStream(multipartReader, simpleFileHandler))

	// Validate file 1 transferred successfully
	file1 = filepath.Join(targetDir, "test1.txt")
	assert.FileExists(t, file1)
	content, err := os.ReadFile(file1)
	assert.NoError(t, err)
	assert.Equal(t, file1Content, content)
	assert.NoError(t, os.Remove(file1))

	// Validate file 2 transferred successfully
	file2 = filepath.Join(targetDir, "test2.txt")
	assert.FileExists(t, file2)
	content, err = os.ReadFile(file2)
	assert.NoError(t, err)
	assert.Equal(t, file2Content, content)
	assert.NoError(t, os.Remove(file2))
}

func simpleFileHandler(fileName string) (fileWriter io.WriteCloser, err error) {
	return os.Create(filepath.Join(targetDir, fileName))
}
