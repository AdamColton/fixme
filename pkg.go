package fixme

import (
	"go/build"
	"os/exec"
)

type TestState int

const (
	notRun TestState = iota
	failBuild
	failTest
	failLint
	passing
)

var stateStrs = map[TestState]string{
	notRun:    "Not Run",
	failBuild: "Build",
	failTest:  "Test",
	failLint:  "Lint",
	passing:   "Passing",
}

func (t TestState) String() string {
	return stateStrs[t]
}

type Action byte

const (
	None Action = iota
	Watch
	Test
	Lint
)

type Package struct {
	Path         string
	Import       string
	Action       Action
	dependants   []*Package
	dependancies []*Package
	state        TestState
	Data         string
}

func (p *Package) Test() (string, error) {
	return p.run("go", "test")
}

func (p *Package) Build() (string, error) {
	return p.run("go", "build", ".", "errors")
}

func (p *Package) Linter() (string, error) {
	if p.Action != Lint {
		return "", nil
	}
	return p.run("golint")
}

func (p *Package) run(base string, args ...string) (string, error) {
	cmd := exec.Command(base, args...)
	cmd.Dir = p.Path
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (p *Package) Imports() ([]string, error) {
	pkg, err := build.Default.Import(p.Import, p.Path, 0)
	if err != nil {
		return nil, err
	}
	return pkg.Imports, nil
}

func (p *Package) State() TestState { return p.state }

func (p *Package) Clone() *Package {
	if p == nil {
		return nil
	}
	return &Package{
		Path:   p.Path,
		Import: p.Import,
		Action: p.Action,
	}
}

type pkgMap struct {
	byPath   map[string]*Package
	byImport map[string]*Package
}

func newPkgMap() *pkgMap {
	return &pkgMap{
		byPath:   make(map[string]*Package),
		byImport: make(map[string]*Package),
	}
}

func (p *pkgMap) add(pkg *Package) {
	p.byImport[pkg.Import] = pkg
	p.byPath[pkg.Path] = pkg
}

func (p *pkgMap) remove(pkg *Package) {
	delete(p.byImport, pkg.Import)
	delete(p.byPath, pkg.Path)
}

type PackageRecord struct {
	Import string
	Action Action
}

func (p *Package) PackageRecord() PackageRecord {
	return PackageRecord{
		Import: p.Import,
		Action: p.Action,
	}
}

func (p PackageRecord) Package() *Package {
	if !loaded {
		load()
	}
	pkg := PackageByImport(p.Import)
	if pkg == nil {
		return nil
	}
	pkg.Action = p.Action
	return pkg
}
