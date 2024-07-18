package io

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClose(t *testing.T) {
	var err error
	f, err := os.Create(filepath.Join(t.TempDir(), "test"))
	assert.NoError(t, err)

	Close(f, &err)
	assert.NoError(t, err)

	// Try closing the same file again and expect error
	Close(f, &err)
	assert.Error(t, err)

	// Check that both errors are aggregated
	err = errors.New("original error")
	Close(f, &err)
	assert.Len(t, strings.Split(err.Error(), "\n"), 2)

	nilErr := new(error)
	Close(f, nilErr)
	assert.NotNil(t, nilErr)
}

func TestFindFileInDirAndParents(t *testing.T) {
	const goModFileName = "go.mod"
	wd, err := os.Getwd()
	assert.NoError(t, err)
	projectRoot := filepath.Join(wd, "testdata", "project")

	// Find the file in the current directory
	root, err := FindFileInDirAndParents(projectRoot, goModFileName)
	assert.NoError(t, err)
	assert.Equal(t, projectRoot, root)

	// Find the file in the current directory's parent
	projectSubDirectory := filepath.Join(projectRoot, "dir")
	root, err = FindFileInDirAndParents(projectSubDirectory, goModFileName)
	assert.NoError(t, err)
	assert.Equal(t, projectRoot, root)

	// Look for a file that doesn't exist
	_, err = FindFileInDirAndParents(projectRoot, "notexist")
	assert.Error(t, err)
}

func TestReadNLines(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	path := filepath.Join(wd, "testdata", "oneline")
	lines, err := ReadNLines(path, 2)
	assert.NoError(t, err)
	assert.Len(t, lines, 1)
	assert.True(t, strings.HasPrefix(lines[0], ""))

	path = filepath.Join(wd, "testdata", "twolines")
	lines, err = ReadNLines(path, 2)
	assert.NoError(t, err)
	assert.Len(t, lines, 2)
	assert.True(t, strings.HasPrefix(lines[1], "781"))
	assert.True(t, strings.HasSuffix(lines[1], ":true}}}"))

	path = filepath.Join(wd, "testdata", "threelines")
	lines, err = ReadNLines(path, 2)
	assert.NoError(t, err)
	assert.Len(t, lines, 2)
	assert.True(t, strings.HasPrefix(lines[1], "781"))
	assert.True(t, strings.HasSuffix(lines[1], ":true}}}"))
}

func TestCreateTempDir(t *testing.T) {
	tempDir, err := CreateTempDir()
	assert.NoError(t, err)

	assert.DirExists(t, tempDir)

	defer func() {
		// Check that a timestamp can be extracted from the temp dir name
		timestamp, err := extractTimestamp(tempDir)
		assert.NoError(t, err)
		assert.False(t, timestamp.IsZero())

		assert.NoError(t, os.RemoveAll(tempDir))
	}()
}

func TestMoveFile_New(t *testing.T) {
	// Init test
	sourcePath, destPath := initMoveTest(t)

	// Move file
	assert.NoError(t, MoveFile(sourcePath, destPath))

	// Assert expected file paths
	assert.FileExists(t, destPath)
	assert.NoFileExists(t, sourcePath)
}

func TestMoveFile_Override(t *testing.T) {
	// Init test
	sourcePath, destPath := initMoveTest(t)
	err := os.WriteFile(destPath, []byte("dst"), os.ModePerm)
	assert.NoError(t, err)

	// Move file
	assert.NoError(t, MoveFile(sourcePath, destPath))

	// Assert file overidden
	assert.FileExists(t, destPath)
	destFileContent, err := os.ReadFile(destPath)
	assert.NoError(t, err)
	assert.Equal(t, "src", string(destFileContent))

	// Assert source file removed
	assert.NoFileExists(t, sourcePath)
}

func TestMoveFile_NoPerm(t *testing.T) {
	// Init test
	sourcePath, destPath := initMoveTest(t)
	err := os.WriteFile(destPath, []byte("dst"), os.ModePerm)
	assert.NoError(t, err)

	// Remove all permissions from destination file
	assert.NoError(t, os.Chmod(destPath, 0000))
	_, err = os.Create(destPath)
	assert.ErrorContains(t, err, "permission")

	// Move file
	assert.NoError(t, MoveFile(sourcePath, destPath))

	// Assert file overidden
	assert.FileExists(t, destPath)
	destFileContent, err := os.ReadFile(destPath)
	assert.NoError(t, err)
	assert.Equal(t, "src", string(destFileContent))
	
	// Assert source file removed
	assert.NoFileExists(t, sourcePath)
}

func initMoveTest(t *testing.T) (sourcePath, destPath string) {
	// Create source and destination paths
	tmpDir := t.TempDir()
	sourcePath = filepath.Join(tmpDir, "src")
	destPath = filepath.Join(tmpDir, "dst")

	// Write content to source file
	err := os.WriteFile(sourcePath, []byte("src"), os.ModePerm)
	assert.NoError(t, err)
	return
}
