module dagger.io/dagger

go 1.20

replace github.com/dagger/dagger => ../..

// retract engine releases from SDK releases
retract [v0.0.0, v0.2.36]

require (
	github.com/99designs/gqlgen v0.17.2
	github.com/Khan/genqlient v0.5.0
	github.com/adrg/xdg v0.4.0
	github.com/iancoleman/strcase v0.2.0
	github.com/stretchr/testify v1.8.1
	github.com/vektah/gqlparser/v2 v2.5.1
	golang.org/x/sync v0.2.0
	golang.org/x/tools v0.9.3
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/pretty v0.2.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/mod v0.10.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
