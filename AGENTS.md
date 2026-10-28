Follow the rules in ../AGENTS.md.

Project notes:

- Go module `stopan-cli`, entry point `main.go`, the command table in
  `commands.go`, packages under `internal/` (api, config, text, tui,
  update). The binary is `stopan`.
- The server and its API spec are in ../stopan (specs/api.md). When the
  API changes there, update internal/api and specs/cli.md here.
- `make check` before every commit; `make build`, `make run`, `make help`
  for the rest. Tests use a fake API server and need no network.
- Tests and docs use invented names, `example.org` hosts and fake tokens
  such as `stopan_test`; never a real token or server data.
