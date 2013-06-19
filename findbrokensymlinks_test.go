package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

var (
	root   = ""
	src    = ""
	good   = ""
	broken = ""
	// bad     = ""
	dir         = ""
	dirLink     = ""
	dirfileLink = ""
	dirfile     = ""
	dirRecurse  = ""
)

func makeTestDir(t *testing.T) string {
	root = filepath.Join(os.TempDir(), "/_TestGoFindBrokenLinks_")
	src = filepath.Join(root, "/src")
	good = filepath.Join(root, "/good")
	broken = filepath.Join(root, "/broken")
	// bad = filepath.Join(root, "/bad")
	dir = filepath.Join(root, "/dir")
	dirLink = filepath.Join(root, "/dirlink")
	dirfile = filepath.Join(dir, "/dirfile")
	dirfileLink = filepath.Join(dir, "/dirfilelink")
	dirRecurse = filepath.Join(dir, "/dirRecurse")

	// Clean up old test files
	os.RemoveAll(root)

	err := os.Mkdir(root, 0777)
	if err != nil {
		t.Fatalf("MkdirAll %q: %s", root, err)
	}

	_, err = os.Create(src)
	if err != nil {
		t.Fatalf("Create %q: %s", src, err)
	}

	err = os.Symlink("src", good)
	if err != nil {
		t.Fatalf("Symlink %q: %s", src, err)
	}

	err = os.Symlink("brk", broken)
	if err != nil {
		t.Fatalf("Symlink %q: %s", broken, err)
	}

	// err = os.Symlink("", bad)
	// if err != nil {
	// 	t.Fatalf("Symlink %q: %s", bad, err)
	// }

	err = os.Mkdir(dir, 0777)
	if err != nil {
		t.Fatalf("Mkdir %q: %s", dir, err)
	}

	err = os.Symlink("dir", dirLink)
	if err != nil {
		t.Fatalf("Symlink %q: %s", dirLink, err)
	}

	_, err = os.Create(dirfile)
	if err != nil {
		t.Fatalf("Create %q: %s", dirfile, err)
	}

	_, err = os.Create(dirfile)
	if err != nil {
		t.Fatalf("Create %q: %s", dirfile, err)
	}

	err = os.Symlink("../dir", dirRecurse)
	if err != nil {
		t.Fatalf("Symlink %q: %s", dirRecurse, err)
	}

	err = os.Symlink("dirfile", dirfileLink)
	if err != nil {
		t.Fatalf("Symlink %q: %s", dirLink, err)
	}

	return root
}

func TestInspectLinkRegularFile(t *testing.T) {
	path := makeTestDir(t)
	defer os.RemoveAll(path)

	isLink, isDir, _, _ := inspectLink(src)
	if isLink {
		t.Fatalf("expected %v, but got %v", false, isLink)
	}
	if isDir {
		t.Fatalf("expected %v, but got %v", false, isDir)
	}
}

func TestInspectLinkGoodLink(t *testing.T) {
	path := makeTestDir(t)
	defer os.RemoveAll(path)

	isLink, isDir, target, cat := inspectLink(good)
	if !isLink {
		t.Fatalf("expected %v, but got %v", true, isLink)
	}
	if isDir {
		t.Fatalf("expected %v, but got %v", false, isDir)
	}
	if target != "src" {
		t.Fatalf("expected %v, but got %v", "src", target)
	}
	if cat != GoodLink {
		t.Fatalf("expected %v, but got %v", GoodLink, cat)
	}
}

func TestInspectLinkBrokenLink(t *testing.T) {
	path := makeTestDir(t)
	defer os.RemoveAll(path)

	isLink, isDir, target, cat := inspectLink(broken)
	if !isLink {
		t.Fatalf("expected %v, but got %v", true, isLink)
	}
	if isDir {
		t.Fatalf("expected %v, but got %v", false, isDir)
	}
	if target != "brk" {
		t.Fatalf("expected %v, but got %v", "brk", target)
	}
	if cat != BrokenLink {
		t.Fatalf("expected %v, but got %v", BrokenLink, cat)
	}
}

func TestInspectLinkDir(t *testing.T) {
	path := makeTestDir(t)
	defer os.RemoveAll(path)

	isLink, isDir, _, _ := inspectLink(dir)
	if isLink {
		t.Fatalf("expected %v, but got %v", false, isLink)
	}
	if !isDir {
		t.Fatalf("expected %v, but got %v", true, isDir)
	}
}

func TestInspectLinkDirLink(t *testing.T) {
	path := makeTestDir(t)
	defer os.RemoveAll(path)

	isLink, isDir, target, cat := inspectLink(dirLink)
	if !isLink {
		t.Fatalf("expected %v, but got %v", true, isLink)
	}
	if !isDir {
		t.Fatalf("expected %v, but got %v", true, isDir)
	}
	if target != "dir" {
		t.Fatalf("expected %v, but got %v", "dir", target)
	}
	if cat != GoodLink {
		t.Fatalf("expected %v, but got %v", GoodLink, cat)
	}

}

