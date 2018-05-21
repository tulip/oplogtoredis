package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/tools/benchmark/parse"
)

// Reads in the benchmark reports from `go test -bench`
// and verified that oplogtoredis adds at most 35% latency
// overhead
func main() {
	overheads := []float64{}
	for _, resultFile := range getTestResultFiles() {
		overheads = append(overheads, overheadForTestRun(resultFile))
	}

	meanOverhead := mean(overheads)
	fmt.Printf("Overhead: %.1f%%\n", meanOverhead*100)

	if meanOverhead < getPassThreshold() {
		// pass
		os.Exit(0)
	} else {
		// fail
		os.Exit(1)
	}
}

func getTestResultFiles() []string {
	resultGlob := filepath.Join(os.Getenv("RESULT_DIR"), "*.out")

	files, err := filepath.Glob(resultGlob)
	if err != nil {
		panic(err)
	}

	return files
}

func overheadForTestRun(filepath string) float64 {
	file, err := os.Open(filepath)
	if err != nil {
		panic(err)
	}

	results, err := parse.ParseSet(file)
	if err != nil {
		panic(err)
	}

	baseline := results["BenchmarkInsertNoWait-4"][0].NsPerOp
	withOverhead := results["BenchmarkInsertWaitForRedis-4"][0].NsPerOp

	return (withOverhead / baseline) - 1
	//fmt.Printf("Overhead: %.1f%%\n", overhead*100)
}

func mean(in []float64) float64 {
	total := 0.0
	for _, v := range in {
		total += v
	}
	return total / float64(len(in))
}

func getPassThreshold() float64 {
	threshold, err := strconv.ParseFloat(os.Getenv("PASS_THRESHOLD"), 64)
	if err != nil {
		panic(err)
	}

	return threshold
}
