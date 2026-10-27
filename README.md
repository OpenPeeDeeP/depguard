# Depguard

A Go linter that checks package imports are in a list of acceptable packages.
This allows you to allow imports from a whole organization or only
allow specific packages within a repository. 

## Install

```bash
go install github.com/OpenPeeDeeP/depguard/v2/cmd/depguard@latest
```

## Config

The Depguard binary looks for a file named `^\.?depguard\.(yaml|yml|json|toml)$` in the current working directory. Examples include (`.depguard.yml` or `depguard.toml`).

The following is an example configuration file.

```json
{
  "main": {
    "files": [
      "$all",
      "!$test"
    ],
    "listMode": "Strict",
    "allow": [
      "$gostd",
      "github.com/OpenPeeDeeP"
    ],
    "deny": {
      "reflect": "Who needs reflection",
    }
  },
  "tests": {
    "files": [
      "$test"
    ],
    "listMode": "Lax",
    "deny": {
      "github.com/stretchr/testify": "Please use standard library for tests"
    }
  }
}
```

- The top level is a map of lists. The key of the map is a name that shows up in 
the linter's output.
- `files` - list of file globs that will match this list of settings to compare against
- `allow` - list of allowed packages
- `deny` - map of packages that are not allowed where the value is a suggestion
- `listMode` - the mode to use for package matching

Files are matched using [Globs](https://github.com/gobwas/glob). If the files 
list is empty, then all files will match that list. Prefixing a file
with an exclamation mark `!` will put that glob in a "don't match" list. A file
will match a list if it is allowed and not denied.

> Should always prefix a file glob with `**/` as files are matched against absolute paths.

`**` matches any number of characters, including `/`, but it doesn't make the
slashes around it optional. So `**/internal/**/*.go` matches
`internal/foo/a.go` but not `internal/a.go`: there has to be at least one folder
between `internal/` and the file. To match zero or more folders, put the slash
inside a brace group with an empty option: `**/internal/{**/,}*.go`.

| Glob | `/project/internal/a.go` | `/project/internal/foo/a.go` |
|---|---|---|
| `**/internal/**/*.go` | no | yes |
| `**/internal/{**/,}*.go` | yes | yes |
| `**/internal/{**,}/*.go` | no | yes |

The last one doesn't work because the empty option leaves `internal//*.go`.

Allow is a list of packages to allow. Deny is a map where the key is a package
to deny, and the value is a suggestion on what to use instead (it can be empty).

ListMode is used to determine the package matching priority. There are three
different modes; Original, Strict, and Lax.

Original is the original way that the package was written to use. It is not recommended
to stay with this and is only here for backwards compatibility.

Strict, at its roots, is everything is denied unless in allowed.

Lax, at its roots, is everything is allowed unless it is denied.

### How Package Matching Works

Every `allow` and `deny` entry is a **prefix**: it matches an import if the
import path starts with it. This is a plain string comparison, it does not look
at path segments. End an entry with a dollar sign `$` to match that exact
package only. Matching is case-sensitive, like Go import paths, so use the
package's exact casing (`github.com/OpenPeeDeeP/`, not `github.com/openpeedeep/`).

| Entry | Matches | Does not match |
|---|---|---|
| `github.com/bob` | `github.com/bob`, `github.com/bob/pkg`, `github.com/bob2/pkg` | |
| `github.com/bob/` | `github.com/bob/pkg` | `github.com/bob`, `github.com/bob2/pkg` |
| `github.com/bob$` | `github.com/bob` | `github.com/bob/pkg`, `github.com/bob2/pkg` |

Some things to keep in mind:

- To match a package and all of its subpackages, but nothing else, use two
entries: `github.com/bob$` and `github.com/bob/`.
- A new major version shares the old version's prefix. `math/rand` also matches
`math/rand/v2`, so deny `math/rand$` to only deny the old version.
- Entries without a dot or slash are prefixes too: `os` matches `os/exec`, but
also `osext` and `os.example.com/pkg`. For the standard library, prefer `$gostd`
(which lists each package exactly) or `os$` and `os/`.

#### An Import Matching More Than One Entry

Within a list, the **longest** matching entry is used. With this deny list:

```yaml
deny:
  github.com/org/legacy/: "use github.com/org/v2"
  github.com/org/legacy/auth/: "use github.com/org/authn"
```

`github.com/org/legacy/auth/token` gets the `authn` suggestion, and every other
package under `github.com/org/legacy/` gets the `v2` suggestion.

When an import matches both the allow and deny lists, the list mode decides:

- **Original:** deny always wins. An empty allow list allows everything.
- **Strict:** the import must match an allow entry. If it also matches a deny
entry, it is only allowed when the allow entry is longer (more specific) than
the deny entry.
- **Lax:** the import is allowed unless it matches a deny entry. If it also
matches an allow entry that is longer than the deny entry, it is allowed.

An exact entry counts as one character longer than the same entry without the
`$`, so `allow: [foo$]` beats `deny: {foo: ...}` for the import `foo` itself, but
`foo/bar` is still denied. Entries of the same length are denied.

For example, in Strict mode with `allow: [github.com/org/]` and
`deny: {github.com/org/legacy/: ...}`, `github.com/org/api` is allowed and
`github.com/org/legacy/db` is denied. The `settings_test.go` file has many more
scenarios under `TestListImportAllowed`.

#### A File Matching More Than One List

Each list a file matches is checked on its own, and an import must be allowed
by **all** of them. Every list that denies it reports its own error. Put the
rules every file shares in one list (for example with `$all`), and add separate
lists for the extra rules of a subset of files (for example `$test`).

### Variables

There are variable replacements for each type of list (file or package). This is
to reduce repetition and tedious behaviors.

#### File Variables

> you can still use an exclamation mark `!` in front of a variable to say not to 
use it. Example `!$test` will match any file that is not a go test file.

- `$all` - matches all go files
- `$test` - matches all go test files

#### Package Variables

- `$gostd` - matches all of go's standard library packages exactly (each one as if
it ended with `$`). If the `GOROOT` environment variable is set, packages are read
from `$GOROOT/src`, otherwise they come from `go list std` using the go toolchain
for the current directory. Internal, vendored, and `cmd` packages are not included.

