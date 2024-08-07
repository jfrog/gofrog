package crypto

import (
	"bufio"
	"regexp"

	// #nosec G501 -- md5 is supported by Artifactory.
	"crypto/md5"
	// #nosec G505 -- sha1 is supported by Artifactory.
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"os"

	ioutils "github.com/jfrog/gofrog/io"
	"github.com/minio/sha256-simd"
)

type Algorithm int

const (
	MD5 Algorithm = iota
	SHA1
	SHA256
)

var algorithmFunc = map[Algorithm]func() hash.Hash{
	// Go native crypto algorithms:
	MD5: md5.New,
	//#nosec G401 -- Sha1 is supported by Artifactory.
	SHA1: sha1.New,
	// sha256-simd algorithm:
	SHA256: sha256.New,
}

type Checksum struct {
	Sha1   string `json:"sha1,omitempty"`
	Md5    string `json:"md5,omitempty"`
	Sha256 string `json:"sha256,omitempty"`
}

func (c *Checksum) IsEmpty() bool {
	return c.Md5 == "" && c.Sha1 == "" && c.Sha256 == ""
}

// If the 'other' checksum matches the current one, return true.
// 'other' checksum may contain regex values for sha1, sha256 and md5.
func (c *Checksum) IsEqual(other Checksum) (bool, error) {
	match, err := regexp.MatchString(other.Md5, c.Md5)
	if !match || err != nil {
		return false, err
	}
	match, err = regexp.MatchString(other.Sha1, c.Sha1)
	if !match || err != nil {
		return false, err
	}
	match, err = regexp.MatchString(other.Sha256, c.Sha256)
	if !match || err != nil {
		return false, err
	}

	return true, nil
}

func GetFileChecksums(filePath string, checksumType ...Algorithm) (checksums map[Algorithm]string, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer ioutils.Close(file, &err)
	return CalcChecksums(file, checksumType...)
}

// CalcChecksums calculates all hashes at once using AsyncMultiWriter. The file is therefore read only once.
func CalcChecksums(reader io.Reader, checksumType ...Algorithm) (map[Algorithm]string, error) {
	hashes, err := calcChecksums(reader, checksumType...)
	if err != nil {
		return nil, err
	}
	results := sumResults(hashes)
	return results, nil
}

// CalcChecksumsBytes calculates hashes like `CalcChecksums`, returns result as bytes
func CalcChecksumsBytes(reader io.Reader, checksumType ...Algorithm) (map[Algorithm][]byte, error) {
	hashes, err := calcChecksums(reader, checksumType...)
	if err != nil {
		return nil, err
	}
	results := sumResultsBytes(hashes)
	return results, nil
}

func calcChecksums(reader io.Reader, checksumType ...Algorithm) (map[Algorithm]hash.Hash, error) {
	hashes := getChecksumByAlgorithm(checksumType...)
	var multiWriter io.Writer
	pageSize := os.Getpagesize()
	sizedReader := bufio.NewReaderSize(reader, pageSize)
	var hashWriter []io.Writer
	for _, v := range hashes {
		hashWriter = append(hashWriter, v)
	}
	multiWriter = ioutils.AsyncMultiWriter(pageSize, hashWriter...)
	_, err := io.Copy(multiWriter, sizedReader)
	if err != nil {
		return nil, err
	}
	return hashes, nil
}

func sumResults(hashes map[Algorithm]hash.Hash) map[Algorithm]string {
	results := map[Algorithm]string{}
	for k, v := range hashes {
		results[k] = fmt.Sprintf("%x", v.Sum(nil))
	}
	return results
}

func sumResultsBytes(hashes map[Algorithm]hash.Hash) map[Algorithm][]byte {
	results := map[Algorithm][]byte{}
	for k, v := range hashes {
		results[k] = v.Sum(nil)
	}
	return results
}

func getChecksumByAlgorithm(checksumType ...Algorithm) map[Algorithm]hash.Hash {
	hashes := map[Algorithm]hash.Hash{}
	if len(checksumType) == 0 {
		for k, v := range algorithmFunc {
			hashes[k] = v()
		}
		return hashes
	}

	for _, v := range checksumType {
		hashes[v] = algorithmFunc[v]()
	}
	return hashes
}

func CalcChecksumDetails(filePath string) (checksum Checksum, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer ioutils.Close(file, &err)

	checksums, err := CalcChecksums(file)
	if err != nil {
		return Checksum{}, err
	}
	checksum = Checksum{Md5: checksums[MD5], Sha1: checksums[SHA1], Sha256: checksums[SHA256]}
	return
}

type FileDetails struct {
	Checksum Checksum
	Size     int64
}

func GetFileDetails(filePath string, includeChecksums bool) (details *FileDetails, err error) {
	details = new(FileDetails)
	if includeChecksums {
		details.Checksum, err = CalcChecksumDetails(filePath)
		if err != nil {
			return
		}
	} else {
		details.Checksum = Checksum{}
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return
	}
	details.Size = fileInfo.Size()
	return
}
