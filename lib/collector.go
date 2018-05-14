package gorun

// 这个文件主要实现根据依赖
// TODO: 优化开发体验，容忍语法错误和导入错误
import (
	"errors"
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"log"
	"path/filepath"
	"strings"
)

var noopStruct = struct{}{}

// Filter 过滤依赖包
type Filter = func(pkg *build.Package) bool

// Pkg 依赖树表示
type Pkg struct {
	// pkg name
	Name string
	// pkg import path
	ImportPath string
	// pkg absoluted dir
	Dir string
	// pkg watchable denpencies
	Dep map[string]*Pkg
	// all imports
	Imports []string
	// global cache: 优化包解析
	cache *pkgCache
	// 被引用次数
	ref int
}

// ToDir 获取依赖所在的目录
func (p *Pkg) ToDir() string {
	return p.Dir
}

type pkgCache struct {
	// 已解析, 避免重复构建 importPath -> *Pkg
	resolved map[string]*Pkg
	// 以过滤, 避免重复导入 importPath -> dir
	rejected map[string]string
	// 文件dir -> 依赖索引
	// 当文件变动时，fsnotify会传递一个文件路径，通过这个文件路径可以索引到依赖节点，
	// 从而只更新这个依赖节点
	dirIndex map[string]*Pkg
}

func (c *pkgCache) Resolve(importPath string, pkg *Pkg) {
	c.resolved[importPath] = pkg
	dir := pkg.ToDir()
	// 目录索引
	if _, ok := c.dirIndex[dir]; !ok {
		c.dirIndex[dir] = pkg
	}
}

func (c *pkgCache) IsResolved(importPath string) (ok bool, pkg *Pkg) {
	pkg, ok = c.resolved[importPath]
	return
}

func (c *pkgCache) Remove(importPath string) {
	delete(c.resolved, importPath)
}

func (c *pkgCache) Reject(importPath, dir string) {
	c.rejected[importPath] = dir
}

func (c *pkgCache) IsRejected(importPath string) bool {
	_, ok := c.rejected[importPath]
	return ok
}

func (c *pkgCache) GetPkgByDir(dir string) *Pkg {
	return c.dirIndex[dir]
}

func newPkgCache() *pkgCache {
	return &pkgCache{
		resolved: make(map[string]*Pkg),
		rejected: make(map[string]string),
		dirIndex: make(map[string]*Pkg),
	}
}

func filterCompose(filters ...Filter) Filter {
	return func(pkg *build.Package) bool {
		for _, filter := range filters {
			res := filter(pkg)
			if !res {
				return false
			}
		}
		return true
	}
}

// 获取绝对路径
func normalizeGofiles(pwd string, files []string) ([]string, error) {
	var normalizes []string
	var dir string
	for _, file := range files {
		var fullPath = file
		if !filepath.IsAbs(file) {
			fullPath = filepath.Join(pwd, file)
		}

		if dir == "" {
			dir = filepath.Dir(fullPath)
		} else if curDir := filepath.Dir(fullPath); curDir != dir {
			return nil, fmt.Errorf("named files must all be in one directory; have %s and %s", dir, curDir)
		}

		normalizes = append(normalizes, fullPath)
	}

	return normalizes, nil
}

