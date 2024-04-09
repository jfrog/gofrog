package filestream

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var targetDir string

func TestWriteFilesToStreamAndReadFilesFromStream(t *testing.T) {
	sourceDir := t.TempDir()
	// Create 2 file to be transferred via our multipart stream
	file1 := &FileInfo{Name: "test1.txt", Path: filepath.Join(sourceDir, "test1.txt")}
	file2 := &FileInfo{Name: "test2.txt", Path: filepath.Join(sourceDir, "test2.txt")}
	file1Content := []byte("test content1")
	file2Content := []byte("test content2")
	assert.NoError(t, os.WriteFile(file1.Path, file1Content, 0600))
	assert.NoError(t, os.WriteFile(file2.Path, file2Content, 0600))

	// Create the multipart writer that will stream our files
	body := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(body)
	assert.NoError(t, WriteFilesToStream(multipartWriter, []*FileInfo{file1, file2}))

	// Create local temp dir that will store our files
	targetDir = t.TempDir()

	// Create the multipart reader that will read the files from the stream
	multipartReader := multipart.NewReader(body, multipartWriter.Boundary())
	assert.NoError(t, ReadFilesFromStream(multipartReader, simpleFileWriter))

	// Validate file 1 transferred successfully
	content, err := os.ReadFile(filepath.Join(targetDir, file1.Name))
	assert.NoError(t, err)
	assert.Equal(t, file1Content, content)

	// Validate file 2 transferred successfully
	content, err = os.ReadFile(filepath.Join(targetDir, file2.Name))
	assert.NoError(t, err)
	assert.Equal(t, file2Content, content)
}

func TestWriteFilesToStreamWithError(t *testing.T) {
	// Create a temporary directory for the test
	sourceDir := t.TempDir()

	nonExistentFileName := "nonexistent.txt"
	// Create a FileInfo with a non-existent file
	file := &FileInfo{Name: nonExistentFileName, Path: filepath.Join(sourceDir, nonExistentFileName)}

	// Create a buffer and a multipart writer
	body := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(body)

	// Call WriteFilesToStream and expect an error
	err := WriteFilesToStream(multipartWriter, []*FileInfo{file})
	require.NoError(t, err)

	multipartReader := multipart.NewReader(body, multipartWriter.Boundary())
	form, err := multipartReader.ReadForm(10 * 1024)
	require.NoError(t, err)

	assert.Len(t, form.Value[ErrorType], 1)
	var multipartErr MultipartError
	assert.NoError(t, json.Unmarshal([]byte(form.Value[ErrorType][0]), &multipartErr))

	assert.Equal(t, nonExistentFileName, multipartErr.FileName)
	assert.NotEmpty(t, multipartErr.Error())
}

func simpleFileWriter(fileName string) (fileWriter []io.WriteCloser, err error) {
	writer, err := os.Create(filepath.Join(targetDir, fileName))
	if err != nil {
		return nil, err
	}
	return []io.WriteCloser{writer}, nil
}
