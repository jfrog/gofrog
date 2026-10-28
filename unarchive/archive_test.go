package unarchive

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnarchive(t *testing.T) {
	tests := []string{"zip", "tar", "tar.gz"}
	uarchiver := Unarchiver{}
	for _, extension := range tests {
		t.Run(extension, func(t *testing.T) {
			// Create temp directory
			tmpDir := t.TempDir()

			// Run unarchive on archive created on Unix
			err := runUnarchive(t, uarchiver, "unix."+extension, "archives", filepath.Join(tmpDir, "unix"))
			assert.NoError(t, err)
			assert.FileExists(t, filepath.Join(tmpDir, "unix", "link"))
			assert.FileExists(t, filepath.Join(tmpDir, "unix", "dir", "file"))

			// Run unarchive on archive created on Windows
			err = runUnarchive(t, uarchiver, "win."+extension, "archives", filepath.Join(tmpDir, "win"))
			assert.NoError(t, err)
			assert.FileExists(t, filepath.Join(tmpDir, "win", "link.lnk"))
			assert.FileExists(t, filepath.Join(tmpDir, "win", "dir", "file.txt"))
		})
	}
}

var unarchiveSymlinksCases = []struct {
	prefix        string
	expectedFiles []string
}{
	{prefix: "softlink-rel", expectedFiles: []string{filepath.Join("softlink-rel", "a", "softlink-rel"), filepath.Join("softlink-rel", "b", "c", "d", "file")}},
	{prefix: "softlink-cousin", expectedFiles: []string{filepath.Join("a", "b", "softlink-cousin"), filepath.Join("a", "c", "d")}},
	{prefix: "softlink-uncle-file", expectedFiles: []string{filepath.Join("a", "b", "softlink-uncle"), filepath.Join("a", "c")}},
}

func TestUnarchiveSymlink(t *testing.T) {
	testExtensions := []string{"zip", "tar", "tar.gz"}
	uarchiver := Unarchiver{}
	for _, extension := range testExtensions {
		t.Run(extension, func(t *testing.T) {
			for _, testCase := range unarchiveSymlinksCases {
				t.Run(testCase.prefix, func(t *testing.T) {
					// Create temp directory
					tmpDir := t.TempDir()

					// Run unarchive
					err := runUnarchive(t, uarchiver, testCase.prefix+"."+extension, "archives", tmpDir)
					assert.NoError(t, err)

					// Assert the all expected files were extracted
					for _, expectedFiles := range testCase.expectedFiles {
						assert.FileExists(t, filepath.Join(tmpDir, expectedFiles))
					}
				})
			}
		})
	}
}

func TestUnarchiveZipSlip(t *testing.T) {
	tests := []struct {
		testType    string
		archives    []string
		errorSuffix string
	}{
		{"rel", []string{"zip", "tar", "tar.gz"}, "illegal path in archive: '../file'"},
		{"abs", []string{"tar", "tar.gz"}, "illegal path in archive: '/tmp/bla/file'"},
		{"softlink-abs", []string{"zip", "tar", "tar.gz"}, "illegal link path in archive: '/tmp/bla/file'"},
		{"softlink-rel", []string{"zip", "tar", "tar.gz"}, "illegal link path in archive: '../../file'"},
		{"softlink-loop", []string{"tar"}, "a link can't lead to an ancestor directory"},
		{"softlink-uncle", []string{"zip", "tar", "tar.gz"}, "a link can't lead to an ancestor directory"},
		{"hardlink-tilde", []string{"tar", "tar.gz"}, "walking hardlink: illegal link path in archive: '~/../../../../../../../../../Users/Shared/sharedFile.txt'"},
	}

	uarchiver := Unarchiver{}
	for _, test := range tests {
		t.Run(test.testType, func(t *testing.T) {
			// Create temp directory
			tmpDir := t.TempDir()

			for _, archive := range test.archives {
				// Unarchive and make sure an error returns
				err := runUnarchive(t, uarchiver, test.testType+"."+archive, "zipslip", tmpDir)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.errorSuffix)
			}
		})
	}
}

func TestUnarchiveAllowsDotsInFilename(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "dots.zip")
	archiveFile, err := os.Create(archivePath)
	require.NoError(t, err)
	zipWriter := zip.NewWriter(archiveFile)
	entry, err := zipWriter.Create("name..patch")
	require.NoError(t, err)
	_, err = entry.Write([]byte("patch"))
	require.NoError(t, err)
	require.NoError(t, zipWriter.Close())
	require.NoError(t, archiveFile.Close())

	destinationDir := t.TempDir()
	uarchiver := Unarchiver{}
	assert.NoError(t, uarchiver.Unarchive(archivePath, "dots.zip", destinationDir))
	assert.FileExists(t, filepath.Join(destinationDir, "name..patch"))
}

func TestUnarchiveRejectsDotPrefixedTraversal(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "traversal.zip")
	archiveFile, err := os.Create(archivePath)
	require.NoError(t, err)
	zipWriter := zip.NewWriter(archiveFile)
	entry, err := zipWriter.Create("./../outside.file")
	require.NoError(t, err)
	_, err = entry.Write([]byte("outside"))
	require.NoError(t, err)
	require.NoError(t, zipWriter.Close())
	require.NoError(t, archiveFile.Close())

	parentDir := t.TempDir()
	destinationDir := filepath.Join(parentDir, "destination")
	require.NoError(t, os.Mkdir(destinationDir, 0o755))
	uarchiver := Unarchiver{}
	err = uarchiver.Unarchive(archivePath, "traversal.zip", destinationDir)
	assert.ErrorContains(t, err, "illegal path in archive: './../outside.file'")
	assert.NoFileExists(t, filepath.Join(parentDir, "outside.file"))
}

func TestUnarchiveWithStripComponents(t *testing.T) {
	tests := []string{"zip", "tar", "tar.gz"}
	uarchiver := Unarchiver{}
	uarchiver.StripComponents = 1
	for _, extension := range tests {
		t.Run(extension, func(t *testing.T) {
			// Create temp directory
			tmpDir := t.TempDir()

			// Run unarchive on archive created on Unix
			err := runUnarchive(t, uarchiver, "strip-components."+extension, "archives", filepath.Join(tmpDir, "unix"))
			assert.NoError(t, err)
			assert.DirExists(t, filepath.Join(tmpDir, "unix", "nested_folder_1"))
			assert.DirExists(t, filepath.Join(tmpDir, "unix", "nested_folder_2"))

			// Run unarchive on archive created on Windows
			err = runUnarchive(t, uarchiver, "strip-components."+extension, "archives", filepath.Join(tmpDir, "win"))
			assert.NoError(t, err)
			assert.DirExists(t, filepath.Join(tmpDir, "win", "nested_folder_1"))
			assert.DirExists(t, filepath.Join(tmpDir, "win", "nested_folder_2"))
		})
	}
}

// Test unarchive file with a directory named "." in the root directory
func TestUnarchiveDotDir(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Run unarchive
	err := runUnarchive(t, Unarchiver{}, "dot-dir.tar.gz", "archives", tmpDir+string(os.PathSeparator))
	assert.NoError(t, err)
	assert.DirExists(t, filepath.Join(tmpDir, "dir"))
}

func runUnarchive(t *testing.T, uarchiver Unarchiver, archiveFileName, sourceDir, targetDir string) error {
	archivePath := filepath.Join("testdata", sourceDir, archiveFileName)
	assert.True(t, IsSupportedArchive(archivePath))
	return uarchiver.Unarchive(filepath.Join("testdata", sourceDir, archiveFileName), archiveFileName, targetDir)
}
