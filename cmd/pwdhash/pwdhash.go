package main

import (
	"fmt"
	"os"

	"github.com/cyclopcam/cyclops/pkg/pwdhash"
)

// Takes a password as the first argument, and prints out a base64 encoded version of the hashed password.
// You can use this to generate a password for a user in a database, if you need to do that manually.
// For example:
// sqlite3 config.sqlite "insert into user (username, username_normalized, permissions, name, password, created_at) values ('admin', 'admin', 'a', 'Samuel Taylor', 'HASHEDPASSWORD', CAST((julianday('now') - 2440587.5)*86400000 AS INTEGER))"

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: pwdhash <password>\n")
		os.Exit(1)
	}
	password := os.Args[1]
	fmt.Printf("%v\n", pwdhash.HashPasswordBase64(password))
}
