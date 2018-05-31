# Depguard

Go linter that checks package imports are in a list of acceptable packages. It
supports a white list and black list option and only checks the prefixes of the
import path. This allows you to allow imports from a whole organization or only
allow specific packages within a repository.

## Config

By default, Depguard looks for a file named `.depguard.json` in the current
current working directory. If it is somewhere else, pass in the `-c` flag with
the location of your configuration file.

The following is an example configuration file.

```json
{
  "type": "whitelist",
  "packages": [
    "github.com/OpenPeeDeeP/depguard"
  ],
  "includeGoRoot": true
}
```

- `type` can be either `whitelist` or `blacklist`. This check is case insensitive.
- `packages` is a list of packages for the list type specified.
- Set `includeGoRoot` to true if you want to check the list against standard lib.

## Gometalinter

## Golangci-lint
