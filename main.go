// Command stopan works with a Stopan service desk through its JSON API:
// list, read, answer and change tickets from the shell or in a terminal
// dashboard.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"stopan-cli/internal/config"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

// env is where a command reads and writes, so tests can capture it.
type env struct {
	in       io.Reader
	out, err io.Writer
}

type action func(e env, args []string) error

// profile is the -P/--profile option, read by client, login and logout.
var profile string

type command struct {
	name     string
	aliases  []string
	synopsis string // what follows "stopan NAME" in the usage line
	summary  string // one line for the command list
	about    string // body of the command's help
	permute  bool   // options may follow positional arguments
	setup    func(f *flagDefs) action
}

func main() {
	os.Exit(run(os.Args[1:], env{os.Stdin, os.Stdout, os.Stderr}))
}

// run executes one invocation and returns the exit status: 0 success,
// 1 failure, 2 usage error.
func run(args []string, e env) int {
	profile = ""
	// -P NAME before the command applies to it, as after it.
	for len(args) > 0 && (args[0] == "-P" || args[0] == "--profile" || strings.HasPrefix(args[0], "--profile=")) {
		if v, ok := strings.CutPrefix(args[0], "--profile="); ok {
			profile, args = v, args[1:]
			continue
		}
		if len(args) < 2 {
			fmt.Fprintf(e.err, "stopan: %s needs a profile name\nRun 'stopan --help' for usage.\n", args[0])
			return 2
		}
		profile, args = args[1], args[2:]
	}
	if profile != "" {
		if err := config.ValidName(profile); err != nil {
			fmt.Fprintf(e.err, "stopan: %v\n", err)
			return 2
		}
	}
	if len(args) > 0 {
		switch args[0] {
		case "-h", "-help", "--help":
			writeUsage(e.out)
			return 0
		case "-V", "-version", "--version", "version":
			fmt.Fprintln(e.out, "stopan", version)
			return 0
		case "help":
			if len(args) == 1 {
				writeUsage(e.out)
				return 0
			}
			c, ok := lookup(args[1])
			if !ok {
				fmt.Fprintf(e.err, "stopan: unknown command %q\nRun 'stopan --help' for usage.\n", args[1])
				return 2
			}
			c.writeHelp(e.out)
			return 0
		}
	}

	// No command, or only options: the dashboard.
	name, rest, implicit := "tui", args, true
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, rest, implicit = args[0], args[1:], false
	}
	c, ok := lookup(name)
	if !ok {
		fmt.Fprintf(e.err, "stopan: unknown command %q\nRun 'stopan --help' for usage.\n", name)
		return 2
	}
	f := newFlagDefs(c.name)
	act := c.setup(f)
	prof := addProfileOption(f)
	pos, err := f.parse(rest, c.permute)
	if err == nil && *prof != "" {
		if verr := config.ValidName(*prof); verr != nil {
			err = usageError(verr.Error())
		}
		profile = *prof
	}
	if err == nil {
		err = act(e, pos)
	}

	var ue usageError
	switch {
	case err == nil:
		return 0
	case errors.Is(err, flag.ErrHelp):
		c.writeHelp(e.out)
		return 0
	case errors.As(err, &ue):
		prog, hint := "stopan "+c.name, "stopan "+c.name+" --help"
		if implicit {
			prog, hint = "stopan", "stopan --help"
		}
		fmt.Fprintf(e.err, "%s: %s\nRun '%s' for usage.\n", prog, ue, hint)
		return 2
	default:
		fmt.Fprintln(e.err, "stopan:", err)
		return 1
	}
}

func lookup(name string) (command, bool) {
	for _, c := range commands {
		if c.name == name {
			return c, true
		}
		for _, a := range c.aliases {
			if a == name {
				return c, true
			}
		}
	}
	return command{}, false
}

func writeUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: stopan [COMMAND] [OPTIONS] [ARGS...]

Work with a Stopan service desk from the terminal: list, read, answer and
change tickets, and read notifications, through the server's JSON API
with a personal token. Without a COMMAND, starts the interactive
dashboard.

Commands:
`)
	for _, c := range commands {
		fmt.Fprintf(w, "  %-13s %s\n", c.name, c.summary)
	}
	fmt.Fprintf(w, `  %-13s show help for a command
  %-13s print the version

Options:
  -P, --profile NAME  use this profile (also after the command)
  -h, --help          show this help and exit
  -V, --version       print the version and exit

TICKET is a ticket number, with or without '#'.

VIEW (for list) is unsolved (the default), mine (assigned to you),
unassigned, pending, postponed, done or all; requesters have open, done
and all. Example: 'stopan list mine'.

Start with 'stopan login': make a token on the account page of the
Stopan site ("API tokens") and paste it when asked. Several servers or
accounts are profiles: 'stopan login -P dev', 'stopan profiles',
'stopan use dev', 'stopan -P dev list'. STOPAN_PROFILE chooses a
profile; STOPAN_URL and STOPAN_TOKEN override its server and token.

Exit status: 0 on success, 1 on failure, 2 on a usage error.

Run 'stopan help COMMAND' or 'stopan COMMAND --help' for details.
`, "help", "version")
}

func (c command) writeHelp(w io.Writer) {
	name := c.name
	if c.name == "tui" {
		name = "[tui]"
	}
	fmt.Fprintf(w, "Usage: stopan %s %s\n\n%s\n\n", name, c.synopsis, strings.TrimSpace(c.about))
	if len(c.aliases) > 0 {
		fmt.Fprintf(w, "Aliases: %s\n\n", strings.Join(c.aliases, ", "))
	}
	f := newFlagDefs(c.name)
	c.setup(f)
	addProfileOption(f)
	f.writeOptions(w)
}

// addProfileOption gives every command the -P/--profile option.
func addProfileOption(f *flagDefs) *string {
	return f.String("P", "profile", "NAME", "", "use this profile (see 'stopan profiles')")
}
