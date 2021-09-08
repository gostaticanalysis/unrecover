package unrecover

import (
	"github.com/gostaticanalysis/analysisutil"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
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
		buildssa.Analyzer,
	},
}

func run(pass *analysis.Pass) (interface{}, error) {
	s := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	g := vta.CallGraph(ssautil.AllFunctions(s.Pkg.Prog), cha.CallGraph(s.Pkg.Prog))
	analysisutil.InspectFuncs(s.SrcFuncs, func(i int, instr ssa.Instruction) bool {
		goinstr, _ := instr.(*ssa.Go)
		if goinstr == nil {
			return true
		}

		f := callee(g, goinstr)
		if len(f.Blocks) != 0 && f.Recover == nil {
			pass.Reportf(goinstr.Pos(), "this goroutine does not recover a panic")
		}

		return true
	})
	return nil, nil
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
