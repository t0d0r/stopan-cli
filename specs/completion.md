# Shell completion

```
stopan completion SHELL      # SHELL: bash, zsh or fish
```

Prints a completion script on standard output. Any other SHELL, or no
or several arguments, is a usage error (exit 2).

## Generated from the command table

The scripts are built from the same command table and option
definitions as the help (completion.go), so a new command or option is
completed without further work. The `completion` command itself is added
to the table in `init()`, since its output is made from the table.

## What is completed

| where | values |
|---|---|
| the first word | the commands (`help` included); zsh and fish show each command's summary |
| `-<TAB>` before a command | `-P --profile -h --help -V --version` |
| `-<TAB>` after a command | that command's options, `-P/--profile` and `-h/--help` |
| `-P`, `--profile` (anywhere) | the saved profile names, from `stopan profiles --names` at completion time |
| `-v`, `--view` | unsolved, mine, unassigned, pending, postponed, done, all, open |
| `-s`, `--status` | new, open, pending, postponed, done |
| `-p`, `--priority` | low, normal, high, urgent |
| `--sort` | updated, created, priority |
| `-a`, `--assignee` | me, none |
| other options with an argument (`--url`, `--search`, `--page`, ...) | nothing (free text) |
| first argument of `list` | the views |
| first argument of `use` | the profile names |
| first argument of `completion` | bash, zsh, fish |
| first argument of `help` | the commands |

Options are found per command, so `-a` is `--assignee` for `list`,
`new` and `set` but `--all` (no value) for `read`. Ticket numbers are
not completed: that would need a request to the server on every TAB.

`stopan profiles --names` (`-n`) prints the profile names one a line,
sorted, for the scripts and for other scripts; it never prints tokens.

## Installation

| installed by | where |
|---|---|
| Homebrew | `generate_completions_from_executable(bin/"stopan", "completion")` in the formula: brew's bash, zsh and fish directories |
| .deb | `/usr/share/bash-completion/completions/stopan`, `/usr/share/zsh/vendor-completions/_stopan`, `/usr/share/fish/vendor_completions.d/stopan.fish` |
| tarball | `completions/stopan.bash`, `completions/_stopan`, `completions/stopan.fish` next to the binary |
| by hand | `stopan help completion` and README "Shell completion" |

The zsh script works both from `fpath` (autoloaded `_stopan`) and when
sourced (it then calls `compdef`).

## Decisions

- Scripts written by stopan, not a completion framework. Reason: no
  dependency, the same command table as the help, and three small
  scripts that can be read.
- Profile names are read live; nothing about them is baked into the
  scripts, so they stay right after login, use and logout.

## Acceptance

- completion_test.go: every command and option appears in each script;
  unknown and missing shells exit 2; bash and fish are driven with real
  command lines against a built binary with two profiles (commands,
  prefixes, views as the first list word, `-v`, `-s`, `-p`, `-a`
  values, `-P` and `--profile` before and after the command, `use`,
  `read -`, `completion`, `help`); `zsh -n` checks the zsh script.
- By hand: zsh in tmux with compinit: commands with descriptions,
  `list -v p`, `use`, `re`.
