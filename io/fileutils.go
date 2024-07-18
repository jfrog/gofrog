package io

import (
	"bufio"
	cr "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jfrog/gofrog/log"
)

const (
	tempDirPrefix = "temp-"

	// Max temp file age in hours
	maxFileAge = 24.0
)

type RandFile struct {
	*os.File
	Info os.FileInfo
}

const buflen = 4096

var src = rand.NewSource(time.Now().UnixNano())

// #nosec G404 - No cryptographic level encryption is needed in random file
var rnd = rand.New(src)

// Create a temp file with the requested prefix at the provided dir. File length and contents are random, up to the requested max length.
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

// Create a file at the provided path with a request number of random bytes.
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

// Checxk if path points at a file.
// If path points at a symlink and `followSymlink == false`,
// function will return `true` regardless of the symlink target
func IsFileExists(path string, followSymlink bool) (bool, error) {
	fileInfo, err := GetFileInfo(path, followSymlink)
	if err != nil {
		if os.IsNotExist(err) { // If doesn't exist, don't omit an error
			return false, nil
		}
		return false, err
	}
	return !fileInfo.IsDir(), nil
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

// Move directory content from one path to another.
func MoveDir(fromPath, toPath string) error {
	err := CreateDirIfNotExist(toPath)
	if err != nil {
		return err
	}

	files, err := ListFiles(fromPath, true)
	if err != nil {
		return err
	}

	for _, v := range files {
		dir, err := IsDirExists(v, true)
		if err != nil {
			return err
		}

		if dir {
			toPath := toPath + GetFileSeparator() + filepath.Base(v)
			err := MoveDir(v, toPath)
			if err != nil {
				return err
			}
			continue
		}
		err = MoveFile(v, filepath.Join(toPath, filepath.Base(v)))
		if err != nil {
			return err
		}
	}
	return err
}

// GoLang: os.Rename() give error "invalid cross-device link" for Docker container with Volumes.
// MoveFile(source, destination) will work moving file between folders
// Therefore, we are using our own implementation (MoveFile) in order to rename files.
func MoveFile(sourcePath, destPath string) (err error) {
	inputFileOpen := true
	var inputFile *os.File
	inputFile, err = os.Open(sourcePath)
	if err != nil {
		return
	}
	defer func() {
		if inputFileOpen {
			err = errors.Join(err, inputFile.Close())
		}
	}()
	inputFileInfo, err := inputFile.Stat()
	if err != nil {
		return
	}

	outputFile, err := createFileForWriting(destPath)
	if err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, outputFile.Close())
	}()

	_, err = io.Copy(outputFile, inputFile)
	if err != nil {
		return
	}
	err = os.Chmod(destPath, inputFileInfo.Mode())
	if err != nil {
		return
	}

	// The copy was successful, so now delete the original file
	err = inputFile.Close()
	if err != nil {
		return
	}
	inputFileOpen = false
	err = os.Remove(sourcePath)
	return
}

// Create file for writing. If file already exists, it will be truncated.
// If the file exists and is read-only, the function will try to change the file permissions to read-write.
// The caller should update the permissions and close the file when done.
func createFileForWriting(destPath string) (*os.File, error) {
	// Try to create the destination file
	outputFile, err := os.Create(destPath)
	if err == nil {
		return outputFile, nil
	}

	log.Debug(fmt.Sprintf("Couldn't to open the destination file: '%s' due to %s. Attempting to set the file permissions to read-write.", destPath, err.Error()))
	if chmodErr := os.Chmod(destPath, 0600); chmodErr != nil {
		return nil, errors.Join(err, chmodErr)
	}

	// Try to open the destination file again
	return os.Create(destPath)
}

// Return the list of files and directories in the specified path
func ListFiles(path string, includeDirs bool) ([]string, error) {
	sep := GetFileSeparator()
	if !strings.HasSuffix(path, sep) {
		path += sep
	}
	fileList := []string{}
	files, _ := os.ReadDir(path)
	path = strings.TrimPrefix(path, "."+sep)

	for _, f := range files {
		filePath := path + f.Name()
		exists, err := IsFileExists(filePath, false)
		if err != nil {
			return nil, err
		}
		if exists || IsPathSymlink(filePath) {
			fileList = append(fileList, filePath)
		} else if includeDirs {
			isDir, err := IsDirExists(filePath, false)
			if err != nil {
				return nil, err
			}
			if isDir {
				fileList = append(fileList, filePath)
			}
		}
	}
	return fileList, nil
}

