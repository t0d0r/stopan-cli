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

## Config file and profiles

`$XDG_CONFIG_HOME/stopan/config.json`, default
`~/.config/stopan/config.json` (also on macOS). It holds named
profiles, each a server and a token, and the current profile:

```json
{
  "current": "prod",
  "profiles": {
    "prod": {"url": "https://stopan.example.org", "token": "stopan_...", "user": "Ben (agent)"},
    "dev":  {"url": "http://localhost:3000", "token": "stopan_..."}
  }
}
```

- A profile name is 1 to 32 letters, digits, `.`, `_` or `-`, starting
  with a letter or digit.
- `user` is `NAME (ROLE)` from the last login, only for `stopan
  profiles`.
- The v0.1.0 file (`{"url": ..., "token": ...}`) is read as the profile
  `default`, which is current; it is rewritten in the new form at the
  next save.
- Directory mode 0700, file mode 0600; written to a temporary file in the
  same directory and renamed, so it is never half written.
- A file that group or others can read is refused with
  `... run: chmod 600 PATH`, since it holds tokens.

## Profiles

Which profile a command uses, first match wins:

1. `-P/--profile NAME` (cli.md);
2. `STOPAN_PROFILE`;
3. the current profile;
4. the only profile, when there is one;
5. `default`.

`STOPAN_URL` and `STOPAN_TOKEN` then override the chosen profile's
server and token for one run (CI, scripts); they are never written to
the file. A profile that does not exist fails with `no profile "NAME";
run 'stopan login -P NAME' or 'stopan profiles'` unless both variables
are set; with no profiles at all, `not logged in; run 'stopan login'`.
A profile without a token fails with `profile NAME: no token; run
'stopan login'`.

- `stopan profiles`: a table `PROFILE SERVER USER TOKEN` (`saved` or
  `none`) with `*` on the profile used when none is named; a note when
  `STOPAN_PROFILE` chooses.
- `stopan use NAME`: makes NAME current; NAME must exist. A note when
  `STOPAN_PROFILE` is set to another name, since it still wins.
- The dashboard names the profile in its header (tui.md).

Decision: profiles in the one config file, not one file per server.
Reason: one place to list, switch and protect with 0600.

Decision: `login` makes its profile current. Reason: logging in to a
server is almost always followed by using it; `use` switches back.

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
stopan login [-P NAME] [--url URL] [--token-stdin]
```

The profile is the one chosen as above (`-P`, `STOPAN_PROFILE`, the
current one, the only one, `default`).

1. URL: `--url`, else asked (`Server URL [DEFAULT]:`), where DEFAULT is
   the profile's saved URL or `https://stopan.linuxfan.org`. With
   `--token-stdin` and no `--url`, DEFAULT is used without asking.
2. Token: read without echo from the terminal (`Token (not shown):`),
   or the first line of standard input with `--token-stdin` (or when
   standard input is not a terminal). Must be one word.
3. `GET /api/v1/me` checks it; on failure nothing is saved and the
   server's error is shown (`unauthorized: ...`).
4. The profile is saved (replacing its URL, token and user) and made
   current; prints `logged in to URL as NAME (ROLE); profile P, saved in
   PATH`.

Decision: no `--token` option. Reason: a token on the command line ends
up in the shell history and in `ps` output.

## logout

```
stopan logout [-P NAME] [--revoke] [--forget]
```

Works on the chosen profile, which must exist. Without options it
removes the profile's token and keeps its URL (`removed the token of
profile P`), so the next login asks only for the token. `--forget`
removes the whole profile (`removed profile P`); when it was current,
the first remaining profile by name becomes current and is named. With
`--revoke` it first finds the token in use among `GET /api/v1/tokens`
by its last four characters (`hint`) and revokes it with `DELETE
/api/v1/tokens/:id`; when the hint matches no token or more than one, it
stops and says to use `stopan revoke ID`. A set `STOPAN_TOKEN` is
mentioned, since it still works.

## tokens, revoke

`stopan tokens` lists the user's tokens (id, name, `...hint`, access,
created, last used, expires) and marks the one in use `(this one)`.
`stopan revoke ID...` revokes tokens by id.

## Acceptance

- Unit tests: profiles saved and read back with modes 0600/0700, the
  order of choice (-P, STOPAN_PROFILE, current), environment overrides,
  a missing profile, the v0.1.0 file read as `default` and saved in the
  new form, removing the current profile, profile names.
- CLI test (profiles): login to two fake servers as `prod` and `dev`,
  `profiles` table, `-P` before and after the command and
  `--profile=`, `use`, `STOPAN_PROFILE`, missing and bad names (exit 1
  and 2), logout of one profile, `logout --forget` moving current.
- Unit tests: URL normalization (https only, localhost http, no
  credentials), the Bearer header and no query string, error decoding
  (401, 422 with details, 429 with Retry-After, non-JSON 502), redirect
  to another host refused; a world-readable config file refused.
- CLI test: login with `--token-stdin` against a fake server saves the
  file; a wrong token saves nothing; plain http to a remote host is
  refused; `logout --revoke` revokes and removes the token but keeps the
  URL.

## Security Note

The token is as strong as its user's password. Keep it in the 0600 file
or a secret store feeding `STOPAN_TOKEN`; prefer a read-only token and an
expiry for scripts. A token that may have leaked is revoked with
`stopan revoke ID` or on the account page.
