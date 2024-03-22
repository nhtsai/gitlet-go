/*
Gitlet provides a simple git-like version control system.
*/
package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(0)
	if len(os.Args) == 1 {
		log.Fatal("Please enter a command.")
	}

	command := os.Args[1]
	if command != "init" {
		checkGitletInit()
	}

	switch command {
	case "init":
		validateArgs(os.Args, 1)
		if err := newRepository(); err != nil {
			log.Fatal(err)
		}
		if cwd, err := os.Getwd(); err != nil {
			log.Println("Initialized new Gitlet repository.")
		} else {
			log.Printf("Initialized new Gitlet repository in %v\n", filepath.Join(cwd, gitletDir))
		}
	case "add":
		validateArgs(os.Args, 2)
		file := os.Args[2]
		if err := stageFile(file); err != nil {
			log.Fatal(err)
		}
	case "commit":
		validateArgs(os.Args, 2)
		message := os.Args[2]
		if err := newCommit(message); err != nil {
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
		validateArgs(os.Args, 2)
		query := os.Args[2]
		if err := printMatchingCommits(query); err != nil {
			log.Fatal(err)
		}
	case "status":
		validateArgs(os.Args, 1)
		if err := printStatus(); err != nil {
			log.Fatal(err)
		}
	case "checkout":
		if (len(os.Args) == 4) && os.Args[2] == "--" {
			file := os.Args[3]
			if err := checkoutHeadCommit(file); err != nil {
				log.Fatal(err)
			}
		} else if (len(os.Args) == 5) && os.Args[3] == "--" {
			commitUID := os.Args[2]
			file := os.Args[4]
			if err := checkoutCommit(file, commitUID); err != nil {
				log.Fatal(err)
			}
		} else if len(os.Args) == 3 {
			branchName := os.Args[2]
			if err := checkoutBranch(branchName); err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal("Incorrect operands.")
		}
	case "branch":
		validateArgs(os.Args, 2)
		branchName := os.Args[2]
		if err := addBranch(branchName); err != nil {
			log.Fatal("Could not create new branch: ", err)
		}
	case "rm-branch":
		validateArgs(os.Args, 2)
		branchName := os.Args[2]
		if err := removeBranch(branchName); err != nil {
			log.Fatal("Could not remove branch: ", err)
		}
	case "reset":
		validateArgs(os.Args, 2)
		commitUID := os.Args[2]
		if err := resetFile(commitUID); err != nil {
			log.Fatal(err)
		}
	case "merge":
		validateArgs(os.Args, 2)
		branchName := os.Args[2]
		if err := mergeBranch(branchName); err != nil {
			log.Fatal(err)
		}
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
	_, err := os.Stat(gitletDir)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatal("Not in an initialized Gitlet directory.")
	}
}
