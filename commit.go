package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const blobHeaderDelim byte = 0

type commit struct {
	Message    string            // User supplied commit message.
	Timestamp  int64             // When the commit was created in UNIX time in UTC.
	FileToBlob map[string]string // Map of file names to file blob UIDs tracked in the commit.
	ParentUIDs [2]string         // SHA1 hash of the parent commit. Merge commits should have two parents.
}

func (c *commit) String(hash string) string {
	if c.ParentUIDs[1] != "" {
		return fmt.Sprintf(
			"commit %v\n"+
				"Merge: %v %v\n"+
				"Date: %v\n"+
				"%v\n"+
				"Merged %v into %v.\n",
			hash,
			c.ParentUIDs[0][:6], c.ParentUIDs[1][:6],
			time.Unix(c.Timestamp, 0).Local().Format("Mon Jan 02 15:04:05 2006 -0700"),
			c.Message,
			c.ParentUIDs[0], c.ParentUIDs[1],
		)
	} else {
		return fmt.Sprintf(
			"commit %v\n"+
				"Date: %v\n"+
				"%v\n",
			hash,
			time.Unix(c.Timestamp, 0).Local().Format("Mon Jan 02 15:04:05 2006 -0700"),
			c.Message,
		)
	}
}

// getCommit returns a commit given its hash.
func getCommit(hash string) (commit, error) {
	var c commit
	commitData, err := readContents(filepath.Join(objectsDir, hash))
	if err != nil {
		return c, err
	}
	c, err = deserialize[commit](commitData)
	if err != nil {
		return c, err
	}
	return c, nil
}

func getHeadCommit() (commit, error) {
	var c commit
	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return c, err
	}
	headCommitHash, err := readContentsAsString(currentBranchFile)
	if err != nil {
		return c, err
	}
	c, err = getCommit(headCommitHash)
	if err != nil {
		return c, err
	}
	return c, nil
}

func writeCommitBlob(c commit) error {
	b, err := serialize[commit](c)
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

func getBlobHeader(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(f)
	reader.ReadString(blobHeaderDelim)
	return nil
}

func writeBlob(header string, b []byte) error {
	payload := []any{header, []byte{blobHeaderDelim}, b}
	hash, err := getHash(payload)
	if err != nil {
		return err
	}
	blobFile := filepath.Join(objectsDir, hash)
	return writeContents[any](blobFile, payload)
}

func readBlob(file string) (string, []byte, error) {
	var header string
	var contents []byte
	b, err := readContents(file)
	if err != nil {
		return header, contents, err
	}
	splitIdx := slices.Index(b, []byte("\n")[0])
	header = string(b[:splitIdx])
	contents = b[splitIdx+1:]
	return header, contents, nil
}
