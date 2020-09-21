# Requirements

* [Task](https://taskfile.dev/) — alternative for Make
* [Gonstructor](https://github.com/moznion/gonstructor) — tool to generate constructors in Go
* [Genny](https://github.com/cheekybits/genny) — generics through `go generate`

# Generating code

```
task go:generate
```

# Building binary

```
task go:build
```

# Running a new "random" runner

This will create a directory in `.runners/` and assign a new name for the new runner.
```
task rr token=$(taskutils/token)
```
