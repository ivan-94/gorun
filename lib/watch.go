package gorun

import (
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/carney520/go-debounce"
	"github.com/fsnotify/fsnotify"
)

var (
	// IgnoreFiles 忽略的文件正则表达式
	IgnoreFiles = []string{
		`.#(\w+).go`, `.(\w+).go.swp`, `(\w+).go~`, `(\w+).tmp`,
	}
	// WatchExts 监视的文件扩展名
	WatchExts = []string{".go"}
)

// Updater 更新器
type Updater = func(files []string) *DepUpdate

// Watcher 文件更新监听器
type Watcher struct {
	watcher *fsnotify.Watcher
	update  Updater
	done    chan struct{}
}

func (w *Watcher) updateWatchDir(update *DepUpdate) error {
	if update == nil {
		return nil
	}
	for _, added := range update.Added {
		err := w.watcher.Add(added)
		if err != nil {
			return err
		}
		log.Printf("watching: %s\n", added)
	}

	for _, removed := range update.Removed {
		err := w.watcher.Remove(removed)
		if err != nil {
			return err
		}
		log.Printf("stop watch: %s", removed)
	}

	return nil
}

// NewWatcher Watcher构造函数
func NewWatcher(initWatchDir []string, update Updater) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	fsWatcher := &Watcher{
		watcher: watcher,
		update:  update,
		done:    make(chan struct{}),
	}

	log.Printf("initialize watcher")
	for _, path := range initWatchDir {
		log.Printf("watching: %s\n", path)
		err := watcher.Add(path)
		if err != nil {
			log.Printf("failed to watch %s", path)
			return fsWatcher, err
		}
	}

	go func() {
		var mux sync.Mutex
		var changedFiles []string

		debounced := debounce.New(500*time.Millisecond, func() {
			mux.Lock()
			files := changedFiles
			changedFiles = []string{}
			mux.Unlock()
			if len(files) > 1 {
				files = StringSliceUniq(files)
			}
			log.Printf("change: %s", files)
			// update
			depUpdate := update(files)
			fsWatcher.updateWatchDir(depUpdate)
		})

		for {
			select {
			case evt := <-watcher.Events:
				// 忽略Create和Chmod事件, 因为没有实际的内容变动
				op := evt.Op
				if op == fsnotify.Create || op == fsnotify.Chmod {
					continue
				}

				// 过滤一些非go文件
				if shouldIgnoreExts(evt.Name, WatchExts) {
					continue
				}

				// 过滤一些文件
				if shouldIgnore(evt.Name, IgnoreFiles) {
					continue
				}

				mux.Lock()
				l := len(changedFiles)
				if l == 0 || changedFiles[l-1] != evt.Name {
					changedFiles = append(changedFiles, evt.Name)
				}
				mux.Unlock()
				debounced.Trigger()
			case err := <-watcher.Errors:
				log.Printf("wacther warning: %s", err)
			case <-fsWatcher.done:
				watcher.Close()
				debounced.Stop()
				return
			}
		}
	}()

	return fsWatcher, nil
}

// 检查是否应该忽略指定的路径
func shouldIgnore(path string, ignoreList []string) bool {
	for _, igstr := range ignoreList {
		reg, err := regexp.Compile(igstr)
		if err != nil {
			log.Fatalf("编译正则表达式(%s)失败: %s \n", igstr, err)
		}
		if reg.MatchString(path) {
			return true
		}
	}
	return false
}

func shouldIgnoreExts(path string, alloweds []string) bool {
	for _, ext := range alloweds {
		if strings.HasSuffix(path, ext) {
			return false
		}
	}
	return true
}
