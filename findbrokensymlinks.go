package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

type LinkCategory int

const (
	BadLink LinkCategory = iota
	BrokenLink
	GoodLink
)

const timeFormat = "2006-01-02-15-04-05.000000"
const defaultDBDir = "/usr/local/share/linkdb"

type Link struct {
	Target string
	Typ    LinkCategory
}

func (l *Link) String() string {
	return fmt.Sprintf("%v:%q", l.Typ, l.Target)
}

var linkCategoryMap = map[string]LinkCategory{
	"Bad":    BadLink,
	"Broken": BrokenLink,
	"Good":   GoodLink,
}

func (c LinkCategory) String() string {
	switch c {
	case BadLink:
		return "Bad"
	case BrokenLink:
		return "Broken"
	case GoodLink:
		return "Good"
	default:
		return fmt.Sprintf("(c LinkCategory) String(): unknown LinkCategoryType %v", c)
	}
}

func (c LinkCategory) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

func (c *LinkCategory) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	v, ok := linkCategoryMap[str]
	if !ok {
		return fmt.Errorf("UnmarshalJSON: unknown LinkCategoryType %v", str)
	}
	*c = v

	return nil
}

type DB struct {
	TimeStamp time.Time
	Links     map[string]*Link
}

func (db *DB) add(path string, l *Link) {
	if v := db.Links[path]; v != nil {
		panic(fmt.Sprintf("duplicate path: %v", v))
	}
	db.Links[path] = l
}

func (db *DB) report() {
	fmt.Println("Bad Links")
	fmt.Println("=========")
	for k, v := range db.Links {
		if v.Typ == BadLink {
			fmt.Printf("%q->%q\n", k, v.Target)
		}
	}
	fmt.Println()

	fmt.Println("Broken Links")
	fmt.Println("=========")
	for k, v := range db.Links {
		if v.Typ == BrokenLink {
			fmt.Printf("%q->%q\n", k, v.Target)
		}
	}
	fmt.Println()

	fmt.Println("Good Links")
	fmt.Println("=========")
	for k, v := range db.Links {
		if v.Typ == GoodLink {
			fmt.Printf("%q->%q\n", k, v.Target)
		}
	}
	fmt.Println()
}

func (db *DB) save(path string) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	b, err := json.MarshalIndent(db, "", "\t")
	if err != nil {
		panic(err)
	}
	_, err = f.Write(b)
	if err != nil {
		panic(err)
	}
}

func printTitle(s string) {
	n := len(s)
	fmt.Println(s)
	for i := 0; i < n; i++ {
		fmt.Print("-")
	}
	fmt.Println()
}

type LinkPair struct {
	oldLink *Link
	newLink *Link
}

func (db *DB) diff(old *DB) *map[string]*LinkPair {
	m := map[string]*LinkPair{}

	for k, v := range db.Links {
		vOld := old.Links[k]
		if vOld == nil {
			m[k] = &LinkPair{vOld, v}
			continue
		}
		if vOld.Target != v.Target || vOld.Typ != v.Typ {
			m[k] = &LinkPair{vOld, v}
		}
	}

	for k, vOld := range old.Links {
		vNew := db.Links[k]
		if vNew != nil {
			continue
		}
		m[k] = &LinkPair{vOld, nil}
	}
	return &m
}

func LoadDB(path string) (*DB, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	var db DB
	err = json.Unmarshal(b, &db)

	return &db, err
}

func GetDBDir(path string) string {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		panic(err)
	}

	fmt.Printf("dbdir:%q\n", path)
	return path
}

func inspectLink(path string) (isLink bool, isDir bool, targ string, cat LinkCategory) {
	fi, err := os.Lstat(path)
	if err != nil {
		panic(err)
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		isLink = true

		targ, err = os.Readlink(path)
		if err != nil {
			panic(err)
		}
		fullTarg := targ
		if !filepath.IsAbs(fullTarg) {
			dir := filepath.Dir(path)
			fullTarg = filepath.Join(dir, targ)
		}

		targFI, err := os.Stat(fullTarg)
		if err == nil {
			// The link points to a real location
			isDir = targFI.IsDir()
			cat = GoodLink
		} else {
			// The link  is invalid
			if utf8.ValidString(targ) {
				cat = BrokenLink
			} else {
				targ = fmt.Sprintf("%q", targ)
				cat = BadLink
			}
		}
	} else {
		isDir = fi.IsDir()
	}

	return
}

func walk(path string, info os.FileInfo, err error, db *DB, count *uint64, showProgress bool) error {
	if err != nil {
		panic(err)
	}

	if showProgress {
		*count++
		if *count%1000 == 0 {
			fmt.Print(".")
		}
	}

	isLink, _, target, cat := inspectLink(path)

	if isLink {
		switch cat {
		case GoodLink:
			db.add(path, &Link{target, GoodLink})
		case BrokenLink:
			db.add(path, &Link{target, BrokenLink})
		case BadLink:
			s := fmt.Sprintf("%q", target)
			db.add(path, &Link{s, BadLink})
		}
	}

	return err
}

