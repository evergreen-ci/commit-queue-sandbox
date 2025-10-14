package lint

import (
	"go/ast"
	"strings"
)

var (
	testifyFunctionsWeMisspell = []string{"TeardownSuite", "TeardownTest",
		"SetUpSuite", "SetUpTest"}
	testifyFunctionsCorrected = []string{"TearDownSuite", "TearDownTest",
		"SetupSuite", "SetupTest"}
)

func implementsTestifySuite(obj *ast.Object) bool {
	if obj == nil {
		return false
	}
	typespec, ok := obj.Decl.(*ast.TypeSpec)
	if !ok {
		return false
	}
	st, ok := typespec.Type.(*ast.StructType)
	if !ok {
		return false
	}

	if st.Fields == nil {
		return false
	}

	for _, field := range st.Fields.List {
		switch t := field.Type.(type) {
		case *ast.SelectorExpr:
			xIdent, ok := t.X.(*ast.Ident)
			if !ok || t.Sel == nil {
				continue
			}

			if xIdent.Name == "suite" && t.Sel.Name == "Suite" {
				return true
			}
		case *ast.Ident:
			if implementsTestifySuite(t.Obj) {
				return true
			}
		}
	}

	return false
}

func (f *file) lintTestify() {
	if !f.isTest() {
		return
	}
	files := map[string]*ast.File{}
	for k, v := range f.pkg.files {
		files[k] = v.f
	}

	// Cross-file resolution of identifiers in the package
	_, _ = ast.NewPackage(f.pkg.fset, files, nil, nil)

	f.walk(func(node ast.Node) bool {
		switch v := node.(type) {
		case *ast.FuncDecl:
			if v.Recv == nil || v.Name == nil || len(v.Recv.List) == 0 {
				return true
			}
			if v.Type != nil && len(v.Type.Params.List) != 0 {
				return true
			}

			if f.isIgnored(v) {
				return true
			}

			// relax testify detection when linting a single file,
			// in case the struct decl is in a different file
			if len(files) > 1 {
				obj := v.Recv.List[0].Type
				switch t := obj.(type) {
				case *ast.Ident:
					if !implementsTestifySuite(t.Obj) {
						return true
					}

				case *ast.StarExpr:
					ident, ok := t.X.(*ast.Ident)
					if !ok || !implementsTestifySuite(ident.Obj) {
						return true
					}

				default:
					return true
				}
			}

			for i, s := range testifyFunctionsWeMisspell {
				if v.Name.Name == s {
					f.errorf(node, 0.8, "Testify method was spelled '%s', did you mean '%s'?", v.Name.Name, testifyFunctionsCorrected[i])
				}
			}
		}
		return true
	})
}

// Lint the spelling of the word "canceled", ensuring it's spelled the AmE way
func (f *file) lintCancelled() {
	f.walk(func(node ast.Node) bool {
		switch v := node.(type) {
		case *ast.FuncDecl:
			if v.Name == nil {
				return true
			}
			if f.isIgnored(node) {
				return true
			}
			if strings.Contains(strings.ToLower(v.Name.Name), "cancelled") {
				f.errorf(node, 0.7, "prefer the AmE spelling of \"canceled\" in function names (remove an l)")
			}
		}
		return true
	})
}

func (f *file) lintForLoopDefer() {
	f.walk(func(node ast.Node) bool {
		switch v := node.(type) {
		case *ast.ForStmt:
			for _, stmt := range v.Body.List {
				if f.isIgnored(stmt) {
					return true
				}
				tryDefer, ok := stmt.(*ast.DeferStmt)
				if !ok {
					continue
				}

				f.errorf(tryDefer, 0.8, "for loop containing defer will not run until end of function")
			}
		case *ast.RangeStmt:
			for _, stmt := range v.Body.List {
				if f.isIgnored(stmt) {
					return true
				}

				tryDefer, ok := stmt.(*ast.DeferStmt)
				if !ok {
					continue
				}

				f.errorf(tryDefer, 0.8, "for loop containing defer will not run until end of function")
			}
		}

		return true
	})
}
