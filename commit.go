package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const blobHeaderDelim byte = 0
const bufferSize int = 4096

type commit struct {
	Message    string            // User supplied commit message.
	Timestamp  int64             // When the commit was created in UNIX time in UTC.
	FileToBlob map[string]string // Map of file names to file blob UIDs tracked in the commit.
	ParentUIDs [2]string         // SHA1 hash of the parent commit. Merge commits have two parents.
}

func (c *commit) String(hash string) string {
	if isMergeCommit := c.ParentUIDs[1] != ""; isMergeCommit {
		return fmt.Sprintf(
			"commit %v\n"+
				"Merge: %v %v\n"+
				"Date: %v\n"+
				"%v\n",
			hash,
			c.ParentUIDs[0][:6], c.ParentUIDs[1][:6],
			time.Unix(c.Timestamp, 0).Local().Format("Mon Jan 02 15:04:05 2006 -0700"),
			c.Message,
		)
	}
	return fmt.Sprintf(
		"commit %v\n"+
			"Date: %v\n"+
			"%v\n",
		hash,
		time.Unix(c.Timestamp, 0).Local().Format("Mon Jan 02 15:04:05 2006 -0700"),
		c.Message,
	)
}

func getHeadCommitHash() (string, error) {
	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return "", fmt.Errorf("getHeadCommitHash: %w", err)
	}
	headCommitHash, err := readContentsAsString(currentBranchFile)
	if err != nil {
		return "", fmt.Errorf("getHeadCommitHash: %w", err)
	}
	return headCommitHash, nil
}

func getHeadCommit() (commit, error) {
	var c commit
	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return c, fmt.Errorf("getHeadCommit: %w", err)
	}
	headCommitHash, err := readContentsAsString(currentBranchFile)
	if err != nil {
		return c, fmt.Errorf("getHeadCommit: %w", err)
	}
	c, err = getCommit(headCommitHash)
	if err != nil {
		return c, fmt.Errorf("getHeadCommit: %w", err)
	}
	return c, nil
}

func writeCommitBlob(c commit) error {
	b, err := serialize(c)
	if err != nil {
		return err
	}
	return writeBlob("commit", b)
}

func writeFileBlob(file string) error {
	b, err := readContents(file)
	if err != nil {
		return err
	}
	return writeBlob("file", b)
}

// parseBlobHeader returns a blob's header given the hash of the blob.
func parseBlobHeader(hash string) (string, error) {
	f, err := os.Open(filepath.Join(objectsDir, hash))
	if err != nil {
		return "", fmt.Errorf("parseBlobHeader: %w", err)
	}
	defer f.Close()
	reader := bufio.NewReader(f)
	header, err := reader.ReadBytes(blobHeaderDelim)
	if err != nil {
		return "", err
	}
	header = bytes.TrimSuffix(header, []byte{blobHeaderDelim})
	return string(header), f.Close()
}

// readBlob returns the header and contents of a blob given the hash of the blob.
func readBlob(hash string) (string, []byte, error) {
	var header string
	var contents []byte
	f, err := os.Open(filepath.Join(objectsDir, hash))
	if err != nil {
		return header, contents, fmt.Errorf("readBlob: %w", err)
	}
	defer f.Close()
	reader := bufio.NewReader(f)

	headerBytes, err := reader.ReadBytes(blobHeaderDelim)
	if err != nil {
		return header, contents, fmt.Errorf("readBlob: %w", err)
	}
	header = string(bytes.TrimSuffix(headerBytes, []byte{blobHeaderDelim}))

	contents = make([]byte, bufferSize)
	bytesRead, err := reader.Read(contents)
	if err != nil {
		return header, contents, fmt.Errorf("readBlob: %w", err)
	}
	return header, contents[:bytesRead], f.Close()
}

// Get commit object given the hash of the commit blob.
// Returns an error if the blob is not a commit blob.
func getCommit(hash string) (commit, error) {
	var c commit
	var err error
	if len(hash) < 40 {
		hash, err = resolveHash(hash)
		if err != nil {
			return c, fmt.Errorf("getCommit: could not resolve hash %v: %w", hash, err)
		}
	}

	header, contents, err := readBlob(hash)
	if err != nil {
		return c, fmt.Errorf("getCommit: %w", err)
	}
	if header != "commit" {
		return c, fmt.Errorf("getCommit: incorrect blob header, want 'commit', got '%v'", header)
	}
	c, err = deserialize[commit](contents)
	if err != nil {
		return c, fmt.Errorf("getCommit: %w", err)
	}
	return c, nil
}

func writeBlob(header string, b []byte) error {
	payload := []any{header, []byte{blobHeaderDelim}, b}
	hash, err := getHash(payload)
	if err != nil {
		return err
	}
	blobFile := filepath.Join(objectsDir, hash)
	return writeContents(blobFile, payload)
}

// resolveHash matches the given hash abbreviation and returns the corresponding a full
// hash in the objects directory.
func resolveHash(hash string) (string, error) {
	blobFiles, err := getFilenames(objectsDir)
	if err != nil {
		return "", fmt.Errorf("resolveHash: %w", err)
	}
	var matched []string
	for _, file := range blobFiles {
		if strings.HasPrefix(file, hash) {
			matched = append(matched, file)
		}
	}
	if len(matched) < 1 {
		return "", errors.New("resolveHash: no matching blobs found")
	} else if len(matched) > 1 {
		return "", errors.New("resolveHash: ambiguous hash prefix")
	} else {
		return matched[0], nil
	}
}
