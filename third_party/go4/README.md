# go4.org (vendored subset)

`syncutil` and `readerutil` are copied unchanged from
[go4.org](https://github.com/go4org/go4) at commit `f5505b9728dd`
(module version `v0.0.0-20200411211856-f5505b9728dd`), without their test files.
They are the only go4 packages used by `github.com/bodgit/sevenzip`.

The full go4.org module declares dependencies (such as `cloud.google.com/go`) that
gofm does not need; vendoring these two standard-library-only packages, wired in with
`replace go4.org => ./third_party/go4` in `go.mod`, keeps the dependency graph small.

Licence: Apache License 2.0 — see `LICENSE` in this folder.
