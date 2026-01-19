module github.com/ermes-thesis/ermes-common/caches

go 1.24.0

require (
	github.com/ermes-thesis/ermes-common/logger v0.0.0-00010101000000-000000000000
	github.com/ermes-thesis/ermes-common/model v0.0.0-00010101000000-000000000000
	github.com/ermes-thesis/ermes-common/utils v0.0.0-00010101000000-000000000000
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/stretchr/testify v1.11.1
	golang.org/x/sync v0.19.0
)

replace github.com/ermes-thesis/ermes-common/model => ../model

replace github.com/ermes-thesis/ermes-common/utils => ../utils

replace github.com/ermes-thesis/ermes-common/logger => ../logger

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
