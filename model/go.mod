module github.com/ermes-thesis/ermes-common/model

go 1.24.0

toolchain go1.24.4

require (
	github.com/ermes-thesis/ermes-common/utils v0.0.0-00010101000000-000000000000
	github.com/mitchellh/hashstructure/v2 v2.0.2
	github.com/stretchr/testify v1.11.1
)

replace github.com/ermes-thesis/ermes-common/utils => ../utils

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
