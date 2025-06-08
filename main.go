package main

import (
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gosimple/slug"
	cli "github.com/urfave/cli/v2"
)

const VERSION = "0.2.0"

const (
	version         = "version"
	v               = "v"
	scanDir         = "scanDir"
	sd              = "sd"
	termSearch      = "termSearch"
	ts              = "ts"
	fileSearch      = "fileSearch"
	fs              = "fs"
	stats           = "stats"
	s               = "s"
	duplicates      = "duplicates"
	d               = "d"
	smartDuplicates = "smartDuplicates"
	smartD          = "smartd"
	ignoreList      = "ignoreList"
	il              = "il"
)

const (
	slow = "slow"
	fast = "fast"
)

const (
	MB = 1024 * 1024
)

const (
	maxLines         = 100
	defaultMinLength = 15
)

const (
	redBold    = "\033[1m\033[31m"
	yellowBold = "\033[1m\033[33m"
	blueBold   = "\033[1m\033[43m"
	reset      = "\033[0m"
)

const (
	flagMode                    = "mode"
	flagSearchMinLength         = "search-min-length"
	flagForceWrite              = "force-write"
	flagIgnoreDirectoryMatch    = "ignore-directory-mismatch"
	flagIgnoreCategoryMismatch  = "ignore-category-mismatch"
	flagIgnoreDimensionMismatch = "ignore-dimensions-mismatch"
)

const (
	aliasIdm   = "idm"
	aliasIcm   = "icm"
	aliasIdimm = "idimm"
)

const (
	unknownCategory   = "unknown"
	unknownDimensions = "unknown"
)

func main() {
	app := CreateApp(NewStdOut())

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func CreateApp(output Output) *cli.App {
	return &cli.App{
		Commands: []*cli.Command{
			{
				Name:    version,
				Aliases: []string{v},
				Usage:   "Display version",
				Action: func(cCtx *cli.Context) error {
					fmt.Println(VERSION)

					return nil
				},
			},
			{
				Name:    scanDir,
				Aliases: []string{sd},
				Usage:   "Scan will scan a list of directories and store them in the DB file",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  flagForceWrite,
						Value: false,
						Usage: "Force updating a directory, even if it's empty",
					},
				},
				Action: func(cCtx *cli.Context) error {
					return ScanCommand(
						output,
						cCtx.Args().Get(0),
						cCtx.Args().Slice()[1:],
						cCtx.Bool(flagForceWrite),
					)
				},
			},
			{
				Name:    termSearch,
				Aliases: []string{ts},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  flagMode,
						Value: slow,
						Usage: "Find only exact-search terms (fast) or search by contains (slow)",
					},
				},
				Action: func(cCtx *cli.Context) error {
					return TermSearchCommand(
						output,
						cCtx.Args().Get(0),
						cCtx.String(flagMode),
						cCtx.Args().Slice()[1:],
					)
				},
			},
			{
				Name:    fileSearch,
				Aliases: []string{fs},
				Action: func(cCtx *cli.Context) error {
					return FileSearchCommand(
						output,
						cCtx.Args().Get(0),
						cCtx.String(flagMode),
						cCtx.Args().Get(1),
					)
				},
			},
			{
				Name:    duplicates,
				Aliases: []string{d},
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  flagSearchMinLength,
						Value: defaultMinLength,
						Usage: "Find only exact-search terms (fast) or search by contains (slow)",
					},
				},
				Action: func(cCtx *cli.Context) error {
					return DuplicateCommand(
						output,
						cCtx.Args().Get(0),
						cCtx.Int(flagSearchMinLength),
					)
				},
			},
			{
				Name:    smartDuplicates,
				Aliases: []string{smartD},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    flagIgnoreDirectoryMatch,
						Aliases: []string{aliasIdm},
						Value:   false,
						Usage:   "Compare files even if they're in the same directory.",
					},
					&cli.BoolFlag{
						Name:    flagIgnoreCategoryMismatch,
						Aliases: []string{aliasIcm},
						Value:   false,
						Usage:   "Compare files even if they're in non-matching categories",
					},
					&cli.BoolFlag{
						Name:    flagIgnoreDimensionMismatch,
						Aliases: []string{aliasIdimm},
						Value:   false,
						Usage:   "Compare files even if they have different dimensions set",
					},
				},
				Action: func(cCtx *cli.Context) error {
					return SmartDuplicateCommand(
						output,
						cCtx.Args().Get(0),
						cCtx.Bool(flagIgnoreDirectoryMatch),
						cCtx.Bool(flagIgnoreCategoryMismatch),
						cCtx.Bool(flagIgnoreDimensionMismatch),
					)
				},
			},
			{
				Name:    ignoreList,
				Aliases: []string{il},
				Flags:   []cli.Flag{},
				Action: func(cCtx *cli.Context) error {
					return IgnoreListCommand(
						output,
						cCtx.Args().Get(0),
					)
				},
			},
			{
				Name:    stats,
				Aliases: []string{s},
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  flagSearchMinLength,
						Value: defaultMinLength,
						Usage: "Find only exact-search terms (fast) or search by contains (slow)",
					},
				},
				Action: func(cCtx *cli.Context) error {
					return StatsCommand(
						output,
						cCtx.Args().Get(0),
						cCtx.Int(flagSearchMinLength),
					)
				},
			},
		},
	}
}

