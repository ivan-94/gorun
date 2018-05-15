package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/carney520/gorun/lib"
)

// Config 配置
type Config struct {
	Entry        string
	IgnoreVendor bool
	PrintDeps    bool
	Verbose      bool
}

var flagset *flag.FlagSet
var config Config

func init() {
	flagset = flag.NewFlagSet("gorun", flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Println(
			`usage: gorun [build flags] [gorun flags] [-exec xprog] gofiles... [arguments...]

Run compiles and runs the main package comprising the named Go source files.
A Go source file is defined to be a file ending in a literal ".go" suffix.

Gorun will generate dependencies upon gofiles, and use Fsnotify to watch the
package dir. When go file changed in watched dir, will reimport related package, 
add new or remove unused package from the watching list. In the end, gorun will 
rerun 'go run'.

Extended flags:`)
		flagset.PrintDefaults()
		fmt.Println("For more about 'go run', see 'go run -h'.")
	}

	// entry
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get current directory: %s\n", err)
	}

	flagset.StringVar(&config.Entry, "e", "", "alias of -entry")
	flagset.StringVar(&config.Entry, "entry", wd, "directory to watch, package out of this directory will be ignore. Default is process.Getwd()\n\t")
	flagset.BoolVar(&config.IgnoreVendor, "ignoreVendor", true, "ignore watch pacakges in vendor")
	flagset.BoolVar(&config.PrintDeps, "printDeps", false, "just print watchable package dirs")
	flagset.BoolVar(&config.Verbose, "debug", false, "verbose")

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
	if config.Verbose {
		gorun.SetVerbose()
	}

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
	if config.PrintDeps {
		fmt.Println(strings.Join(initialWatchDir, "\n"))
		os.Exit(0)
	}

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
