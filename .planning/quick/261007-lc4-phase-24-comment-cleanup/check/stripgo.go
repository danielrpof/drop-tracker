// Command stripgo prints a Go file's canonical form with every comment
// dropped, so two versions that differ only in comments print identically.
// Usage: go run stripgo.go - < file.go (or a non-.go path argument).
package main

import (
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: stripgo <file|->")
		os.Exit(2)
	}
	var src []byte
	var err error
	if os.Args[1] == "-" {
		src, err = io.ReadAll(os.Stdin)
	} else {
		src, err = os.ReadFile(os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, os.Args[1], src, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := format.Node(os.Stdout, fset, f); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