func ScanCommand(output Output, dbFile string, roots []string, forceWrite bool) error {
	db := NewDB(output, dbFile)

	db.Load(false)

	err := db.Scan(roots, forceWrite)
	if err != nil {
		output.Printf("Error scanning directories: %v\n", err)
		output.Exit(1)
	}

	err = db.Write()
	if err != nil {
		output.Printf("Error writing DB: %v\n", err)
		output.Exit(1)
	}

	return nil
}

func TermSearchCommand(output Output, dbFile, modeFlag string, searchTerms []string) error {
	db := NewDB(output, dbFile)

	db.Load(false)

	db.Search(modeFlag, searchTerms)

	return nil
}

func FileSearchCommand(output Output, dbFile, modeFlag, filePath string) error {
	db := NewDB(output, dbFile)

	db.Load(false)

	searchTerms := pathToSearchTerms(filePath)

	db.Search(modeFlag, searchTerms)

	return nil
}

func DuplicateCommand(output Output, dbFile string, searchMinLength int) error {
	db := NewDB(output, dbFile)

	db.Load(false)

	db.Duplicates(searchMinLength)

	err := db.Write()
	if err != nil {
		output.Printf("Error writing DB: %v\n", err)
		output.Exit(1)
	}

	return nil
}

func SmartDuplicateCommand(output Output, dbFile string, ignoreDirectory, ignoreCategory, ignoreDimensions bool) error {
	db := NewDB(output, dbFile)

	db.Load(true)

	db.lock.Lock()
	defer db.lock.Unlock()

	groups := db.smartDuplicates(ignoreDirectory, ignoreCategory, ignoreDimensions)

	db.handleDuplicateGroups(groups)

	err := db.Write()
	if err != nil {
		output.Printf("Error writing DB: %v\n", err)
		output.Exit(1)
	}

	return nil
}

var cleanRegexpSpec = regexp.MustCompile(`^[0-9]*[a-z]+$`)

func IgnoreListCommand(output Output, dbFile string) error {
	db := NewDB(output, dbFile)

	db.Load(false)

	m := make(map[string]int)
	for _, file := range db.Files {
		base := filepath.Base(file.Path)
		if base == strings.ToLower(base) {
			continue
		}

		parts := strings.Split(base, "-")
		for _, part := range parts {
			if !cleanRegexpSpec.MatchString(part) {
				break
			}

			if len(part) < 4 {
				continue
			}

			m[part] += 1
		}
	}

	ignoreList := make([]string, 0, len(m)/4)
	for part, count := range m {
		if count > 5 {
			ignoreList = append(ignoreList, part)
		}
	}

	slices.Sort(ignoreList)

	for _, part := range ignoreList {
		db.output.Println(part)
	}

	// db.output.Printf("\n\n%d words found for the ignore list.\n\n", len(ignoreList))

	return nil
}

func StatsCommand(output Output, dbFile string, searchMinLength int) error {
	db := NewDB(output, dbFile)

	db.Load(false)

	db.Stats(searchMinLength)

	return nil
}

