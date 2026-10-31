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

## Release v0.3.0

- make check passes; README install examples name v0.3.0; annotated tag
  v0.3.0; make dist and make deb VERSION=v0.3.0 on the tag (tarballs
  with completions/); gitleaks: no leaks. Pushed main and the tag.
- GitHub release v0.3.0 with four tarballs, two .deb files and
  SHA256SUMS; all six pass the checksum check after download.
- Homebrew: formula 0.3.0 in t0d0r/homebrew-tap with
  generate_completions_from_executable (source archive sha256
  abc763343d87f34afeafed24b94767b68c56d7f7e4cce73bb52b887c4622976b);
  brew style and audit --strict pass; pushed. `stopan update -y` on
  macOS ran brew upgrade 0.2.0 -> 0.3.0; brew test passes (it checks the
  bash script); brew linked etc/bash_completion.d/stopan,
  share/zsh/site-functions/_stopan and
  share/fish/vendor_completions.d/stopan.fish.
- A fresh fish completes `stopan use <TAB>` with the saved profile,
  `stopan list -v p` with pending and postponed, and `stopan li` with
  list, from the brew-installed script.
- The open item above (formula without completions) is closed.

Next step: use stopan against the production server with a read-only
token; then the next feature from todo/todo.md Later (attachments).
