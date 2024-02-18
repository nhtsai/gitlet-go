// Package main provides a simple git-like version control system called 'Gitlet'.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"time"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) == 1 {
		log.Fatal("Please enter a command.")
	}

	var commandArg string = os.Args[1]

	switch commandArg {
	case "init":
		validateArgs(os.Args, 1)
		if err := initRepository(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Gitlet repository initialized (on branch 'main').")

		// create initial commit
		initialCommit := commit{
			message:    "initial commit",
			timestamp:  time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC),
			fileToBlob: make(map[string]string),
			parent1:    "",
			parent2:    "",
		}
		err = repo.addCommit(initialCommit)
		if err != nil {
			log.Fatal(err)
		}

	case "add":
		checkGitletInit()
	case "commit":
		checkGitletInit()
	case "log":
		checkGitletInit()
		validateArgs(os.Args, 1)
	case "global-log":
		checkGitletInit()
		validateArgs(os.Args, 1)
	case "find":
		checkGitletInit()
	case "status":
		checkGitletInit()
		validateArgs(os.Args, 1)
		fmt.Println("=== Branches ===")
		// mark current branch with *

		fmt.Println("=== Staged Files ===")

		fmt.Println("=== Removed Files ===")

		fmt.Println("=== Modifications Not Staged For Commit ===")

		fmt.Println("=== Untracked Files ===")
	case "checkout":
		checkGitletInit()
	case "branch":
		checkGitletInit()
	case "rm-branch":
		checkGitletInit()
	case "reset":
		checkGitletInit()
	case "merge":
		checkGitletInit()
	default:
		log.Fatal("No command with that name exists.")
	}
}

func validateArgs(args []string, expected int) {
	if len(args)-1 != expected {
		log.Fatal("Incorrect operands.")
	}
}

func checkGitletInit() {
	_, err := os.Stat(".gitlet")
	if errors.Is(err, os.ErrNotExist) {
		log.Fatal("Not in an initialized Gitlet directory.")
	}
}
