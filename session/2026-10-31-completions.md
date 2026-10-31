# 2026-10-31 - shell completions

## Done

- `stopan completion bash|zsh|fish` prints a completion script
  generated from the command table (specs/completion.md): commands with
  summaries (zsh, fish), per-command options, values of --view,
  --status, --priority, --sort, --assignee, the view as the first word
  of list, profile names for -P/--profile and use (read live from the new
  `stopan profiles --names`), shells for completion, commands for help.
- Packaging: the tarballs carry completions/ (stopan.bash, _stopan,
  stopan.fish); the .deb installs them under /usr/share/bash-completion,
  /usr/share/zsh/vendor-completions and /usr/share/fish/
  vendor_completions.d; the formula source calls
  generate_completions_from_executable and its test checks the bash
  script.
- Checked: completion_test.go drives bash and fish with real command
  lines against a built binary with two profiles; zsh -n; zsh by hand in
  tmux with compinit; make dist and make deb with a test version: the
  tarball lists completions/, the .deb installs the three files, and in
  debian:latest with bash-completion `complete -p stopan` shows
  `_stopan`.
- README: a "Why Stopan?" section after the introduction (the name
  from Bulgarian folklore), and a "Shell completion" section.

## Decisions

- Own small scripts instead of a completion framework; profile names
  read live, never baked into a script.
- Ticket numbers are not completed (a server request on every TAB).

## Open

- The formula in t0d0r/homebrew-tap stays at 0.2.0 without completions:
  0.2.0 has no completion command, so generate_completions_from_executable
  would fail there. It goes to the tap with v0.3.0.

## Files

completion.go, completion_test.go, commands.go (profiles --names),
Makefile (dist, deb), packaging/homebrew/stopan-cli.rb, README.md,
specs/{completion,cli,release,overview,README}.md, todo/todo.md.

## Run and verify

`make check`; `stopan completion fish | source` in fish, then
`stopan list <TAB>`; `make dist VERSION=v0.0.0-test` and `tar -tzf`.

## Next step

Release v0.3.0 (specs/release.md), with the formula's completions, and
check `brew install` puts the three scripts in place.
