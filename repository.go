package main

import (
	"log"
	"os"
	"path"
)

type repository struct {
	cwd                string
	gitlet_dir         string
	branchHeadToCommit map[string]string
}

func newRepository() repository {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	gitlet_dir := path.Join(cwd, ".gitlet")
	repo := repository{cwd, gitlet_dir, make(map[string]string)}
	return repo
}

func (r *repository) addCommit(c commit) error {
	return nil
}
