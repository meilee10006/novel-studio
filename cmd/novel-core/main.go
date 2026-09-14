package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chenhongyang/novel-studio/internal/core"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: novel-core <init|serve|status|verify|export|restore|migrate> [options]")
		return 2
	}
	switch args[0] {
	case "init":
		fs := flag.NewFlagSet("init", flag.ContinueOnError)
		fs.SetOutput(stderr)
		projectRoot := fs.String("project", "", "local authoritative project root")
		workspaceRoot := fs.String("workspace", "", "Drive workspace root")
		projectID := fs.String("project-id", "", "stable project id")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
			return 2
		}
		project, err := core.InitProject(core.InitOptions{ProjectID: *projectID, LocalRoot: *projectRoot, WorkspaceRoot: *workspaceRoot})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
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
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ContinueOnError)
		fs.SetOutput(stderr)
		projectRoot := fs.String("project", ".", "local project root")
		scanInterval := fs.Duration("scan-interval", 2*time.Second, "low-frequency correctness scan interval")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
			return 2
		}
		project, err := core.OpenProject(*projectRoot)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := project.Serve(ctx, *scanInterval); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
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
	case "migrate":
		fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
		fs.SetOutput(stderr)
		projectRoot := fs.String("project", ".", "local project root")
		backup := fs.String("backup", "", "pre-migration backup directory")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
			return 2
		}
		project, err := core.OpenProject(*projectRoot)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		result, err := project.Migrate(*backup)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "restore":
		fs := flag.NewFlagSet("restore", flag.ContinueOnError)
		fs.SetOutput(stderr)
		backup := fs.String("backup", "", "verified backup directory")
		projectRoot := fs.String("project", "", "new local project root")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
			return 2
		}
		project, err := core.RestoreBackup(*backup, *projectRoot)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
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
	case "export":
		fs := flag.NewFlagSet("export", flag.ContinueOnError)
		fs.SetOutput(stderr)
		projectRoot := fs.String("project", ".", "local project root")
		out := fs.String("out", "", "output manuscript path")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
			return 2
		}
		project, err := core.OpenProject(*projectRoot)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		result, err := project.ExportBook(*out)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintln(stderr, err)
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
