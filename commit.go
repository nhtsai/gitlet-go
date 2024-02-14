package main

import (
	"time"
)

/**
* TODO: add instance variables here.
*
* List all instance variables of the Commit class here with a useful
* comment above them describing what that variable represents and how that
* variable is used. We've provided one example for `message`.
 */
type commit struct {
	message    string
	timestamp  time.Time
	fileToBlob map[string]string
	parent1    string
	parent2    string
}
