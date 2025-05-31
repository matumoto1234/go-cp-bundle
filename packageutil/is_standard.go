package packageutil

import (
	"go/types"

	"golang.org/x/tools/go/packages"
)

var standardPackages = make(map[string]struct{})

func init() {
	pkgs, err := packages.Load(nil, "std")
	if err != nil {
		panic(err)
	}

	for _, p := range pkgs {
		standardPackages[p.PkgPath] = struct{}{}
	}
}

func IsStandardPackageName(name string) bool {
	_, ok := standardPackages[name]
	return ok
}

func IsStandardPackage(pkg *types.Package) bool {
	if pkg == nil {
		return false
	}

	return IsStandardPackageName(pkg.Name())
}