type Output interface {
	Println(a ...any)
	Printf(format string, a ...any)
	Scanln(a *string) error
	Exit(code int)
}

type StdOut struct{}

func (out *StdOut) Println(a ...any) {
	fmt.Println(a...)
}

func (out *StdOut) Printf(format string, a ...any) {
	fmt.Printf(format, a...)
}

func (out *StdOut) Scanln(a *string) error {
	_, err := fmt.Scanln(a)
	if err != nil {
		return fmt.Errorf("error scanning input: %w", err)
	}

	return nil
}

func (out *StdOut) Exit(code int) {
	os.Exit(code)
}

func NewStdOut() *StdOut {
	return &StdOut{}
}

type Record struct {
	Path        string
	Directory   string
	Size        int
	Hash        string
	SearchTerms []string
	SmartTerms  []uint64
	Category    string
	Dimensions  string
}

type ID string

type DB struct {
	lock        *sync.RWMutex
	Files       map[ID]Record
	Sizes       map[int][]ID
	Hashes      map[string][]ID
	SearchTerms map[string][]ID
	output      Output
	dbFile      string
	ids         []ID
	smartDB     *SmartDB
}

func NewDB(output Output, dbFile string) *DB {
	return &DB{
		lock:        &sync.RWMutex{},
		Files:       make(map[ID]Record),
		Sizes:       make(map[int][]ID),
		Hashes:      make(map[string][]ID),
		SearchTerms: make(map[string][]ID),
		output:      output,
		dbFile:      dbFile,
		smartDB:     NewSmartDB(),
	}
}

func (db *DB) Load(smartTermsNeeded bool) {
	db.lock.Lock()
	defer db.lock.Unlock()

	lines, err := readCsvFile(db.dbFile)
	if err != nil {
		db.output.Printf("Unable to read DB file '%s', error: %v", db.dbFile, err)

		db.output.Exit(1)
	}

	for _, columns := range lines {
		err := db.handleLine(columns, smartTermsNeeded)
		if err != nil {
			break
		}
	}
}

func readCsvFile(filePath string) ([][]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read input file '%s', err: %w", filePath, err)
	}
	defer f.Close()

	csvReader := csv.NewReader(f)
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("unable to parse file as CSV for '%s', err: %w", filePath, err)
	}

	return records, nil
}

func (db *DB) handleLine(columns []string, smartTermsNeeded bool) error {
	filePath := columns[0]

	filePath = strings.TrimSpace(filePath)

	if len(filePath) == 0 {
		return nil
	}

	size, err := strconv.Atoi(columns[1])
	if err != nil {
		db.output.Println("Unable to parse size from record. File path:", columns[0], "Raw data:", columns[1], ", error:", err.Error())

		return err
	}

	hash := columns[2]

	searchTerms := pathToSearchTerms(filePath)

	var (
		smartTerms []uint64
		category   string
		dimensions string
	)
	if smartTermsNeeded {
		smartTerms, category, dimensions, err = db.smartDB.ProcessPath(filePath)
		if err != nil {
			db.output.Printf("Unable to retrieve smart terms for file. file path: %s, error: %s\n", filePath, err.Error())

			return err
		}
	}

	err = db.add(filePath, size, hash, searchTerms, smartTerms, category, dimensions)
	if err != nil {
		db.output.Println("Unable to add record to DB, file path:", filePath, ", error:", err.Error())

		return err
	}

	return nil
}

func (db *DB) Scan(roots []string, forceWrite bool) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	for _, root := range roots {
		files, err := collectFiles(root)
		if err != nil {
			return fmt.Errorf("unable to collect files in root %s, err: %w", root, err)
		}

		if len(files) == 0 && !forceWrite {
			continue
		}

		db.handleMatches(root, files)
	}

	return nil
}

func collectFiles(root string) (map[string]struct{}, error) {
	result := make(map[string]struct{})

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			result[path] = struct{}{}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("unable to walk directory %s, err: %w", root, err)
	}

	return result, nil
}

