/*
Gitlet provides a simple git-like version control system.
*/
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
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		fmt.Printf("Initialized new Gitlet repository in %v\n", filepath.Join(cwd, gitletDir))
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
		if err := printAllCommitIDsByMessage(query); err != nil {
			log.Fatal(err)
		}
	case "status":
		validateArgs(os.Args, 1)

		// staged for removal
		//  * look at currently tracked files that are not in working dir?

		currentBranchFile, err := readContentsAsString(headFile)
		if err != nil {
			log.Fatal(err)
		}
		currentBranch := filepath.Base(currentBranchFile)
		branches, err := getFilenames(branchHeadsDir)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("=== Branches ===")
		for _, b := range branches {
			if b == currentBranch {
				fmt.Print("*")
			}
			fmt.Println(b)
		}

		// staged
		index, err := readIndex()
		if err != nil {
			log.Fatal(err)
		}
		for k := range index {
			fmt.Println(k)
		}

		/*
			in WD only
			- Untracked

			in HEAD only
			- removed (not staged, not in WD)

			in Index and WD
			- staged   (same in WD)    == Index, WD, and Head
			- modified (changed in WD) == Index, WD, and Head
			- modified (deleted in WD) == in Index and Head, Index only

			in WD and Head
			- modified, unstaged (changed in WD)
		*/
		// var modified, untracked []string
		// filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		// 	if d.IsDir() {
		// 		if d.Name() == ".git" || d.Name() == ".gitlet" {
		// 			return fs.SkipDir
		// 		}
		// 		return nil
		// 	}

		// 	modified = append(modified, path)
		// 	return nil
		// })

		// for _, m := range modified {
		// 	fmt.Println(m)
		// }
		// fmt.Println(untracked)

		fmt.Println("\n=== Staged Files ===")

		fmt.Println("\n=== Removed Files ===")
		// files in head commit that are not in wd and not staged
		headCommit, err := getHeadCommit()
		if err != nil {
			log.Fatal(err)
		}

		for trackedFile := range headCommit.FileToBlob {
			_, isStaged := index[trackedFile]
			if !isStaged {
				fmt.Println(trackedFile)
			}
		}

		fmt.Println("\n=== Modifications Not Staged For Commit ===")
		// modified, not staged
		//  * tracked in current commit, changed in CWD, but not staged
		//  * staged for addition, different contents (hash) than CWD
		//  * staged for addition, deleted in CWD

		fmt.Println("\n=== Untracked Files ===")
		// files in wd that are not in latest commit

	case "checkout":
		// checkout -- filename
		// filename := os.Args[3]
		// commit_id, err := readContentsToString(".gitlet/HEAD")
		// if err != nil {
		// 	log.Fatal("No commit with that id exists.")
		// }

		// checkout commit_id -- filename
		// commit := os.Args[2]

		// checkout branch_name
		// commit, err := readContentsToString(fmt.Sprintf(".gitlet/refs/heads/%s", os.Args[2]))

		// targetBranchName := os.Args[2]
		// targetBranchPath := filepath.Join(".gitlet", "refs", "heads", targetBranchName)
		// currentBranch, err := readContentsToString(".gitlet/HEAD")
		// if err != nil {
		// 	log.Fatal(err)
		// }
		// if targetBranchName == currentBranch {
		// 	log.Fatal("No need to checkout the current branch.")
		// }
		// _, err = os.Stat(targetBranchPath)
		// if errors.Is(err, fs.ErrNotExist) {
		// 	log.Fatal("No such branch exists.")
		// }

		// var targetCommit commit
		// blob_id, missing := targetCommit.fileToBlob[filename]
		// if missing {
		// 	log.Fatal("File does nto exist in that commit.")
		// }

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
		// targetCommit := os.Args[2]

		// look for commit blob
		// readContentsAsString(filepath.Join(".gitlet", "objects", targetCommit))

		// checkout
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
