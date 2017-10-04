## Fixme

[Quick Demo](https://youtu.be/4lU-iX9lWls)

Fixme is a tool for Go projects. It's in kind of a rough state right now and it
may not fit your workflow, but it works, so I'm sharing it. The idea is to
organize several packages into a project and stay notified of the thing that
most needs to be fixed.

Each package in a project is set to one of three levels; watch, test or lint.
Watch will only set a watch on the package, it won't even build it. Test will
build and test that package and lint will also run golint. All the packages in
a project are also resolved into dependency order.

Say a project has three packages; A, B and C where both B and C import A. If
package A is broken, Fixme will not even build B and C because they won't build
until A is fixed. The same thing is true for tests, if A is failing it's tests,
it doesn't bother with B and C. But if A is failing a test and B is failing a
build, it will report the failing build because that is more important. This
lets you focus on one thing at a time instead of seeing every point of failure
in the project.

To install, make sure you have golint installed

```
go get -u github.com/golang/lint/golint
go get -u github.com/adamcolton/fixme
go install github.com/adamcolton/fixme
```

Please send me any questions, requests or suggestions.