func (db *DB) handleMatches(root string, files map[string]struct{}) {
	// Add files found to the database, if not already there
	skipped := 0
	created := 0
	for filename := range files {
		if _, ok := db.Files[ID(filename)]; ok {
			skipped++

			continue
		}

		err := db.handleMatch(filename)
		if err != nil {
			db.output.Println(err.Error())

			break
			// TODO: continue
		}

		created++
	}

	// Remove the files from the database which can no longer be found in the file system
	deleted := 0
	for _, record := range db.Files {
		if !strings.HasPrefix(record.Path, root) {
			continue
		}

		if _, ok := files[record.Path]; !ok {
			delete(db.Files, ID(record.Path))

			deleted++
		}
	}

	db.output.Printf("root: %s, %d found files, %d skipped, %d created, %d deleted\n", root, len(files), skipped, created, deleted)
}

func (db *DB) handleMatch(filename string) error {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("unable to stat file %s, err: %w", filename, err)
	}

	size := fileInfo.Size()
	searchTerms := pathToSearchTerms(filename)

	hashSize := MB
	if size < MB {
		hashSize = int(size)
	}

	hash, err := hashFile(filename, hashSize)
	if err != nil {
		return fmt.Errorf("unable to hash file %s, err: %w", filename, err)
	}

	err = db.add(filename, int(size), hash, searchTerms, nil, "", "")
	if err != nil {
		return fmt.Errorf("unable to add record to DB, file path: %s, err: %w", filename, err)
	}

	return nil
}

func (db *DB) add(filePath string, size int, hash string, searchTerms []string, smartTerms []uint64, category, dimensions string) error {
	id := ID(filePath)

	db.ids = append(db.ids, id)
	db.Files[id] = Record{
		Path:        filePath,
		Directory:   filepath.Dir(filePath),
		Size:        size,
		Hash:        hash,
		SearchTerms: searchTerms,
		SmartTerms:  smartTerms,
		Category:    category,
		Dimensions:  dimensions,
	}
	db.Sizes[size] = append(db.Sizes[size], id)
	for _, term := range searchTerms {
		db.SearchTerms[term] = append(db.SearchTerms[term], id)
	}
	db.Hashes[hash] = append(db.Hashes[hash], id)

	return nil
}