func TestScan(t *testing.T) {
	path := filepath.Clean(makeTestDir(t))
	dbPath := filepath.Clean(filepath.Join(os.TempDir(), "/_TestGoFindBrokenLinks_DB_"))
	defer os.RemoveAll(path)
	defer os.RemoveAll(dbPath)

	var args []string
	args = append(args, "-db")
	args = append(args, dbPath)
	args = append(args, path)
	scanCommand(args)

	fis, err := ioutil.ReadDir(dbPath)
	if err != nil {
		t.Fatalf("ioutil.ReadDir %s", err)
	}

	db, err := LoadDB(dbPath + "/" + fis[0].Name())
	if err != nil {
		t.Fatalf("LoadDB %s", err)
	}

	count := len(db.Links)
	if count != 5 {
		t.Fatalf("expected %v, but got %v", 4, count)
	}

	p := filepath.Join(path, "broken")
	l, ok := db.Links[p]
	if !ok {
		t.Fatalf("expected: %q", p)
	}
	if l.Target != "brk" {
		t.Fatalf("expected %v, but got %v", "brk", l.Target)
	}

	p = filepath.Join(path, "good")
	l, ok = db.Links[p]
	if !ok {
		t.Fatalf("expected: %q", p)
	}
	if l.Target != "src" {
		t.Fatalf("expected %v, but got %v", "src", l.Target)
	}

	p = filepath.Join(path, "dirlink")
	l, ok = db.Links[p]
	if !ok {
		t.Fatalf("expected: %q", p)
	}
	if l.Target != "dir" {
		t.Fatalf("expected %v, but got %v", "dir", l.Target)
	}

}

func TestDiff(t *testing.T) {
	path := filepath.Clean(makeTestDir(t))
	dbPath := filepath.Clean(filepath.Join(os.TempDir(), "/_TestGoFindBrokenLinks_DB_"))
	defer os.RemoveAll(path)
	defer os.RemoveAll(dbPath)
	os.RemoveAll(dbPath)

	var args []string
	args = append(args, "-db")
	args = append(args, dbPath)
	args = append(args, path)
	scanCommand(args)
	os.Remove(path + "/src")
	scanCommand(args)

	fis, err := ioutil.ReadDir(dbPath)
	if err != nil {
		t.Fatalf("ioutil.ReadDir %s", err)
	}

	if len(fis) < 2 {
		t.Fatalf("Need at least two scans to diff")
	}

	oldDB, err := LoadDB(dbPath + "/" + fis[0].Name())
	if err != nil {
		t.Fatalf("LoadDB %s", err)
	}

	newDB, err := LoadDB(dbPath + "/" + fis[1].Name())
	if err != nil {
		t.Fatalf("LoadDB %s", err)
	}

	diff := newDB.diff(oldDB)

	l, ok := (*diff)[path+"/good"]
	if !ok {
		t.Fatalf("expected: %q", path+"/good")
	}
	old := l.oldLink
	if old == nil {
		t.Fatalf("expected oldlink")
	}
	if old.Target != "src" {
		t.Fatalf("expected %v, but got %v", "src", old.Target)
	}
	if old.Typ != GoodLink {
		t.Fatalf("expected %v, but got %v", GoodLink, old.Typ)
	}
	if old.Target != "src" {
		t.Fatalf("expected %v, but got %v", "src", old.Target)
	}
	newLink := l.newLink
	if newLink == nil {
		t.Fatalf("expected newLink")
	}
	if newLink.Typ != BrokenLink {
		t.Fatalf("expected %v, but got %v", BrokenLink, newLink.Typ)
	}
	if newLink.Target != "src" {
		t.Fatalf("expected %v, but got %v", "src", newLink.Target)
	}
}

func TestSkipping(t *testing.T) {
	path := filepath.Clean(makeTestDir(t))
	dbPath := filepath.Clean(filepath.Join(os.TempDir(), "/_TestGoFindBrokenLinks_DB_"))
	defer os.RemoveAll(path)
	defer os.RemoveAll(dbPath)

	var args []string
	args = append(args, "-db")
	args = append(args, dbPath)
	args = append(args, "-skip")
	args = append(args, dir)
	args = append(args, path)
	scanCommand(args)

	fis, err := ioutil.ReadDir(dbPath)
	if err != nil {
		t.Fatalf("ioutil.ReadDir %s", err)
	}

	db, err := LoadDB(dbPath + "/" + fis[0].Name())
	if err != nil {
		t.Fatalf("LoadDB %s", err)
	}

	p := filepath.Join(path, "dir/dirfilelink")
	_, ok := db.Links[p]
	if ok {
		t.Fatalf("unexpected: %q", p)
	}

}
