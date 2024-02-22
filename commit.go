package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"time"
)

type commit struct {
	// The SHA1 hash of commit and header metadata.
	UID string

	// User supplied commit message.
	Message string

	// When the commit was created in UNIX time in UTC.
	Timestamp int64

	// Mapping from file names to blob UIDs tracked in the commit.
	FileToBlob map[string]string

	// SHA1 hash of the parent commit. Merge commits should have two parents.
	ParentUIDs [2]string
}

func (c *commit) String() string {
	if c.ParentUIDs[1] != "" {
		return fmt.Sprintf(
			"commit %v\n"+
				"Merge: %v %v\n"+
				"Date: %v\n"+
				"%v\n"+
				"Merged %v into %v.\n",
			c.UID,
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
			c.UID,
			time.Unix(c.Timestamp, 0).Local().Format("Mon Jan 02 15:04:05 2006 -0700"),
			c.Message,
		)
	}
}

func createCommit(message string) (commit, error) {
	var c commit
	index, err := readIndex()
	if err != nil {
		return c, err
	}
	// create commit
	c.UID = "" // hash of all staged blobs?
	c.Message = message
	c.Timestamp = time.Now().UTC().Unix()

	c.FileToBlob = make(map[string]string)
	for k, v := range index {
		c.FileToBlob[k] = v.Hash
	}

	// get current head commit hash
	currentBranchFile, err := readContentsToString(filepath.Join(".gitlet", "HEAD"))
	if err != nil {
		return c, err
	}
	headCommitHash, err := readContentsToString(currentBranchFile)
	if err != nil {
		return c, err
	}
	c.ParentUIDs = [2]string{headCommitHash}

	return c, nil
}

// Return a commit given a hash.
func getCommit(hash string) (commit, error) {
	var c commit
	commitData, err := readContentsToBytes(filepath.Join(".gitlet", "objects", hash))
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
	currentBranchFile, err := readContentsToString(filepath.Join(".gitlet", "HEAD"))
	if err != nil {
		return c, err
	}
	headCommitHash, err := readContentsToString(currentBranchFile)
	if err != nil {
		return c, err
	}
	c, err = getCommit(headCommitHash)
	if err != nil {
		return c, err
	}
	return c, nil
}

func (c *commit) writeBlob() error {
	b, err := serialize[*commit](c)
	if err != nil {
		return fmt.Errorf("commit: writeBlob: %w", err)
	}
	commitFile := filepath.Join(".gitlet", "objects", c.UID)
	err = writeContents[any](commitFile, []any{b})
	if err != nil {
		return fmt.Errorf("commit: writeBlob: %w", err)
	}
	return nil
}

// print all commits
func printAllCommits() error {
	return filepath.WalkDir(
		filepath.Join(".gitlet", "objects"),
		func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() {
				return nil
			}
			c, c_err := getCommit(d.Name())
			if c_err != nil {
				return c_err
			}
			fmt.Printf("===\n%v\n", c.String())
			return err
		},
	)
}