### Example Configs

Below:

- non-test go files will match `Main` and test go files will match `Test`.
- both allow all of go standard library except for the `reflect` package which will
tell the user "Please don't use reflect package".
- go test files are also allowed to use https://github.com/stretchr/testify package
and any sub-package of it.

```yaml
Main:
  files:
  - $all
  - "!$test"
  allow:
  - $gostd
  deny:
    reflect: Please don't use reflect package
Test:
  files:
  - $test
  allow:
  - $gostd
  - github.com/stretchr/testify
  deny:
    reflect: Please don't use reflect package
```

Below:

- All go files will match `Main`
- Go files in internal will match both `Main` and `Internal`

```yaml
Main:
  files:
  - $all
Internal:
  files:
  - "**/internal/{**/,}*.go"
```

Below:

- All packages are allowed except for `github.com/OpenPeeDeeP/depguard`. Though
`github.com/OpenPeeDeeP/depguard/v2` and `github.com/OpenPeeDeeP/depguard/somepackage`
would be allowed.

```yaml
Main:
  deny:
    github.com/OpenPeeDeeP/depguard$: Please use v2
```

## golangci-lint

This linter was built with
[golangci-lint](https://github.com/golangci/golangci-lint) in mind, read the [linters docs](https://golangci-lint.run/usage/linters/#depguard) to see how to configure all their linters, including this one.

The config is similar to the YAML depguard config documented above, however due to [golangci-lint limitation](https://github.com/golangci/golangci-lint/pull/4227) the `deny` value must be provided as a list, with `pkg` and `desc` keys (otherwise a [panic](https://github.com/OpenPeeDeeP/depguard/issues/74) may occur):

```yaml
# golangci-lint config
linters-settings:
  depguard:
    rules:
      prevent_unmaintained_packages:
        list-mode: lax # allow unless explicitely denied
        files:
          - $all
          - "!$test"
        allow:
          - $gostd
        deny:
          - pkg: io/ioutil
            desc: "replaced by io and os packages since Go 1.16: https://tip.golang.org/doc/go1.16#ioutil"
```

## AI Disclosure

AI tools (Claude) are used in this project as a discussion partner for reviewing
changes, weighing design options, and talking through ideas. The AI does not
write the linter's code; that is written by the maintainer and contributors. It
may assist with some unit tests and documentation, but most of those are also
written by hand, and anything it contributes is reviewed by the maintainer before
it is committed.