// 获取go文件依赖的包
func getDependencies(srcPkg *Pkg, srcDir string, imports []string, filter Filter, recurse bool) error {
	cache := srcPkg.cache
	for _, file := range imports {
		// 已过滤
		if cache.IsRejected(file) {
			continue
		}

		// 已添加
		if _, ok := srcPkg.Dep[file]; ok {
			continue
		}

		// 是否已经缓存
		if ok, cachePkg := cache.IsResolved(file); ok {
			// 已缓存, 不需要向下遍历
			srcPkg.Dep[file] = cachePkg
			cachePkg.ref++
			continue
		}

		// 导入文件
		pkg, err := build.Import(file, srcDir, 0)
		if err != nil {
			return err
		}

		// 过滤: 排除GOROOT 导入, 自定义过滤器过滤
		if pkg.Goroot || !filter(pkg) {
			cache.Reject(file, pkg.Dir)
			continue
		}

		curPkg := &Pkg{
			Name:       pkg.Name,
			ImportPath: pkg.ImportPath,
			Dir:        pkg.Dir,
			Dep:        make(map[string]*Pkg),
			Imports:    pkg.Imports,
			cache:      srcPkg.cache,
		}

		// 递归
		srcPkg.Dep[file] = curPkg
		cache.Resolve(file, curPkg)
		fmt.Println(pkg.Name)
		fmt.Println(pkg.ImportPath)
		fmt.Println(pkg.Dir)
		fmt.Println(pkg.Root)
		fmt.Println(pkg.SrcRoot)
		fmt.Println(pkg.PkgTargetRoot)
		fmt.Println(pkg.BinDir)
		fmt.Println(pkg.PkgObj)
		fmt.Println(pkg.Imports)
		if recurse {
			err = getDependencies(curPkg, pkg.Dir, pkg.Imports, filter, recurse)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// 解析go文件, 获取其中的导入
func getGoFilesImports(file string) (name string, imports []string, err error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
	if err != nil {
		return "", nil, err
	}

	for _, i := range f.Imports {
		imports = append(imports, strings.TrimSuffix(strings.TrimPrefix(i.Path.Value, "\""), "\""))
	}

	return f.Name.Name, imports, nil
}

// CollectOption 依赖收集配置
type CollectOption struct {
	// 忽略vendor目录
	IgnoreVendor bool
}

// Collector 依赖收集/更新器
type Collector struct {
	// 依赖树
	pkg     *Pkg
	cache   *pkgCache
	pwd     string
	gofiles []string
	option  *CollectOption
}

// 依赖过滤
// TODO: 优化, 不要每次获取都创建
func (c *Collector) filter() Filter {
	return filterCompose(
		// ignore vendor
		func(pkg *build.Package) bool {
			if c.option.IgnoreVendor {
				if strings.Contains(filepath.Dir(pkg.ImportPath), "vendor") {
					return false
				}
			}
			return true
		},
		// ignore pacakges out of pwd
		func(pkg *build.Package) bool {
			return filepath.HasPrefix(pkg.Dir, c.pwd)
		})
}

// 初始化依赖树
func (c *Collector) initDeps() error {
	var pkgCache = c.cache
	var filter = c.filter()
	var pkg *Pkg
	var mainImports = make(map[string]struct{})

	// gofiles 都是属于同一个main包, 而且位于同一个目录下
	for _, file := range c.gofiles {
		name, imports, err := getGoFilesImports(file)
		if err != nil {
			return err
		}

		if name != "main" {
			return fmt.Errorf("cannot run not-main package(%s)", file)
		}

		if pkg == nil {
			// file是go文件的绝对路径
			pkg = &Pkg{
				Name:       "main",
				ImportPath: "main",
				Dir:        filepath.Dir(file),
				Dep:        make(map[string]*Pkg),
				cache:      pkgCache,
			}
			pkgCache.Resolve("main", pkg)
		}

		for _, importPath := range imports {
			mainImports[importPath] = noopStruct
		}

		err = getDependencies(pkg, c.pwd, imports, filter, true)

		if err != nil {
			return err
		}
	}

	imports := []string{}
	for importPath := range mainImports {
		imports = append(imports, importPath)
	}
	pkg.Imports = imports
	c.pkg = pkg
	return nil
}

// GetWatchDirs 获取可以被监听的目录
func (c *Collector) GetWatchDirs() []string {
	var watchs []string
	for dir := range c.cache.dirIndex {
		watchs = append(watchs, dir)
	}
	return watchs
}

// DepUpdate 依赖更新记录
type DepUpdate struct {
	Added, Removed []string
}

// Update 文件变动，需要重新计算依赖，并获取需要移除和添加的目录监听列表
func (c *Collector) Update(files []string) (*DepUpdate, error) {
	var mainUpdated bool
	oldWatchs := c.GetWatchDirs()

	for _, file := range files {
		dir := filepath.Dir(file)
		pkg := c.cache.GetPkgByDir(dir)
		if pkg == nil {
			// 新目录，未在列表中, 一般情况下不会出现，因为只有被监听过后的才会有可能被更新
			// 即目录一定被添加在索引中。这种情况可能是一个目录中新增了目录或者文件
			// TODO: 更新其父目录
			log.Printf("warning: unkown update for %s", dir)
			continue
		} else {
			// 在索引中，重新解析包
			if pkg.ImportPath == "main" {
				// main 包，需要统一重新解析gofiles
				if mainUpdated {
					continue
				}
				err := c.updateMainImports()
				if err != nil {
					return nil, err
				}
				mainUpdated = true
			} else {
				// 通过导入路径重新导入
				err := c.updatePkg(pkg)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	// diff
	newWatchs := c.GetWatchDirs()
	added := StringSliceDiff(newWatchs, oldWatchs)
	removed := StringSliceDiff(oldWatchs, newWatchs)
	return &DepUpdate{
		Added:   added,
		Removed: removed,
	}, nil
}

// 移除导入
func (c *Collector) removeImport(pkg *Pkg, importPath string) {
	if dep, ok := pkg.Dep[importPath]; ok {
		// 无其他包引用
		if dep.ref == 0 {
			delete(pkg.Dep, importPath)
			c.cache.Remove(importPath)
			return
		}
		dep.ref--
	}
}

// TODO: 优化
func (c *Collector) diffAndRemoveImports(pkg *Pkg, newImports []string) {
	importMap := make(map[string]struct{})
	for _, importPath := range newImports {
		importMap[importPath] = noopStruct
	}

	for _, importPath := range pkg.Imports {
		if _, ok := importMap[importPath]; !ok {
			// 已被移除
			c.removeImport(pkg, importPath)
		}
	}

	// remove duplicated
	imports := []string{}
	for key := range importMap {
		imports = append(imports, key)
	}

	// update
	pkg.Imports = imports
}

func (c *Collector) updatePkg(pkg *Pkg) error {
	// 重新导入
	newPkg, err := build.Import(pkg.ImportPath, pkg.Dir, 0)
	if err != nil {
		return err
	}
	err = getDependencies(pkg, pkg.Dir, newPkg.Imports, c.filter(), false)
	if err != nil {
		return err
	}
	c.diffAndRemoveImports(pkg, newPkg.Imports)
	return nil
}

// 更新main 导入
func (c *Collector) updateMainImports() error {
	newImports := []string{}
	for _, file := range c.gofiles {
		name, imports, err := getGoFilesImports(file)
		if err != nil {
			return err
		}

		if name != "main" {
			return fmt.Errorf("cannot run not-main package(%s)", file)
		}

		newImports = append(newImports, imports...)

		err = getDependencies(c.pkg, c.pkg.Dir, imports, c.filter(), false)
		if err != nil {
			return err
		}
	}

	// 检查被移除的导入
	c.diffAndRemoveImports(c.pkg, newImports)
	return nil
}

// NewCollector 收集器构造函数
func NewCollector(pwd string, gofiles []string, option *CollectOption) (*Collector, error) {
	if !filepath.IsAbs(pwd) {
		return nil, errors.New("pwd is not absoluted directory")
	}

	gofiles, err := normalizeGofiles(pwd, gofiles)

	if err != nil {
		return nil, err
	}

	collector := &Collector{
		pwd:     pwd,
		gofiles: gofiles,
		option:  option,
		cache:   newPkgCache(),
	}

	err = collector.initDeps()
	if err != nil {
		return collector, err
	}

	return collector, nil
}
