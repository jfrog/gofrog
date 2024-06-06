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

	_, err = os.Stat(tempDir)
	assert.NotErrorIs(t, err, os.ErrNotExist)

	defer func() {
		// Check that a timestamp can be extracted from the temp dir name
		_, err = extractTimestamp(tempDir)
		assert.NoError(t, err)

		assert.NoError(t, os.RemoveAll(tempDir))
	}()
}
