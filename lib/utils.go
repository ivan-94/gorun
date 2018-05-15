package gorun

import "log"

var verbose bool

// Printf Wrapper for log.Printf
func Printf(format string, v ...interface{}) {
	if verbose {
		log.Printf(format, v...)
	}
}

// SetVerbose enable verbose mode
func SetVerbose() {
	verbose = true
}

// StringSliceDiff a - b
func StringSliceDiff(a, b []string) []string {
	arr := []string{}
	for _, i := range a {
		var eq bool
		for _, j := range b {
			if i == j {
				eq = true
				break
			}
		}
		if !eq {
			arr = append(arr, i)
		}
	}
	return arr
}

// StringSliceUniq 获取唯一的String Slice
// TODO: 改进算法
func StringSliceUniq(a []string) []string {
	arr := []string{}
	for _, i := range a {
		var eq bool
		for _, j := range arr {
			if i == j {
				eq = true
				break
			}
		}
		if !eq {
			arr = append(arr, i)
		}
	}
	return arr
}
