package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/carney520/gorun/lib"
)

// Config 配置
type Config struct {
	Entry        string
	IgnoreVendor bool
}

var flagset *flag.FlagSet
var config Config

func init() {
	flagset = flag.NewFlagSet("gorun", flag.ExitOnError)
	// entry
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get current directory: %s\n", err)
	}

	flagset.StringVar(&config.Entry, "e", "", "alias of -entry")
	flagset.StringVar(&config.Entry, "entry", wd, "directory to watch")
	flagset.BoolVar(&config.IgnoreVendor, "ignoreVendor", true, "ignore watch pacakges in vendor")

	err = flagset.Parse(os.Args[1:])
	if err != nil {
		log.Fatalf("failed to parse arguments: %s\n", err)
	}
}

// 获取go文件
func getFiles(args []string) []string {
	var files []string
	for _, arg := range args {
		if strings.HasSuffix(arg, ".go") {
			files = append(files, arg)
		}
	}
	return files
}

func main() {
	unparsedArgs := flagset.Args()
	gofiles := getFiles(unparsedArgs)

	if len(gofiles) == 0 {
		log.Fatalln("no go files listed")
	}

	runner := gorun.NewRunner(unparsedArgs)
	collector, err := gorun.NewCollector(config.Entry, gofiles, &gorun.CollectOption{
		IgnoreVendor: config.IgnoreVendor,
	})

	if err != nil {
		log.Fatalf("failed to collect denpencies for %s: %s\n", gofiles, err)
	}

	initialWatchDir := collector.GetWatchDirs()
	_, err = gorun.NewWatcher(initialWatchDir, func(files []string) *gorun.DepUpdate {
		if len(files) == 0 {
			return nil
		}
		depUpdate, err := collector.Update(files)
		if err != nil {
			log.Printf("failed to update dependencies: %s", err)
			return nil
		}
		runner.Restart()
		return depUpdate
	})

	if err != nil {
		log.Fatalf("failed to watch files: %s\n", err)
	}

	runner.Run()

	exit := make(chan struct{})
	<-exit
}
