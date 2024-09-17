package main

import (
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/urfave/cli/v2"
)

const (
	scanDir    = "scanDir"
	termSearch = "termSearch"
	ts         = "ts"
	fileSearch = "fileSearch"
	fs         = "fs"
	stats      = "stats"
	s          = "s"
	duplicates = "duplicates"
	d          = "d"
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
	flagMode            = "mode"
	flagSearchMinLength = "search-min-length"
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
				Name:  scanDir,
				Usage: "Scan will scan a list of directories and store them in the DB file",
				Action: func(cCtx *cli.Context) error {
					return ScanCommand(
						output,
						cCtx.Args().Get(0),
						cCtx.Args().Slice()[1:],
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

func ScanCommand(output Output, dbFile string, roots []string) error {
	db := NewDB(output, dbFile)

	db.Load()

	err := db.Scan(roots...)
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

	db.Load()

	db.Search(modeFlag, searchTerms)

	return nil
}

func FileSearchCommand(output Output, dbFile, modeFlag, filePath string) error {
	db := NewDB(output, dbFile)

	db.Load()

	searchTerms := pathToSearchTerms(filePath)

	db.Search(modeFlag, searchTerms)

	return nil
}

func DuplicateCommand(output Output, dbFile string, searchMinLength int) error {
	db := NewDB(output, dbFile)

	db.Load()

	db.Duplicates(searchMinLength)

	err := db.Write()
	if err != nil {
		output.Printf("Error writing DB: %v\n", err)
		output.Exit(1)
	}

	return nil
}

func StatsCommand(output Output, dbFile string, searchMinLength int) error {
	db := NewDB(output, dbFile)

	db.Load()

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
	_, err := fmt.Scanln(&a)
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
	Size        int
	Hash        string
	SearchTerms []string
}

type ID string

type DB struct {
	mutex       *sync.RWMutex
	Files       map[ID]Record
	Sizes       map[int][]ID
	Hashes      map[string][]ID
	SearchTerms map[string][]ID
	output      Output
	dbFile      string
}

func NewDB(output Output, dbFile string) *DB {
	return &DB{
		mutex:       &sync.RWMutex{},
		Files:       make(map[ID]Record),
		Sizes:       make(map[int][]ID),
		Hashes:      make(map[string][]ID),
		SearchTerms: make(map[string][]ID),
		output:      output,
		dbFile:      dbFile,
	}
}

func (db *DB) Load() {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	records, err := readCsvFile(db.dbFile)
	if err != nil {
		db.output.Printf("Unable to read DB file '%s', error: %v", db.dbFile, err)

		db.output.Exit(1)
	}

	for _, record := range records {
		db.handleRecord(record)
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

func (db *DB) handleRecord(record []string) {
	filePath := record[0]

	size, err := strconv.Atoi(record[1])
	if err != nil {
		db.output.Println("Unable to parse size from record. File path:", record[0], "Raw data:", record[1], ", error:", err.Error())

		return
	}

	hash := record[2]

	searchTerms := pathToSearchTerms(filePath)

	err = db.add(filePath, size, hash, searchTerms)
	if err != nil {
		db.output.Println("Unable to add record to DB, file path:", filePath, ", error:", err.Error())
	}
}

func (db *DB) Scan(roots ...string) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	for _, root := range roots {
		files, err := collectFiles(root)
		if err != nil {
			return fmt.Errorf("unable to collect files in root %s, err: %w", root, err)
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

			continue
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

	err = db.add(filename, int(size), hash, searchTerms)
	if err != nil {
		return fmt.Errorf("unable to add record to DB, file path: %s, err: %w", filename, err)
	}

	return nil
}

func (db *DB) add(filePath string, size int, hash string, searchTerms []string) error {
	id := ID(filePath)

	db.Files[id] = Record{Path: filePath, Size: size, Hash: hash, SearchTerms: searchTerms}
	db.Sizes[size] = append(db.Sizes[size], id)
	for _, term := range searchTerms {
		db.SearchTerms[term] = append(db.SearchTerms[term], id)
	}
	db.Hashes[hash] = append(db.Hashes[hash], id)

	return nil
}

func (db *DB) Write() error {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	// write CSV file from db.Files
	file, err := os.Create(db.dbFile)
	if err != nil {
		return fmt.Errorf("unable to create DB file %s, err: %w", db.dbFile, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, record := range db.Files {
		record := []string{record.Path, strconv.Itoa(record.Size), record.Hash}
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
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	var allIDs [][]ID

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

	for _, needle := range searchedTerms {
		termIDs, ok := db.SearchTerms[needle]
		if !ok {
			db.output.Printf("No results found for needle '%s'\n", needle)

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

	for _, needle := range searchedTerms {
		var found []ID
		for term, ids := range db.SearchTerms {
			if strings.Contains(term, needle) {
				found = append(found, ids...)
			}
		}

		if len(found) == 0 {
			db.output.Printf("No results found for needle '%s'\n", needle)

			return nil
		}

		results = append(results, found)
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

func (db *DB) Stats(minLength int) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

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
	db.mutex.Lock()
	defer db.mutex.Unlock()

	db.duplicatesBySizeAndHash()

	db.duplicatesBySearchTerm(minLength)
}

type SearchType string

const (
	SizeAndHash SearchType = "Size and hash"
	SearchTerm  SearchType = "Search term"
)

type SearchGroup struct {
	IDs         []ID
	SearchTerms []string
	Type        SearchType
}

func (db *DB) duplicatesBySizeAndHash() {
	groups := make(map[string]SearchGroup)

	for hash, ids := range db.Hashes {
		if len(ids) < 2 {
			continue
		}

		sizes := make(map[int][]ID)
		for _, id := range ids {
			size := db.Files[id].Size
			sizes[size] = append(sizes[size], id)
		}

		for size, sizeIDs := range sizes {
			groupID := fmt.Sprintf("%s-%d", hash, size)

			slices.Sort(sizeIDs)

			groups[groupID] = SearchGroup{
				IDs:         sizeIDs,
				SearchTerms: []string{},
				Type:        SizeAndHash,
			}
		}
	}

	db.handleDuplicateGroups(groups)
}

func (db *DB) duplicatesBySearchTerm(minLength int) {
	groups := make(map[string]SearchGroup)

	for term, ids := range db.SearchTerms {
		if len(ids) < 2 {
			continue
		}

		if len(term) < minLength {
			continue
		}

		groups[term] = SearchGroup{
			IDs:         ids,
			SearchTerms: []string{term},
			Type:        SearchTerm,
		}
	}

	db.handleDuplicateGroups(groups)
}

func (db *DB) handleDuplicateGroups(searchGroups map[string]SearchGroup) {
	input := ""
	iter := 1

	for _, group := range searchGroups {
		db.output.Printf("Duplicates found: %d (%d / %d) - %s\n", len(group.IDs), iter, len(searchGroups), group.Type)

		iter++

		db.PrintIDs(group.IDs, group.SearchTerms)

		db.output.Println("Delete any files? (comma separated list of numbers)")

		err := db.output.Scanln(&input)
		if err != nil {
			db.output.Println("Error scanning numbers. Scanned:", input)
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