// Return all files in the specified path who satisfy the filter func. Not recursive.
func ListFilesByFilterFunc(path string, filterFunc func(filePath string) (bool, error)) ([]string, error) {
	sep := GetFileSeparator()
	if !strings.HasSuffix(path, sep) {
		path += sep
	}
	var fileList []string
	files, _ := os.ReadDir(path)
	path = strings.TrimPrefix(path, "."+sep)

	for _, f := range files {
		filePath := path + f.Name()
		satisfy, err := filterFunc(filePath)
		if err != nil {
			return nil, err
		}
		if !satisfy {
			continue
		}
		exists, err := IsFileExists(filePath, false)
		if err != nil {
			return nil, err
		}
		if exists {
			fileList = append(fileList, filePath)
			continue
		}

		// Checks if the filepath is a symlink.
		if IsPathSymlink(filePath) {
			// Gets the file info of the symlink.
			file, err := GetFileInfo(filePath, false)
			if err != nil {
				return nil, err
			}
			// Checks if the symlink is a file.
			if !file.IsDir() {
				fileList = append(fileList, filePath)
			}
		}
	}
	return fileList, nil
}

func DownloadFile(downloadTo string, fromUrl string) (err error) {
	// Get the data
	httpClient := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, fromUrl, nil)
	if err != nil {
		return err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed. status code: %s", resp.Status)
	}
	// Create the file
	var out *os.File
	out, err = os.Create(downloadTo)
	if err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, out.Close())
	}()
	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return
}

func DoubleWinPathSeparator(filePath string) string {
	return strings.ReplaceAll(filePath, "\\", "\\\\")
}

// IsPathExists checks if a path exists.
func IsPathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func GetFileContentAndInfo(filePath string) (fileContent []byte, fileInfo os.FileInfo, err error) {
	fileInfo, err = os.Stat(filePath)
	if err != nil {
		return
	}
	fileContent, err = os.ReadFile(filePath)
	return
}

// CreateTempDir creates a temporary directory and returns its path.
func CreateTempDir() (string, error) {
	tempDirBase := os.TempDir()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	return os.MkdirTemp(tempDirBase, tempDirPrefix+timestamp+"-*")
}

func RemoveTempDir(dirPath string) error {
	exists, err := IsDirExists(dirPath, false)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	err = os.RemoveAll(dirPath)
	if err == nil {
		return nil
	}
	// Sometimes removing the directory fails (in Windows) because it's locked by another process.
	// That's a known issue, but its cause is unknown (golang.org/issue/30789).
	// In this case, we'll only remove the contents of the directory, and let CleanOldDirs() remove the directory itself at a later time.
	return removeDirContents(dirPath)
}

// RemoveDirContents removes the contents of the directory, without removing the directory itself.
// If it encounters an error before removing all the files, it stops and returns that error.
func removeDirContents(dirPath string) (err error) {
	d, err := os.Open(dirPath)
	if err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, d.Close())
	}()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dirPath, name))
		if err != nil {
			return
		}
	}
	return
}

