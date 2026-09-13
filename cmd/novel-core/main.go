package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/chenhongyang/novel-studio/internal/core"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: novel-core <status|verify> [--project DIR]")
		return 2
	}
	switch args[0] {
	case "status":
		project, code := openProjectFromArgs("status", args[1:], stderr)
		if code != 0 {
			return code
		}
		status, err := project.Status()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := json.NewEncoder(stdout).Encode(status); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "verify":
		project, code := openProjectFromArgs("verify", args[1:], stderr)
		if code != 0 {
			return code
		}
		result, err := project.Verify()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if !result.OK {
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func openProjectFromArgs(name string, args []string, stderr io.Writer) (*core.Project, int) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectRoot := fs.String("project", ".", "local project root")
	if err := fs.Parse(args); err != nil {
		return nil, 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "%s: unexpected arguments: %v\n", name, fs.Args())
		return nil, 2
	}
	project, err := core.OpenProject(*projectRoot)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, 1
	}
	return project, 0
}
