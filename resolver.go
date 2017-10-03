package fixme

import (
	"go/build"
	"os"
	"path/filepath"
)

//https://github.com/cespare/deplist/blob/master/deplist.go

var packageNames map[string][]*Package
var packageImports map[string]*Package

var loaded bool

func load() {
	packageNames = make(map[string][]*Package)
	packageImports = make(map[string]*Package)
	var dirs []*Package
	for _, dir := range build.Default.SrcDirs() {
		dirs = append(dirs, &Package{
			Path: dir,
		})
	}
	for i := 0; i < len(dirs); i++ {
		dir := dirs[i]
		if dir.Import != "" {
			_, name := filepath.Split(dir.Import)
			packageNames[name] = append(packageNames[name], dir)
			packageImports[dir.Import] = dir
		}

		f, err := os.Open(dir.Path)
		if err != nil {
			continue
		}
		children, err := f.Readdir(-1)
		f.Close()
		if err != nil {
			continue
		}
		for _, child := range children {
			if child.IsDir() {
				name := child.Name()
				if name[0] == '.' {
					continue
				}
				var i string
				if dir.Import == "" {
					i = name
				} else {
					i = dir.Import + "/" + name
				}
				c := &Package{
					Path:   dir.Path + "/" + name,
					Import: i,
				}
				dirs = append(dirs, c)
			}
		}
	}
	loaded = true
}

func PackageByName(name string) []*Package {
	if !loaded {
		load()
	}
	return packageNames[name]
}

func PackageByImport(imp string) *Package {
	if !loaded {
		load()
	}
	return packageImports[imp]
}
