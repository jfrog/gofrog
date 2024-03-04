package io

import (
	"bufio"
	cr "crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type RandFile struct {
	*os.File
	Info os.FileInfo
}

const buflen = 4096

var src = rand.NewSource(time.Now().UnixNano())

// #nosec G404 - No cryptographic level encryption is needed in random file
var rnd = rand.New(src)

func CreateRandomLenFile(maxLen int, filesDir string, prefix string) string {
	file, _ := os.CreateTemp(filesDir, prefix)
	fname := file.Name()
	len := rnd.Intn(maxLen)
	created, err := CreateRandFile(fname, len)
	if err != nil {
		panic(err)
	}
	defer created.Close()
	// Check that the files were created with expected len
	if created.Info.Size() != int64(len) {
		panic(fmt.Errorf("unexpected file length. Expected: %d, Got %d", created.Info.Size(), len))
	}
	return fname
}

func CreateRandFile(path string, len int) (file *RandFile, err error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
	}()

	w := bufio.NewWriter(f)
	buf := make([]byte, buflen)

	for i := 0; i <= len; i += buflen {
		_, err := cr.Read(buf)
		if err != nil {
			return nil, err
		}
		var wbuflen = buflen
		if i+buflen >= len {
			wbuflen = len - i
		}
		wbuf := buf[0:wbuflen]
		_, err = w.Write(wbuf)
		if err != nil {
			return nil, err
		}
	}
	err = w.Flush()
	if err != nil {
		return nil, err
	}

	if info, err := f.Stat(); err != nil {
		return nil, err
	} else {
		file := RandFile{f, info}
		return &file, nil
	}
}

type WalkFunc func(path string, info os.FileInfo, err error) error
type Stat func(path string) (info os.FileInfo, err error)

var ErrSkipDir = errors.New("skip this directory")

var stat = os.Stat
var lStat = os.Lstat

func walk(path string, info os.FileInfo, walkFn WalkFunc, visitedDirSymlinks map[string]bool, walkIntoDirSymlink bool) error {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		realPath = path
	}
	isRealPathDir, err := IsDirExists(realPath, false)
	if err != nil {
		return err
	}
	if walkIntoDirSymlink && IsPathSymlink(path) && isRealPathDir {
		symlinkRealPath, err := evalPathOfSymlink(path)
		if err != nil {
			return err
		}
		visitedDirSymlinks[symlinkRealPath] = true
	}
	err = walkFn(path, info, nil)
	if err != nil {
		if info.IsDir() && err == ErrSkipDir {
			return nil
		}
		return err
	}

	if !info.IsDir() {
		return nil
	}

	names, err := readDirNames(path)
	if err != nil {
		return walkFn(path, info, err)
	}

	for _, name := range names {
		filename := filepath.Join(path, name)
		if walkIntoDirSymlink && IsPathSymlink(filename) {
			symlinkRealPath, err := evalPathOfSymlink(filename)
			if err != nil {
				return err
			}
			if visitedDirSymlinks[symlinkRealPath] {
				continue
			}
		}
		var fileHandler Stat
		if walkIntoDirSymlink {
			fileHandler = stat
		} else {
			fileHandler = lStat
		}
		fileInfo, err := fileHandler(filename)
		if err != nil {
			if err := walkFn(filename, fileInfo, err); err != nil && err != ErrSkipDir {
				return err
			}
		} else {
			err = walk(filename, fileInfo, walkFn, visitedDirSymlinks, walkIntoDirSymlink)
			if err != nil {
				if !fileInfo.IsDir() || err != ErrSkipDir {
					return err
				}
			}
		}
	}
	return nil
}

// The same as filepath.Walk the only difference is that we can walk into symlink.
// Avoiding infinite loops by saving the real paths we already visited.
func Walk(root string, walkFn WalkFunc, walkIntoDirSymlink bool) error {
	info, err := stat(root)
	visitedDirSymlinks := make(map[string]bool)
	if err != nil {
		return walkFn(root, nil, err)
	}
	return walk(root, info, walkFn, visitedDirSymlinks, walkIntoDirSymlink)
}

// Gets a path of a file or a directory, and returns its real path (in case the path contains a symlink to a directory).
// The difference between this function and filepath.EvalSymlinks is that if the path is of a symlink,
// this function won't return the symlink's target, but the real path to the symlink.
func evalPathOfSymlink(path string) (string, error) {
	dirPath := filepath.Dir(path)
	evalDirPath, err := filepath.EvalSymlinks(dirPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(evalDirPath, filepath.Base(path)), nil
}

// readDirNames reads the directory named by dirname and returns
// a sorted list of directory entries.
// The same as path/filepath readDirNames function
func readDirNames(dirname string) ([]string, error) {
	// #nosec G304 - False positive
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	err = f.Close()
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

func IsPathSymlink(path string) bool {
	f, _ := os.Lstat(path)
	return f != nil && IsFileSymlink(f)
}

func IsFileSymlink(file os.FileInfo) bool {
	return file.Mode()&os.ModeSymlink != 0
}

// Check if path points at a directory.
// If path points at a symlink and `followSymlink == false`,
// function will return `false` regardless of the symlink target
func IsDirExists(path string, followSymlink bool) (bool, error) {
	fileInfo, err := GetFileInfo(path, followSymlink)
	if err != nil {
		if os.IsNotExist(err) { // If doesn't exist, don't omit an error
			return false, nil
		}
		return false, err
	}
	return fileInfo.IsDir(), nil
}

// Get the file info of the file in path.
// If path points at a symlink and `followSymlink == false`, return the file info of the symlink instead
func GetFileInfo(path string, followSymlink bool) (fileInfo os.FileInfo, err error) {
	if followSymlink {
		fileInfo, err = os.Lstat(path)
	} else {
		fileInfo, err = os.Stat(path)
	}
	// We should not do CheckError here, because the error is checked by the calling functions.
	return fileInfo, err
}

// Close the reader/writer and append the error to the given error.
func Close(closer io.Closer, err *error) {
	var closeErr error
	if closeErr = closer.Close(); closeErr == nil {
		return
	}

	closeErr = fmt.Errorf("failed to close %T: %w", closer, closeErr)
	if err != nil {
		*err = errors.Join(*err, closeErr)
	}
}
