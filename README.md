# gorun

Empower `go run` with live reload

## Install

```shell
go get github.com/carney520/gorun
```

## Usage

Run compiles and runs the main package comprising the named Go source files.
A Go source file is defined to be a file ending in a literal ".go" suffix.

Gorun will generate dependencies upon gofiles, and use Fsnotify to watch the
package dir. When go file changed in watched dir, will reimport related package,
add new or remove unused package from the watching list. In the end, gorun will
rerun 'go run'.

```shell
# Proxy all flags after '--' to `go run`
gorun -ignoreVendor=false -- -x *.go
```

### Options

* -entry=PATH: Set watch scope, package out of this path will be ignore. Default is current work directory
* -ignoreVendor: Ignore pacakges in `vendor`. Default is true
* -printDeps: Just print the pacakges will be watch.
* -debug: Debug mode. print verbose messages

## License

MIT License

Copyright (c) 2018 carney