func (db *DB) Write() error {
	db.lock.RLock()
	defer db.lock.RUnlock()

	// write CSV file from db.Files
	file, err := os.Create(db.dbFile)
	if err != nil {
		return fmt.Errorf("unable to create DB file %s, err: %w", db.dbFile, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	ids := db.ids

	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	for _, id := range db.ids {
		record := []string{db.Files[id].Path, strconv.Itoa(db.Files[id].Size), db.Files[id].Hash}
		err = writer.Write(record)
		if err != nil {
			return fmt.Errorf("unable to write record to DB file %s, err: %w", db.dbFile, err)
		}
	}

	return nil
}

func pathToSearchTerms(filePath string) []string {
	_, fileName := filepath.Split(filePath)

	var terms []string
	for _, term := range strings.Split(fileName, "-") {
		terms = append(terms, strings.ToLower(strings.TrimSpace(term)))
	}

	return terms
}

func (db *DB) Search(searchType string, searchTerms []string) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	var allIDs [][]ID

	for i := range searchTerms {
		searchTerms[i] = strings.ToLower(searchTerms[i])
	}

	switch searchType {
	case fast:
		allIDs = db.fastCollectIDs(searchTerms)
	case slow:
		allIDs = db.slowCollectIDs(searchTerms)
	}

	if len(allIDs) == 0 {
		db.output.Println("No results found.")

		return
	}

	intersected := intersectAllIDs(allIDs)

	db.PrintIDs(intersected, searchTerms)
}

func (db *DB) fastCollectIDs(searchedTerms []string) [][]ID {
	var results [][]ID

	for _, searchedTerm := range searchedTerms {
		termIDs, ok := db.SearchTerms[searchedTerm]
		if !ok {
			db.output.Printf("No results found for search term '%s'.\n", searchedTerm)

			return nil
		}

		if len(termIDs) == 0 {
			return nil
		}

		results = append(results, termIDs)
	}

	return results
}

func (db *DB) slowCollectIDs(searchedTerms []string) [][]ID {
	var results [][]ID

	for _, searchedTerm := range searchedTerms {
		found := make(map[ID]struct{})

		for term, ids := range db.SearchTerms {
			if !strings.Contains(term, searchedTerm) {
				continue
			}

			for _, id := range ids {
				found[id] = struct{}{}
			}
		}

		if len(found) == 0 {
			db.output.Printf("No results found for search term '%s'.\n", searchedTerm)

			return nil
		}

		uniqueIDs := []ID{}
		for id := range found {
			uniqueIDs = append(uniqueIDs, id)
		}

		results = append(results, uniqueIDs)
	}

	return results
}

func intersectAllIDs(idGroups [][]ID) []ID {
	idGroup := idGroups[0]
	for _, termIDs := range idGroups[1:] {
		idGroup = intersectIDs(idGroup, termIDs)

		if len(idGroup) == 0 {
			return nil
		}
	}

	return idGroup
}

func intersectIDs(a, b []ID) []ID {
	var result []ID

	for _, id := range a {
		if slices.Contains(b, id) {
			result = append(result, id)
		}
	}

	return result
}

func (db *DB) PrintIDs(ids []ID, searchTerms []string) {
	if len(ids) > maxLines {
		ids = ids[:maxLines]
	}

	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	for i, id := range ids {
		record := db.Files[id]

		path := FindHighlights(record.Path, searchTerms)

		db.output.Printf("[%d] %s (%d MB)\n", i+1, path, record.Size/MB)
	}

	if len(ids) >= maxLines {
		db.output.Println("... (truncated)")
	}
}

func FindHighlights(haystack string, needles []string) string {
	var highlights [][2]int

	lower := strings.ToLower(haystack)
	for _, searchTerm := range needles {
		idx := strings.Index(lower, searchTerm)
		if idx == -1 {
			continue
		}

		highlights = append(highlights, [2]int{idx, idx + len(searchTerm)})
	}

	sort.Slice(highlights, func(i, j int) bool {
		return highlights[i][0] < highlights[j][0]
	})

	var parts []string

	tmp := 0
	for _, highlight := range highlights {
		// This means that ranges overlap. Let's abort highlighting for the sake of simplicity.
		if tmp > 0 && tmp > highlight[0] {
			return blueBold + haystack + reset
		}

		if len(haystack) < highlight[1] {
			fmt.Printf("Unexpected highligh issue. tmp: %d, highlight: %v, haystack: %s", tmp, highlight, haystack)

			return blueBold + haystack + reset
		}

		parts = append(parts,
			haystack[tmp:highlight[0]],
			redBold+haystack[highlight[0]:highlight[1]]+reset)

		tmp = highlight[1]
	}

	if tmp < len(haystack) {
		parts = append(parts, haystack[tmp:])
	}

	if len(parts) <= 1 {
		return yellowBold + haystack + reset
	}

	return strings.Join(parts, "")
}

func hashFile(path string, sampleSize int) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("can't stat file: %s, err: %w", path, err)
	}

	if fi.Size() < MB {
		sampleSize = int(fi.Size())
	}

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("can't open file: %s, err: %w", path, err)
	}

	data := make([]byte, sampleSize)

	_, err = f.Read(data)
	if err != nil {
		return "", fmt.Errorf("can't read file: %s, err: %w", path, err)
	}

	if err = f.Close(); err != nil {
		return "", fmt.Errorf("can't close file: %s, err: %w", path, err)
	}

	md5Hasher := md5.New()
	_, err = md5Hasher.Write(data)
	if err != nil {
		return "", fmt.Errorf("can't calculate md5 hash for file: %s, err: %w", path, err)
	}
	sum := md5Hasher.Sum(nil)

	return hex.EncodeToString(sum), nil
}

type SmartDB struct {
	internal map[string]uint64
	lock     *sync.Mutex
}

func NewSmartDB() *SmartDB {
	return &SmartDB{
		internal: make(map[string]uint64),
		lock:     new(sync.Mutex),
	}
}

