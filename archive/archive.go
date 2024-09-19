package archive

import (
	"fmt"
	"github.com/jfrog/archiver/v3"
	"github.com/jfrog/gofrog/io"
	"os"
	"path/filepath"
)

// FilterFunc is a function that determines whether a file should be excluded from an archive.
type FilterFunc func(path string, info os.FileInfo) bool

// ArchiveWithFilterFunc creates an archive from the specified source directory, excluding files based on the filter function.
// The archive is saved to the specified destination file.
// The rootDir is used to calculate the relative path of the files in the archive.
// The srcDir is the directory to be archived.
// The destination is the path to the archive file to be created. The file extension determines the archive format (e.g., .zip, .tar).
// The filterFunc is a function that determines whether a file should be excluded from an archive.
func ArchiveWithFilterFunc(rootDir, srcDir string, destination string, filterFunc FilterFunc) (err error) {
	// Identify the archive format based on the file extension (e.g., .zip, .tar)
	archive, err := archiver.ByExtension(destination)
	if err != nil {
		return err
	}

	// Ensure the identified format supports creating an archive
	archiveWriter, ok := archive.(archiver.Writer)
	if !ok {
		return fmt.Errorf("unsupported archive format for writing: %s", destination)
	}

	// Open the output file for the archive
	out, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed while creating destination file %s: %v", destination, err)
	}
	defer io.Close(out, &err)

	// Initialize the archive (ZIP, TAR, etc.)
	err = archiveWriter.Create(out)
	if err != nil {
		return fmt.Errorf("failed while creating archive: %v", err)
	}
	defer io.Close(archiveWriter, &err)

	// Walk through each source file and directory
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) (walkErr error) {
		if err != nil {
			return err
		}
		// Filter out files that should not be included in the archive
		excluded := filterFunc(path, info)
		if excluded {
			if info.IsDir() {
				// Return SkipDir error to avoid walking into the directory
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories and symlinks
		if info.IsDir() || io.IsFileSymlink(info) {
			return nil
		}

		// Calculate the relative path to preserve the folder structure
		relativePath, walkErr := filepath.Rel(filepath.Dir(rootDir), path)
		if walkErr != nil {
			return walkErr
		}

		// Open the file
		file, walkErr := os.Open(path)
		if walkErr != nil {
			return walkErr
		}
		defer io.Close(archiveWriter, &walkErr)

		// Add the file to the archive, using the relative path
		return archiveWriter.Write(archiver.File{
			FileInfo: archiver.FileInfo{
				FileInfo: info,
				// Preserve the relative path in the archive
				CustomName: relativePath,
			},
			ReadCloser: file,
		})
	})
}
