module github.com/OpenPeeDeeP/depguard/v2/cmd/depguard

go 1.17

replace github.com/OpenPeeDeeP/depguard/v2 => ../../

require (
	github.com/BurntSushi/toml v1.2.0
	github.com/OpenPeeDeeP/depguard/v2 v2.0.0-00010101000000-000000000000
	github.com/google/go-cmp v0.5.8
	golang.org/x/tools v0.1.12
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/gobwas/glob v0.2.3 // indirect
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4 // indirect
	golang.org/x/sys v0.0.0-20220722155257-8c9f86f7a55f // indirect
)