var descriptionRegexp = regexp.MustCompile(`^(.*)-(\d+[a-z]{2,}.*)$`)
var dimensionRegexp = regexp.MustCompile(`(.*)-(\d{3,4}x\d{3,4})$`)
var wellKnownDimensions = []string{
	"-8k-4320p",
	"-4k-2160p",
	"-2k-1080p",
	"-qhd-1440p",
	"-fullhd-1080p",
	"-hd-720p",
	"-ed-540p",
	"-sd-480p",
}

func (sdb *SmartDB) ProcessPath(in string) ([]uint64, string, string, error) {
	in = filepath.Base(in)
	ext := filepath.Ext(in)
	if ext != "" && len(ext) < len(in) {
		in = in[:len(in)-len(ext)]
	}
	finds := descriptionRegexp.FindStringSubmatch(in)

	base := in
	description := ""
	if len(finds) > 1 {
		base = finds[1]
		description = finds[2]
	}
	category := getCategoryFromDescription(description)

	finds = dimensionRegexp.FindStringSubmatch(base)
	dimensions := unknownDimensions
	if len(finds) > 1 {
		base = finds[1]
		dimensions = finds[2]
	}

	for _, dim := range wellKnownDimensions {
		if len(base) > len(dim) && base[len(base)-len(dim):] == dim {
			base = base[:len(base)-len(dim)]
			dimensions = dim[1:]
			break
		}
	}

	cleanParts, err := getCleanedParts(base)
	if err != nil {
		return nil, "", "", err
	}

	smartTerms := make([]uint64, 0, len(cleanParts))
	for _, part := range cleanParts {
		smartTerms = append(smartTerms, sdb.GetValue(part))
	}

	slices.Sort(smartTerms)
	slices.Reverse(smartTerms)

	return smartTerms, category, dimensions, nil
}

var cleanRegexp = regexp.MustCompile(`^[a-z0-9]+$`)

func getCleanedParts(fn string) ([]string, error) {
	parts := strings.Split(fn, "-")

	for i, part := range parts {
		if cleanRegexp.MatchString(part) {
			continue
		}

		parts[i] = cleanPart(part)
	}

	return parts, nil
}

var notCategories = []string{"low", "phone", "brut", "cut"}
var unknownCategories = []string{"full", "clip"}

func getCategoryFromDescription(description string) string {
	parts := strings.Split(description, "-")

	for i := len(parts) - 1; i >= 1; i-- {
		skip := false
		part := strings.TrimRight(parts[i], "123456789")

		for _, word := range unknownCategories {
			if part == word || len(word) == len(part)-1 && part[:len(part)-2] == word {
				return unknownCategory
			}
		}

		for _, word := range notCategories {
			if part == word {
				skip = true
			}
		}

		if !skip {
			return part
		}
	}

	return unknownCategory
}

func cleanPart(in string) string {
	m := map[string]string{
		"-": "",
		"_": "",
		":": "",
		" ": "",
	}

	out := strings.ToLower(in)
	for k, v := range m {
		out = strings.ReplaceAll(out, k, v)
	}

	text := slug.Make(out)

	return text
}

func (sdb *SmartDB) GetValue(in string) uint64 {
	sdb.lock.Lock()
	defer sdb.lock.Unlock()

	if res, ok := sdb.internal[in]; ok {
		return res
	}

	length := min(len(in)+20, 62)
	res := uint64(1<<length) + uint64(len(sdb.internal))
	sdb.internal[in] = res

	return res
}

func (db *DB) Stats(minLength int) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	db.output.Printf("Total records: %d\n", len(db.Files))
	db.output.Printf("Total unique sizes: %d\n", len(db.Sizes))
	db.output.Printf("Total unique search terms: %d\n", len(db.SearchTerms))
	db.output.Printf("Total unique hashes: %d\n", len(db.Hashes))

	db.sizeStats()
	db.hashStats()

	db.searchTermStats(minLength)
}

func (db *DB) sizeStats() {
	sizesWithMultipleIDs := 0

	for _, ids := range db.Sizes {
		if len(ids) == 1 {
			continue
		}

		sizesWithMultipleIDs++
	}

	db.output.Printf("Sizes with multiple records: %d\n", sizesWithMultipleIDs)
}

