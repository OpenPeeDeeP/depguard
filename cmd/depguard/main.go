package main

import (
	depguard "github.com/OpenPeeDeeP/depguard/v2"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	analyzer, _ := depguard.NewAnalyzer(&depguard.LinterSettings{})
	singlechecker.Main(analyzer)
}
