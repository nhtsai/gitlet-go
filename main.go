// Package main provides a simple git-like version control system called 'Gitlet'.
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	log.SetFlags(log.Lshortfile)
	if len(os.Args) == 1 {
		log.Fatal("Please enter a command.")
	}

	var commandArg string = os.Args[1]
	if commandArg != "init" {
		checkGitletInit()
	}

	switch commandArg {
	case "init":
		validateArgs(os.Args, 1)
		if err := initRepository(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Gitlet repository initialized (on branch 'main').")
	case "add":
		validateArgs(os.Args, 2)
		file := os.Args[2]
		if err := stageFile(file); err != nil {
			log.Fatal(err)
		}
	case "commit":
		validateArgs(os.Args, 2)
		message := os.Args[2]
		commit, err := createCommit(message)
		if err != nil {
			log.Fatal(err)
		}
		err = commit.writeBlob()
		if err != nil {
			log.Fatal(err)
		}
	case "rm":
		validateArgs(os.Args, 2)
		file := os.Args[2]
		if err := unstageFile(file); err != nil {
			log.Fatal(err)
		}
	case "log":
		validateArgs(os.Args, 1)
		if err := printBranchLog(); err != nil {
			log.Fatal(err)
		}
	case "global-log":
		validateArgs(os.Args, 1)
		if err := printAllCommits(); err != nil {
			log.Fatal(err)
		}
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
		validateArgs(os.Args, 2)
		branchName := os.Args[2]
		err := addBranch(branchName)
		if err != nil {
			log.Fatal("Could not create new branch: ", err)
		}

	case "rm-branch":
		validateArgs(os.Args, 2)
		branchName := os.Args[2]
		err := removeBranch(branchName)
		if err != nil {
			log.Fatal("Could not remove branch: ", err)
		}

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