func (db *DB) hashStats() {
	hashWithMultipleIDs := 0

	for _, ids := range db.Hashes {
		if len(ids) == 1 {
			continue
		}

		hashWithMultipleIDs++
	}

	db.output.Printf("Hashes with multiple records: %d\n", hashWithMultipleIDs)
}

func (db *DB) searchTermStats(minLength int) {
	searchTermStats := make(map[int]int)
	for searchTerm, ids := range db.SearchTerms {
		if len(ids) < 2 {
			continue
		}

		if len(searchTerm) < minLength {
			continue
		}

		key := len(searchTerm) / 5

		searchTermStats[key] += 1
	}

	// sort searchTerms by keys
	keys := make([]int, 0, len(searchTermStats))
	for k := range searchTermStats {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	db.output.Println()
	db.output.Printf("Search term length distribution:\n")
	for _, length := range keys {
		db.output.Printf("Search terms with length %d: %d\n", length*5, searchTermStats[length])
	}
}

func (db *DB) Duplicates(minLength int) {
	db.lock.Lock()
	defer db.lock.Unlock()

	groups := db.duplicatesBySizeAndHash()
	db.handleDuplicateGroups(groups)

	groups = db.duplicatesBySearchTerm(minLength)
	db.handleDuplicateGroups(groups)
}

type SearchType string

const (
	SizeAndHash SearchType = "Size and hash"
	SearchTerm  SearchType = "Search term"
	Smart       SearchType = "Smart"
)

type SearchGroup struct {
	IDs         []ID
	SearchTerms []string
	Type        SearchType
}

func (db *DB) smartDuplicates(ignoreDirectory, ignoreCategory, ignoreDimensions bool) []SearchGroup {
	ids := make([]ID, 0, len(db.Files))
	for id := range db.Files {
		ids = append(ids, id)
	}

	maxCompared := int64(len(ids) * (len(ids) - 1) / 2)
	compared := int64(0)
	minimum := 0.1
	stored := 0
	onePercent := maxCompared / 100

	tmp := make(map[float64][][2]ID)
	for i := range ids {
		for j := i + 1; j < len(ids); j++ {
			compared++
			cmp, _ := compareSmart(db.Files[ids[i]], db.Files[ids[j]], ignoreDirectory, ignoreCategory, ignoreDimensions)
			if compared%onePercent == 0 {
				db.output.Printf("max compared: %d, compared: %d, percent: %d\n", maxCompared, compared, compared*100/maxCompared)
			}
			if cmp <= minimum {
				continue
			}

			tmp[cmp] = append(tmp[cmp], [2]ID{ids[i], ids[j]})
			stored++

			if stored%10_000 == 0 {
				tmp, minimum = cleanSmart(tmp)
			}
		}
	}

	tmp, _ = cleanSmart(tmp)

	groups := []SearchGroup{}
	for _, idPairs := range tmp {
		for _, idPair := range idPairs {
			r1 := db.Files[idPair[0]]
			r2 := db.Files[idPair[1]]

			groups = append(groups, SearchGroup{
				IDs:         []ID{idPair[0], idPair[1]},
				SearchTerms: []string{r1.Category, r2.Category},
				Type:        Smart,
			})
		}
	}

	return groups
}

func cleanSmart(m map[float64][][2]ID) (map[float64][][2]ID, float64) {
	scores := make([]float64, 0, len(m))
	for score := range m {
		scores = append(scores, score)
	}

	slices.Sort(scores)
	slices.Reverse(scores)

	found := 0
	minScore := -1.0
	for _, score := range scores {
		// Maximum already found we just need to delete entries
		if minScore > 0.0 {
			delete(m, score)
			continue
		}

		// We haven't reached the limit yet
		if found+len(m[score]) < 10 {
			found += len(m[score])
			continue
		}

		// Maximum is just found
		if found+len(m[score]) > 10 {
			m[score] = m[score][:10-found]
		}

		minScore = score
		found += len(m[score])
	}

	if minScore < 0.0 {
		return m, scores[len(scores)-1]
	}

	return m, minScore
}

func (db *DB) duplicatesBySizeAndHash() []SearchGroup {
	groups := []SearchGroup{}

	for _, ids := range db.Hashes {
		if len(ids) < 2 {
			continue
		}

		sizes := make(map[int][]ID)
		for _, id := range ids {
			size := db.Files[id].Size

			if size < 1000 {
				continue
			}

			sizes[size] = append(sizes[size], id)
		}

		for _, sizeIDs := range sizes {
			if len(sizeIDs) < 2 {
				continue
			}

			slices.Sort(sizeIDs)

			groups = append(groups, SearchGroup{
				IDs:         sizeIDs,
				SearchTerms: []string{},
				Type:        SizeAndHash,
			})
		}
	}

	// Sort groups by the path of the first file in each group
	sort.Slice(groups, func(i, j int) bool {
		var pathI, pathJ string

		if len(groups[i].IDs) > 0 {
			pathI = db.Files[groups[i].IDs[0]].Path
		}
		if len(groups[j].IDs) > 0 {
			pathJ = db.Files[groups[j].IDs[0]].Path
		}

		return pathI < pathJ
	})

	return groups
}

func (db *DB) duplicatesBySearchTerm(minLength int) []SearchGroup {
	groups := []SearchGroup{}

	for term, ids := range db.SearchTerms {
		if len(ids) < 2 {
			continue
		}

		if len(term) < minLength {
			continue
		}

		groups = append(groups, SearchGroup{
			IDs:         ids,
			SearchTerms: []string{term},
			Type:        SearchTerm,
		})
	}

	return groups
}

func (db *DB) handleDuplicateGroups(searchGroups []SearchGroup) {
	iter := 1

	for _, group := range searchGroups {
		db.output.Printf("Duplicates found: %d (%d / %d) - %s\n", len(group.IDs), iter, len(searchGroups), group.Type)

		iter++

		db.PrintIDs(group.IDs, group.SearchTerms)

		db.output.Println("Search terms: " + strings.Join(group.SearchTerms, ", "))
		db.output.Println("Delete any files? (comma separated list of numbers)")

		input := ""
		err := db.output.Scanln(&input)
		if err != nil {
			if strings.Contains(err.Error(), "unexpected newline") || err == io.EOF {
				db.output.Println("Skipping deletion.")
			} else {
				db.output.Printf("Error scanning numbers. Scanned: '%s'\n", input)
				db.output.Printf("Error: %s\n", err.Error())
			}

			db.output.Println()

			continue
		}

		if len(strings.TrimSpace(input)) == 0 {
			continue
		}

		numbers := strings.Split(input, ",")
		for _, num := range numbers {
			db.deleteFile(group.IDs, num)
		}

		db.output.Println()
	}
}

func (db *DB) deleteFile(ids []ID, num string) bool {
	index, err := strconv.Atoi(strings.TrimSpace(num))
	if err != nil {
		db.output.Printf("Invalid number: %s, err: %v, skipping...\n", err, num)

		return false
	}

	if index < 1 || index > len(ids) {
		db.output.Printf("Invalid index: %d, skipping...\n", index)

		return false
	}

	id := ids[index-1]

	db.output.Println("Deleting", id)

	delete(db.Files, id)

	err = os.Remove(string(id))
	if err != nil {
		db.output.Printf("Unable to delete file: %s, err: %v\n", id, err)

		return false
	}

	return true
}

func compareSmart(a, b Record, ignoreDirectory, ignoreCategory, ignoreDimensions bool) (float64, int) {
	if !ignoreDirectory && a.Directory == b.Directory {
		return 0.0, 0
	}

	if !ignoreCategory && a.Category != unknownCategory && b.Category != unknownCategory && a.Category != b.Category {
		return 0.0, 0
	}

	if !ignoreDimensions && a.Dimensions != unknownDimensions && b.Dimensions != unknownDimensions && a.Dimensions != b.Dimensions {
		return 0.0, 0
	}

	result := 0.0

	var j, count int
	for _, term := range a.SmartTerms {
		for ; j < len(b.SmartTerms); j++ {
			if b.SmartTerms[j] < term {
				break
			}

			count++
			if b.SmartTerms[j] == term {
				result += float64(term)
			}
		}
	}

	return result, count
}