// Old runs/tests may leave junk at temp dir.
// Each temp file/Dir is named with prefix+timestamp, search for all temp files/dirs that match the common prefix and validate their timestamp.
func CleanOldDirs() error {
	// Get all files at temp dir
	tempDirBase := os.TempDir()
	files, err := os.ReadDir(tempDirBase)
	if err != nil {
		return err
	}
	now := time.Now()
	// Search for files/dirs that match the template.
	for _, file := range files {
		if strings.HasPrefix(file.Name(), tempDirPrefix) {
			timeStamp, err := extractTimestamp(file.Name())
			if err != nil {
				return err
			}
			// Delete old file/dirs.
			if now.Sub(timeStamp).Hours() > maxFileAge {
				if err := os.RemoveAll(path.Join(tempDirBase, file.Name())); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func extractTimestamp(item string) (time.Time, error) {
	// Get timestamp from file/dir.
	endTimestampIndex := strings.LastIndex(item, "-")
	beginningTimestampIndex := strings.LastIndex(item[:endTimestampIndex], "-")
	timestampStr := item[beginningTimestampIndex+1 : endTimestampIndex]
	// Convert to int.
	timestampInt, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	// Convert to time type.
	return time.Unix(timestampInt, 0), nil
}

// FindFileInDirAndParents looks for a file named fileName in dirPath and its parents, and returns the path of the directory where it was found.
// dirPath must be a full path.
func FindFileInDirAndParents(dirPath, fileName string) (string, error) {
	// Create a map to store all paths visited, to avoid running in circles.
	visitedPaths := make(map[string]bool)
	currDir := dirPath
	for {
		// If the file is found in the current directory, return the path.
		exists, err := IsFileExists(filepath.Join(currDir, fileName), true)
		if err != nil || exists {
			return currDir, err
		}

		// Save this path.
		visitedPaths[currDir] = true

		// CD to the parent directory.
		currDir = filepath.Dir(currDir)

		// If we already visited this directory, it means that there's a loop, and we can stop.
		if visitedPaths[currDir] {
			return "", fmt.Errorf("could not find the %s file of the project", fileName)
		}
	}
}

// Copy directory content from one path to another.
// includeDirs means to copy also the dirs if presented in the src folder.
// excludeNames - Skip files/dirs in the src folder that match names in provided slice. ONLY excludes first layer (only in src folder).
func CopyDir(fromPath, toPath string, includeDirs bool, excludeNames []string) error {
	err := CreateDirIfNotExist(toPath)
	if err != nil {
		return err
	}

	files, err := ListFiles(fromPath, includeDirs)
	if err != nil {
		return err
	}

	for _, file := range files {
		fileName := filepath.Base(file)
		// Skip if excluded
		if slices.Contains(excludeNames, fileName) {
			continue
		}
		var isDir bool
		isDir, err = IsDirExists(file, false)
		if err != nil {
			return err
		}

		if isDir {
			err = CopyDir(file, filepath.Join(toPath, fileName), true, nil)
		} else {
			err = CopyFile(toPath, file)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func CopyFile(dst, src string) (err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, srcFile.Close())
	}()
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return
	}
	fileName, _ := GetFileAndDirFromPath(src)
	dstPath, err := CreateFilePath(dst, fileName)
	if err != nil {
		return
	}
	dstFile, err := os.OpenFile(dstPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, dstFile.Close())
	}()
	_, err = io.Copy(dstFile, srcFile)
	return
}

func GetFileSeparator() string {
	return string(os.PathSeparator)
}

// Return the file's name and dir of a given path by finding the index of the last separator in the path.
// Support separators : "/" , "\\" and "\\\\"
func GetFileAndDirFromPath(path string) (fileName, dir string) {
	index1 := strings.LastIndex(path, "/")
	index2 := strings.LastIndex(path, "\\")
	var index int
	offset := 0
	if index1 >= index2 {
		index = index1
	} else {
		index = index2
		// Check if the last separator is "\\\\" or "\\".
		index3 := strings.LastIndex(path, "\\\\")
		if index3 != -1 && index2-index3 == 1 {
			offset = 1
		}
	}
	if index != -1 {
		fileName = path[index+1:]
		// If the last separator is "\\\\" index will contain the index of the last "\\" ,
		// to get the dir path (without separator suffix) we will use the offset's value.
		dir = path[:index-offset]
		return
	}
	fileName = path
	dir = ""
	return
}

func CreateFilePath(localPath, fileName string) (string, error) {
	if localPath != "" {
		err := os.MkdirAll(localPath, 0750)
		if err != nil {
			return "", err
		}
		fileName = filepath.Join(localPath, fileName)
	}
	return fileName, nil
}

func CreateDirIfNotExist(path string) error {
	exist, err := IsDirExists(path, false)
	if exist || err != nil {
		return err
	}
	_, err = CreateFilePath(path, "")
	return err
}

func IsPathSymlink(path string) bool {
	f, _ := os.Lstat(path)
	return f != nil && IsFileSymlink(f)
}

func IsFileSymlink(file os.FileInfo) bool {
	return file.Mode()&os.ModeSymlink != 0
}

// Parses the JSON-encoded data and stores the result in the value pointed to by 'loadTarget'.
// filePath - Path to json file.
// loadTarget - Pointer to a struct
func Unmarshal(filePath string, loadTarget interface{}) (err error) {
	var jsonFile *os.File
	jsonFile, err = os.Open(filePath)
	if err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, jsonFile.Close())
	}()
	var byteValue []byte
	byteValue, err = io.ReadAll(jsonFile)
	if err != nil {
		return
	}
	err = json.Unmarshal(byteValue, &loadTarget)
	return
}

// strip '\n' or read until EOF, return error if read error
// readNLines reads up to 'total' number of lines separated by \n.
func ReadNLines(path string, total int) (lines []string, err error) {
	reader, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, reader.Close())
	}()
	bufferedReader := bufio.NewReader(reader)
	for i := 0; i < total; i++ {
		var line []byte
		line, _, err = bufferedReader.ReadLine()
		lines = append(lines, string(line))
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}
	}
	return
}
