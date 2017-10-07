package fixme

import (
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"math/rand"
	"strings"
	"time"
)

// Project is a list of packages to test and to watch. If package A imports
// package B and both are in test, then package B will be tested first. If
// package B is in a failing state, the tests for package A will never run. The
// watch files will also change the updates.
type Project struct {
	id         []byte
	Name       string
	pkgs       *pkgMap
	watcher    *fsnotify.Watcher
	closer     chan bool
	Update     <-chan *Package
	sendUpdate chan<- *Package
	testOrder  []*Package
	tmpWatch   []string
}

var seeded bool

func NewProject() *Project {
	if !seeded {
		rand.Seed(time.Now().Unix())
		seeded = true
	}
	id := make([]byte, 8)
	rand.Read(id)
	update := make(chan *Package, 3)
	return &Project{
		Name:       "New Project",
		id:         id,
		pkgs:       newPkgMap(),
		Update:     update,
		sendUpdate: update,
	}
}

func (p *Project) Close() {
	p.closer <- true
}

func (p *Project) Tests(imp string) *Package {
	pkg := p.pkgs.byImport[imp]
	if pkg.Action != Test && pkg.Action != Lint {
		return nil
	}
	return pkg
}

func (p *Project) Watches(imp string) *Package {
	pkg := p.pkgs.byImport[imp]
	if pkg.Action != Watch {
		return nil
	}
	return pkg
}

func (p *Project) Run() error {
	if p.watcher != nil {
		return nil
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	p.watcher = watcher
	for _, t := range p.pkgs.byPath {
		// TODO: us os.Glob to just get Go files.
		p.watcher.Add(t.Path)
	}
	p.closer = make(chan bool)
	go func() {
		var timer <-chan time.Time
		for {
			select {
			case <-watcher.Events:
				// restart the timer, if multiple files are being saved, update will
				// only run once
				timer = time.After(time.Millisecond * 100)
			case err := <-watcher.Errors:
				fmt.Println(p.Name, " Error: ", err)
			case <-timer:
				p.DoUpdate()
				timer = nil
			case <-p.closer:
				watcher.Close()
				p.watcher = nil
				return
			}
		}
	}()
	p.DoUpdate()
	return nil
}

type checkStep func(*Package) *Package

func check(pkgs []*Package, fn checkStep) *Package {
	for _, pkg := range pkgs {
		if errPkg := fn(pkg); errPkg != nil {
			return errPkg
		}
	}
	return nil
}

var checkOrder = []checkStep{builder, tester, linter}

func (p *Project) DoUpdate() {
	for _, tmp := range p.tmpWatch {
		p.watcher.Remove(tmp)
	}

	for i, fn := range checkOrder {
		errPkg := check(p.testOrder, fn)
		if errPkg != nil {
			// if it's a build error, add a temporary watch to failing file
			if i == 0 {
				p.addTempWatch(errPkg)
			}
			p.sendUpdate <- errPkg
			return
		}
	}
	p.sendUpdate <- nil
}

func (p *Project) addTempWatch(pkg *Package) {
	for _, line := range strings.Split(pkg.Data, "\n") {
		if idx := strings.Index(line, ".go:"); idx != -1 {
			file := pkg.Path + "/" + line[:idx+3]
			p.tmpWatch = append(p.tmpWatch, file)
			p.watcher.Add(file)
		}
	}
}

func builder(pkg *Package) *Package {
	str, _ := pkg.Build()
	if str != "" {
		pkg.state = failBuild
		pkg.Data = str
		return pkg
	}
	return nil
}

func tester(pkg *Package) *Package {
	str, _ := pkg.Test()
	strs := strings.Split(str, "\n")
	if len(strs) >= 3 && strings.TrimSpace(strs[len(strs)-3]) != "PASS" {
		pkg.state = failTest
		pkg.Data = str
		return pkg
	}

	return nil
}

func linter(pkg *Package) *Package {
	str, _ := pkg.Linter()
	if str != "" {
		pkg.state = failLint
		pkg.Data = str
		return pkg
	}

	return nil
}

func (p *Project) clearDependancies() {
	for _, pkg := range p.pkgs.byPath {
		pkg.dependancies = nil
		pkg.dependants = nil
	}
	p.testOrder = nil
}

func (p *Project) ResolveDependancies() {
	p.clearDependancies()
	allTests := make(map[string]map[string]bool) // [importpath][dependancy]
	for _, tester := range p.pkgs.byPath {
		if tester.Action == Watch {
			continue
		}
		imps, err := tester.Imports()
		if err != nil {
			continue
		}
		for _, imp := range imps {
			if pkg, ok := p.pkgs.byImport[imp]; ok {
				pkg.dependants = append(pkg.dependants, tester)
				tester.dependancies = append(tester.dependancies, pkg)
			}
		}
	}

	for _, tester := range p.pkgs.byPath {
		if tester.Action == Watch {
			continue
		}
		deps := make(map[string]bool)
		for _, dep := range tester.dependancies {
			if dep.Action != Watch {
				deps[dep.Import] = true
			}
		}
		allTests[tester.Import] = deps
	}

	for len(allTests) > 0 {
		var remove []*Package
		for imp, deps := range allTests {
			if len(deps) != 0 {
				continue
			}
			pkg := p.pkgs.byImport[imp]
			p.testOrder = append(p.testOrder, pkg)
			remove = append(remove, pkg)
		}
		if len(remove) == 0 {
			panic("no progress")
		}
		for _, pkg := range remove {
			delete(allTests, pkg.Import)
			for _, dep := range pkg.dependants {
				if depImps, ok := allTests[dep.Import]; ok {
					delete(depImps, pkg.Import)
				}
			}
		}
	}
}

func (p *Project) AddTest(pkg *Package) {
	pkg.Action = Test
	p.addPkg(pkg)
}

func (p *Project) AddLint(pkg *Package) {
	pkg.Action = Lint
	p.addPkg(pkg)
}

func (p *Project) AddWatch(pkg *Package) {
	pkg.Action = Watch
	p.addPkg(pkg)
}

func (p *Project) addPkg(pkg *Package) {
	p.pkgs.add(pkg)
	if p.watcher != nil {
		p.watcher.Add(pkg.Path)
	}
	p.Save()
}

func (p *Project) Remove(pkg *Package) {
	p.pkgs.remove(pkg)
	p.Save()
}

func (p *Project) JSON() []byte {
	b, err := json.Marshal(p.ProjectRecord())
	if err != nil {
		panic(err)
	}
	return b
}

type ProjectRecord struct {
	Name string
	ID   []byte
	Pkgs []PackageRecord
}

func (p *Project) ProjectRecord() ProjectRecord {
	pr := ProjectRecord{
		Name: p.Name,
		ID:   p.id,
		Pkgs: make([]PackageRecord, 0),
	}
	for _, pkg := range p.pkgs.byPath {
		pr.Pkgs = append(pr.Pkgs, pkg.PackageRecord())
	}
	return pr
}

func (pr ProjectRecord) Project(id []byte) *Project {
	update := make(chan *Package, 3)
	p := &Project{
		id:         id,
		Name:       pr.Name,
		pkgs:       newPkgMap(),
		Update:     update,
		sendUpdate: update,
	}
	for _, pkgRec := range pr.Pkgs {
		pkg := pkgRec.Package()
		if pkg != nil {
			p.pkgs.add(pkg)
		}
	}
	return p
}
