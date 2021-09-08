package main

import (
	"github.com/gostaticanalysis/unrecover"
	"golang.org/x/tools/go/analysis/unitchecker"
)

func main() { unitchecker.Main(unrecover.Analyzer) }
