package tokenscli

import (
	"fmt"
	"os"

	"github.com/eduard256/claudecode2openaiapi/pkg/isolation"
	"github.com/eduard256/claudecode2openaiapi/pkg/tokens"
)

// Run dispatches `tokens <subcommand> [args]`.
func Run(args []string) {
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	store := tokens.NewStore(isolation.TokensFile)
	if err := store.Load(); err != nil {
		fail("load tokens: " + err.Error())
	}

	switch args[0] {
	case "add":
		if len(args) < 2 {
			fail("usage: tokens add <name>")
		}
		t, err := store.Add(args[1])
		if err != nil {
			fail(err.Error())
		}
		fmt.Println(t.Token)

	case "list":
		ts := store.List()
		if len(ts) == 0 {
			fmt.Println("(no tokens)")
			return
		}
		for _, t := range ts {
			fmt.Printf("%-20s  %s  %s\n", t.Name, t.Token, t.CreatedAt.Format("2006-01-02 15:04:05"))
		}

	case "rm", "remove":
		if len(args) < 2 {
			fail("usage: tokens rm <name>")
		}
		if err := store.Remove(args[1]); err != nil {
			fail(err.Error())
		}

	case "show":
		if len(args) < 2 {
			fail("usage: tokens show <name>")
		}
		for _, t := range store.List() {
			if t.Name == args[1] {
				fmt.Println(t.Token)
				return
			}
		}
		fail("not found: " + args[1])

	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: claudecode2openaiapi tokens <add|list|rm|show> [args]")
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "error: "+msg)
	os.Exit(1)
}
