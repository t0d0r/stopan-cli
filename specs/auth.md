# Server and token

## Token

A personal API token made on the Stopan site (account page, "API
tokens"; ../stopan/specs/api.md "Tokens"). The API cannot make tokens.
A read-only token is enough for list, show, notifications and tokens;
every write needs a read-and-write token, otherwise the server answers
403 `read_only`, which stopan shows as is.

Every request sends `Authorization: Bearer TOKEN`, `Accept:
application/json` and `User-Agent: stopan-cli/VERSION`. The token is
never put in a URL.

## Config file

`$XDG_CONFIG_HOME/stopan/config.json`, default
`~/.config/stopan/config.json` (also on macOS):

```json
{
  "url": "https://stopan.example.org",
  "token": "stopan_..."
}
```

- Directory mode 0700, file mode 0600; written to a temporary file in the
  same directory and renamed, so it is never half written.
- A file that group or others can read is refused with
  `... run: chmod 600 PATH`, since it holds a token.
- `STOPAN_URL` and `STOPAN_TOKEN` override the file for one run (CI,
  scripts, a second server). They are never written to the file.

## Server URL

`host`, `https://host`, `https://host/` and `https://host/api/v1` all
mean `https://host`; a path prefix is kept (`https://host/desk`).

- Only `https`. Plain `http` is accepted only for `localhost` and
  loopback addresses (development with `bin/dev` of the server).
- No user, password, query or fragment in the URL.
- Redirects are followed only to the same host, and never from https
  down to http, so the token does not leave the server.

## login

```
stopan login [--url URL] [--token-stdin]
```

1. URL: `--url`, else asked (`Server URL [DEFAULT]:`), where DEFAULT is
   the saved URL or `https://stopan.linuxfan.org`. With `--token-stdin`
   and no `--url`, DEFAULT is used without asking.
2. Token: read without echo from the terminal (`Token (not shown):`),
   or the first line of standard input with `--token-stdin` (or when
   standard input is not a terminal). Must be one word.
3. `GET /api/v1/me` checks it; on failure nothing is saved and the
   server's error is shown (`unauthorized: ...`).
4. Saved; prints `logged in to URL as NAME (ROLE); saved in PATH`.

Decision: no `--token` option. Reason: a token on the command line ends
up in the shell history and in `ps` output.

## logout

```
stopan logout [--revoke]
```

Removes the token from the file and keeps the URL. With `--revoke` it
first finds the token in use among `GET /api/v1/tokens` by its last four
characters (`hint`) and revokes it with `DELETE /api/v1/tokens/:id`;
when the hint matches no token or more than one, it stops and says to use
`stopan revoke ID`. A set `STOPAN_TOKEN` is mentioned, since it still
works.

## tokens, revoke

`stopan tokens` lists the user's tokens (id, name, `...hint`, access,
created, last used, expires) and marks the one in use `(this one)`.
`stopan revoke ID...` revokes tokens by id.

## Acceptance

- Unit tests: URL normalization (https only, localhost http, no
  credentials), the Bearer header and no query string, error decoding
  (401, 422 with details, 429 with Retry-After, non-JSON 502), redirect
  to another host refused; config save/load with modes 0600/0700,
  environment override, a world-readable file refused.
- CLI test: login with `--token-stdin` against a fake server saves the
  file; a wrong token saves nothing; plain http to a remote host is
  refused; `logout --revoke` revokes and removes the token but keeps the
  URL.

## Security Note

The token is as strong as its user's password. Keep it in the 0600 file
or a secret store feeding `STOPAN_TOKEN`; prefer a read-only token and an
expiry for scripts. A token that may have leaked is revoked with
`stopan revoke ID` or on the account page.
