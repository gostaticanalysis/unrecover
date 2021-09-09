package unrecover

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/gostaticanalysis/analysisutil"
	"github.com/gostaticanalysis/forcetypeassert"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const doc = "unrecover finds a calling function in other goroutine which does not recover any panic"

var Analyzer = &analysis.Analyzer{
	Name: "unrecover",
	Doc:  doc,
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
		buildssa.Analyzer,
		forcetypeassert.Analyzer,
	},
	FactTypes: []analysis.Fact{(*isPanicableFunc)(nil)},
}

type isPanicableFunc struct{}

func (f *isPanicableFunc) AFact() {}

func (f *isPanicableFunc) String() string {
	return "panicable"
}

func run(pass *analysis.Pass) (interface{}, error) {
	findPanicable(pass)
	checkPanicable(pass)
	return nil, nil
}

func findPanicable(pass *analysis.Pass) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	nodes := []ast.Node{
		(*ast.FuncDecl)(nil),
	}

	callers := make(map[types.Object][]types.Object)
	inspect.Preorder(nodes, func(n ast.Node) {
		fundecl, _ := n.(*ast.FuncDecl)
		if fundecl == nil || fundecl.Body == nil {
			return
		}

		obj := pass.TypesInfo.ObjectOf(fundecl.Name)
		if obj == nil {
			return
		}

		ast.Inspect(fundecl.Body, func(n ast.Node) bool {
			callee := calleeObject(pass, n)

			if callee != nil && callee.Parent() == obj.Parent() {
				callers[callee] = append(callers[callee], obj)
			}

			if isPanicable(pass, n) {
				if !pass.ImportObjectFact(obj, new(isPanicableFunc)) {
					pass.ExportObjectFact(obj, new(isPanicableFunc))
				}
				return false
			}

			return true
		})
	})

	done := make(map[types.Object]bool)
	for callee, _ := range callers {
		exportFact(pass, callers, done, callee)
	}
}

func isPanicable(pass *analysis.Pass, n ast.Node) bool {
	switch n := n.(type) {
	case *ast.CallExpr:
		if isPanicableCall(pass, n) {
			return true
		}
	case *ast.SelectorExpr:
		if isPanicableSelector(pass, n) {
			return true
		}
	case *ast.IndexExpr:
		if isPanicableIndex(pass, n) {
			return true
		}
	case *ast.BinaryExpr:
		if isPanicableBinaryExpr(pass, n) {
			return true
		}
	}

	panicable, _ := pass.ResultOf[forcetypeassert.Analyzer].(*forcetypeassert.Panicable)
	if panicable.Check(n) {
		return true
	}

	return false
}

func calleeObject(pass *analysis.Pass, n ast.Node) types.Object {
	call, _ := n.(*ast.CallExpr)
	if call == nil {
		return nil
	}

	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		return pass.TypesInfo.ObjectOf(fun.Sel)
	case *ast.Ident:
		return pass.TypesInfo.ObjectOf(fun)
	}
	return nil
}

func isPanicableCall(pass *analysis.Pass, n *ast.CallExpr) bool {
	callee := calleeObject(pass, n)
	return (callee != nil && pass.ImportObjectFact(callee, new(isPanicableFunc))) || isBultinPanic(callee)
}

func isBultinPanic(f types.Object) bool {
	return f != nil && f.Name() == "panic" && f.Parent() == types.Universe
}

func isPanicableSelector(pass *analysis.Pass, n *ast.SelectorExpr) bool {
	typ := pass.TypesInfo.TypeOf(n.X)
	if typ == nil {
		return false
	}

	switch typ.Underlying().(type) {
	case *types.Pointer, *types.Interface:
		return true
	}

	return false
}

func isPanicableIndex(pass *analysis.Pass, n *ast.IndexExpr) bool {
	typ := pass.TypesInfo.TypeOf(n.X)
	if typ == nil {
		return false
	}

	switch typ.Underlying().(type) {
	case *types.Slice, *types.Map:
		return true
	}

	return false
}

func isPanicableBinaryExpr(pass *analysis.Pass, n *ast.BinaryExpr) bool {
	typ := pass.TypesInfo.TypeOf(n)
	if typ == nil {
		return false
	}

	switch typ := typ.Underlying().(type) {
	case *types.Basic:
		return typ.Info()&types.IsNumeric != 0 && n.Op == token.QUO
	}

	return false
}

func exportFact(pass *analysis.Pass, m map[types.Object][]types.Object, done map[types.Object]bool, callee types.Object) {
	if done[callee] || !pass.ImportObjectFact(callee, new(isPanicableFunc)) {
		return
	}

	done[callee] = true
	for _, caller := range m[callee] {
		if !pass.ImportObjectFact(caller, new(isPanicableFunc)) {
			pass.ExportObjectFact(caller, new(isPanicableFunc))
			exportFact(pass, m, done, caller)
		}
	}
}

func checkPanicable(pass *analysis.Pass) {
	s := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	g := vta.CallGraph(ssautil.AllFunctions(s.Pkg.Prog), cha.CallGraph(s.Pkg.Prog))
	analysisutil.InspectFuncs(s.SrcFuncs, func(i int, instr ssa.Instruction) bool {
		goinstr, _ := instr.(*ssa.Go)
		if goinstr == nil {
			return true
		}

		f := callee(g, goinstr)
		if f == nil {
			return true
		}

		obj := f.Object()
		if obj != nil && !pass.ImportObjectFact(obj, new(isPanicableFunc)) {
			return true
		}

		if noRecover(g, f) {
			pass.Reportf(goinstr.Pos(), "this goroutine does not recover a panic")
		}

		return true
	})
}

func callee(g *callgraph.Graph, call ssa.CallInstruction) *ssa.Function {
	node := g.Nodes[call.Parent()]
	if node == nil {
		return nil
	}

	for _, edge := range node.Out {
		if edge.Site == call {
			return edge.Callee.Func
		}
	}

	return nil
}

func noRecover(g *callgraph.Graph, f *ssa.Function) bool {
	if f == nil || len(f.Blocks) == 0 {
		return false
	}

	var recovered bool
	analysisutil.InspectInstr(f.Blocks[0], 0, func(i int, instr ssa.Instruction) bool {
		deferInstr, _ := instr.(*ssa.Defer)
		if deferInstr == nil {
			return true
		}

		f := callee(g, deferInstr)
		if !hasRecover(f) {
			return true
		}

		recovered = true
		return false
	})

	return !recovered
}

func hasRecover(f *ssa.Function) bool {
	if f == nil || len(f.Blocks) == 0 {
		return false
	}

	var recovered bool
	analysisutil.InspectInstr(f.Blocks[0], 0, func(i int, instr ssa.Instruction) bool {
		call, _ := instr.(*ssa.Call)
		if call == nil {
			return true
		}

		builtin, _ := call.Common().Value.(*ssa.Builtin)
		if builtin == nil {
			return true
		}

		if builtin.Name() != "recover" {
			return true
		}

		recovered = true
		return false
	})

	return recovered
}