func scanCommand(args []string) {
	flags := flag.NewFlagSet("Scan file system", flag.ExitOnError)
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s scan:\n", os.Args[0])
		flags.PrintDefaults()
	}
	dbDir := flags.String("db", defaultDBDir, "Scan db directory")
	showProgress := flags.Bool("p", false, "Show progress")
	skip := flags.String("skip", "/Volumes,/dev", "Absolute path of directories and files to skip")

	flags.Parse(args)
	*dbDir = GetDBDir(*dbDir)

	flags.Parse(args)
	skipFiles := map[string]bool{}
	for _, v := range strings.Split(*skip, ",") {
		skipFiles[v] = true
	}
	files := flags.Args()

	if len(files) == 0 {
		wd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		files = append(files, wd)
	}

	db := new(DB)
	db.Links = make(map[string]*Link)
	db.TimeStamp = time.Now()
	var count uint64

	fnc := func(path string, info os.FileInfo, err error) error {
		if _, ok := skipFiles[path]; ok {
			if info.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}
		return walk(path, info, err, db, &count, *showProgress)
	}

	for _, f := range files {
		_ = filepath.Walk(f, fnc)
	}

	if *showProgress {
		fmt.Println()
	}

	saveLocation := filepath.Join(*dbDir, db.TimeStamp.Format(timeFormat))
	db.save(saveLocation)
}

func diffCommand(args []string) {
	flags := flag.NewFlagSet("Diff the two most recent scans", flag.ExitOnError)
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s diff:\n", os.Args[0])
		flags.PrintDefaults()
	}
	dbDir := flags.String("db", defaultDBDir, "Scan db directory")
	flags.Parse(args)
	*dbDir = GetDBDir(*dbDir)

	ff, err := ioutil.ReadDir(*dbDir)
	if err != nil {
		panic(err)
	}

	if len(ff) < 2 {
		panic("Need at least two scans to diff")
	}

	f := filepath.Join(*dbDir, ff[len(ff)-1].Name())
	newDB, err := LoadDB(f)
	if err != nil {
		panic(err)
	}

	f = filepath.Join(*dbDir, ff[len(ff)-2].Name())
	oldDB, err := LoadDB(f)
	if err != nil {
		panic(err)
	}

	lps := newDB.diff(oldDB)
	for k, lp := range *lps {
		isBad := false
		if lp.oldLink != nil && lp.oldLink.Typ == BadLink {
			isBad = true
		}
		if lp.newLink != nil && lp.newLink.Typ == BadLink {
			isBad = true
		}
		if !isBad {
			continue
		}
		fmt.Println()
		printTitle(k)
		if lp.oldLink != nil && lp.oldLink.Typ == GoodLink && lp.newLink != nil {
			fmt.Printf("ln -sF %q %q\n", lp.oldLink.Target, k)
		} else {
			if lp.oldLink != nil {
				fmt.Printf("Old: %s\n", lp.oldLink)
			}
			if lp.newLink != nil {
				fmt.Printf("New: %s\n", lp.newLink)
			}
		}
	}
}

func badCommand(args []string) {
	flags := flag.NewFlagSet("Show the bad links from the last scan", flag.ExitOnError)
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s diff:\n", os.Args[0])
		flags.PrintDefaults()
	}
	dbDir := flags.String("db", defaultDBDir, "Scan db directory")
	flags.Parse(args)
	*dbDir = GetDBDir(*dbDir)

	ff, err := ioutil.ReadDir(*dbDir)
	if err != nil {
		panic(err)
	}

	if len(ff) < 1 {
		panic("Need at least one scan to show bad links")
	}

	f := filepath.Join(*dbDir, ff[len(ff)-1].Name())
	db, err := LoadDB(f)
	if err != nil {
		panic(err)
	}

	for k, v := range db.Links {
		if v.Typ == BadLink {
			printTitle(k)
			fmt.Printf("%v: %q\n\n", v.Typ, v.Target)
		}
	}
}

func reportLink(label, src, trg string) {
	printTitle(label)
	fmt.Printf("Src: \v\n", src)
	fmt.Printf("Trg: \v\n", trg)
	fmt.Println()
}

func auditCommand(args []string) {
	fmt.Printf("audit\n")

	flags := flag.NewFlagSet("audit", flag.ExitOnError)
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s audit:\n", os.Args[0])
		flags.PrintDefaults()
	}
	dbDir := flags.String("db", defaultDBDir, "Audit db directory")
	flags.Parse(args)
	*dbDir = GetDBDir(*dbDir)

	ff, err := ioutil.ReadDir(*dbDir)
	if err != nil {
		panic(err)
	}

	if len(ff) < 1 {
		panic("Need at least one scan to audit")
	}

	f := filepath.Join(*dbDir, ff[len(ff)-1].Name())
	db, err := LoadDB(f)
	if err != nil {
		panic(err)
	}

	for path, v := range db.Links {
		isLink, _, target, cat := inspectLink(path)
		fmt.Printf("isLink:%v, target:%v, cat:%v\n", isLink, target, cat)

		if !isLink {
			reportLink("No longer a link:", path, target)
			continue
		}

		if v.Typ != cat || v.Target != target {
			reportLink("Link changed:", path, target)
			continue
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s scan|diff|bad|audit:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\tscan - scan and create link db\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\tdiff - diff last two dbs\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\tbad - report on the bad links from the most recent scan\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\taudit - audit the most recent scan results and display any changes\n", os.Args[0])
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(-1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "scan":
		scanCommand(os.Args[2:])
	case "diff":
		diffCommand(os.Args[2:])
	case "bad":
		badCommand(os.Args[2:])
	case "audit":
		auditCommand(os.Args[2:])
	default:
		usage()
		os.Exit(-1)
	}
}
