package main

import (
	"fmt"
	rand "math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestOutput struct {
	t     *testing.T
	data  []string
	input []string
	count int
}

func (out *TestOutput) Println(a ...any) {
	str := fmt.Sprintln(a...)

	out.data = append(out.data, str)
}

func (out *TestOutput) Printf(format string, a ...any) {
	str := fmt.Sprintf(format, a...)

	out.data = append(out.data, str)
}

func (out *TestOutput) Scanln(a *string) error {
	if out.count >= len(out.input) {
		*a = ""

		return nil
	}

	*a = out.input[out.count]

	out.count++

	return nil
}

func (out *TestOutput) Exit(_ int) {
	out.t.SkipNow()
}

func (out *TestOutput) Get(idx int) string {
	if len(out.data) <= idx {
		return ""
	}

	return out.data[idx]
}

func (out *TestOutput) String() string {
	return strings.Join(out.data, "\n")
}

func NewTestOutput(t *testing.T, input []string) *TestOutput {
	t.Helper()

	return &TestOutput{
		t:     t,
		data:  []string{},
		input: input,
		count: 0,
	}
}

func TestApp_Scan_and_Stats(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (string, []string) {
		t.Helper()

		random := fmt.Sprintf("%f", rand.ExpFloat64())

		dbFile := fmt.Sprintf("_test_%s.csv", random)
		err := os.WriteFile(dbFile, nil, 0o644)
		require.NoError(t, err)

		dirName1 := fmt.Sprintf("_fs1_%s", random)
		dirName2 := fmt.Sprintf("_fs2_%s", random)

		err = os.Mkdir(dirName1, 0o777)
		require.NoError(t, err)

		err = os.Mkdir(dirName2, 0o777)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(dirName1, "duplicate.txt"), []byte(random), 0o644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(dirName1, "bar.txt"), []byte("Bar"), 0o644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(dirName2, "duplicate.txt"), []byte(random), 0o644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(dirName2, "bar.txt"), []byte("bar"), 0o644)
		require.NoError(t, err)

		return dbFile, []string{dirName1, dirName2}
	}

	removeDir := func(t *testing.T, dirName string) {
		t.Helper()

		var err error

		for _, fileName := range []string{"duplicate.txt", "bar.txt"} {
			err = os.Remove(filepath.Join(dirName, fileName))
			require.NoError(t, err)
		}

		err = os.Remove(dirName)
		require.NoError(t, err)
	}

	cleanup := func(t *testing.T, dbFile string, dirNames []string) {
		t.Helper()

		err := os.Remove(dbFile)
		require.NoError(t, err)

		for _, dirName := range dirNames {
			removeDir(t, dirName)
		}
	}

	t.Run("success - scan and stat", func(t *testing.T) {
		t.Parallel()

		dbFile, dirNames := setup(t)
		defer cleanup(t, dbFile, dirNames)

		// setup
		output := NewTestOutput(t, nil)

		// execute
		// - scan directories
		err := ScanCommand(output, dbFile, dirNames, false)
		require.NoError(t, err)

		// - stat
		err = StatsCommand(output, dbFile, defaultMinLength)
		require.NoError(t, err)

		// verify
		// - scan dir
		assert.Equal(t, fmt.Sprintf("root: %s, 2 found files, 0 skipped, 2 created, 0 deleted\n", dirNames[0]), output.Get(0))
		assert.Equal(t, fmt.Sprintf("root: %s, 2 found files, 0 skipped, 2 created, 0 deleted\n", dirNames[1]), output.Get(1))

		// - stats
		assert.Equal(t, "Total records: 4\n", output.Get(2))
		assert.Equal(t, "Total unique sizes: 2\n", output.Get(3))
		assert.Equal(t, "Total unique search terms: 2\n", output.Get(4))
		assert.Equal(t, "Total unique hashes: 3\n", output.Get(5))
		assert.Equal(t, "Sizes with multiple records: 2\n", output.Get(6))
		assert.Equal(t, "Hashes with multiple records: 1\n", output.Get(7))
	})

	t.Run("success - scan /w forceWrite, rescan and stat", func(t *testing.T) {
		t.Parallel()

		dbFile, dirNames := setup(t)
		defer cleanup(t, dbFile, dirNames[1:])

		// setup
		output := NewTestOutput(t, nil)

		// execute
		// - scan directories
		err := ScanCommand(output, dbFile, dirNames, true)
		require.NoError(t, err)

		// delete directory
		removeDir(t, dirNames[0])

		// execute 2
		dbFile2, dirNames2 := setup(t)
		defer cleanup(t, dbFile2, dirNames2)

		// - scan directories
		err = ScanCommand(output, dbFile, []string{dirNames[0], dirNames2[0], dirNames2[1]}, true)
		require.NoError(t, err)

		// - stat
		err = StatsCommand(output, dbFile, defaultMinLength)
		require.NoError(t, err)

		// verify
		// - scan dir
		assert.Equal(t, fmt.Sprintf("root: %s, 2 found files, 0 skipped, 2 created, 0 deleted\n", dirNames[0]), output.Get(0))
		assert.Equal(t, fmt.Sprintf("root: %s, 2 found files, 0 skipped, 2 created, 0 deleted\n", dirNames[1]), output.Get(1))
		assert.Equal(t, fmt.Sprintf("root: %s, 0 found files, 0 skipped, 0 created, 2 deleted\n", dirNames[0]), output.Get(2))
		assert.Equal(t, fmt.Sprintf("root: %s, 2 found files, 0 skipped, 2 created, 0 deleted\n", dirNames2[0]), output.Get(3))
		assert.Equal(t, fmt.Sprintf("root: %s, 2 found files, 0 skipped, 2 created, 0 deleted\n", dirNames2[1]), output.Get(4))

		// - stats
		// assert.Equal(t, "Total records: 6\n", output.Get(5))
		assert.Equal(t, "Total unique sizes: 2\n", output.Get(6))
		assert.Equal(t, "Total unique search terms: 2\n", output.Get(7))
		assert.Equal(t, "Total unique hashes: 4\n", output.Get(8))
		assert.Equal(t, "Sizes with multiple records: 2\n", output.Get(9))
		assert.Equal(t, "Hashes with multiple records: 2\n", output.Get(10))
	})

	t.Run("success - scan /wo forceWrite, rescan and stat", func(t *testing.T) {
		t.Parallel()

		dbFile, dirNames := setup(t)
		defer cleanup(t, dbFile, dirNames[1:])

		// setup
		output := NewTestOutput(t, nil)

		// execute
		// - scan directories
		err := ScanCommand(output, dbFile, dirNames, false)
		require.NoError(t, err)

		// delete directory
		removeDir(t, dirNames[0])

		// execute 2
		dbFile2, dirNames2 := setup(t)
		defer cleanup(t, dbFile2, dirNames2)

		// - scan directories
		err = ScanCommand(output, dbFile, []string{dirNames[0], dirNames2[0], dirNames2[1]}, false)
		require.NoError(t, err)

		// - stat
		err = StatsCommand(output, dbFile, defaultMinLength)
		require.NoError(t, err)

		// verify
		// - scan dir
		assert.Equal(t, fmt.Sprintf("root: %s, 2 found files, 0 skipped, 2 created, 0 deleted\n", dirNames[0]), output.Get(0))
		assert.Equal(t, fmt.Sprintf("root: %s, 2 found files, 0 skipped, 2 created, 0 deleted\n", dirNames[1]), output.Get(1))
		assert.Equal(t, fmt.Sprintf("root: %s, 2 found files, 0 skipped, 2 created, 0 deleted\n", dirNames2[0]), output.Get(2))
		assert.Equal(t, fmt.Sprintf("root: %s, 2 found files, 0 skipped, 2 created, 0 deleted\n", dirNames2[1]), output.Get(3))

		// - stats
		assert.Equal(t, "Total records: 8\n", output.Get(4))
		assert.Equal(t, "Total unique sizes: 2\n", output.Get(5))
		assert.Equal(t, "Total unique search terms: 2\n", output.Get(6))
		assert.Equal(t, "Total unique hashes: 4\n", output.Get(7))
		assert.Equal(t, "Sizes with multiple records: 2\n", output.Get(8))
		assert.Equal(t, "Hashes with multiple records: 4\n", output.Get(9))
	})
}

func TestApp_Duplicates(t *testing.T) {
	t.Parallel()

	files := []string{
		"bar-1786396036.txt",
		"bambam/foo-756381984.txt",
		"bambam/baz.txt",
		"bambam/quix-1786396036.txt",
	}

	setup := func(t *testing.T) string {
		t.Helper()

		random := fmt.Sprintf("%f", rand.ExpFloat64())

		lines := []string{
			fmt.Sprintf("%s,756381984,464f1ce84fed3d6837db4b810462f8de\n", files[0]),
			fmt.Sprintf("%s,1786396036,4d09a656f20fee1beb093f30c7ec504c\n", files[1]),
			fmt.Sprintf("%s,756381984,464f1ce84fed3d6837db4b810462f8de\n", files[2]),
			fmt.Sprintf("%s,123,788b62828f73d4bac70088ea91c90ef5\n", files[3]),
		}

		dbFile := fmt.Sprintf("_test_%s.csv", random)

		err := os.WriteFile(dbFile, []byte(strings.Join(lines, "\n")), 0o644)
		require.NoError(t, err)

		return dbFile
	}

	cleanup := func(t *testing.T, dbFile string) {
		t.Helper()

		err := os.Remove(dbFile)
		require.NoError(t, err)
	}

	cleanColor := func(t *testing.T, str string) string {
		t.Helper()

		str = strings.ReplaceAll(str, redBold, "")
		str = strings.ReplaceAll(str, yellowBold, "")
		str = strings.ReplaceAll(str, blueBold, "")
		str = strings.ReplaceAll(str, reset, "")

		return str
	}

	t.Run("success finding duplicates by size and hashes", func(t *testing.T) {
		t.Parallel()

		dbFile := setup(t)
		defer cleanup(t, dbFile)

		// setup
		output := NewTestOutput(t, nil)

		// execute
		err := DuplicateCommand(output, dbFile, defaultMinLength)
		require.NoError(t, err)

		// verify
		assert.Equal(t, "Duplicates found: 2 (1 / 1) - Size and hash\n", output.Get(0))
		assert.Contains(t, output.Get(1), files[2])
		assert.Contains(t, output.Get(2), files[0])
	})

	t.Run("success finding duplicates by partial name match", func(t *testing.T) {
		t.Parallel()

		dbFile := setup(t)
		defer cleanup(t, dbFile)

		// data
		const reducedSearchMinLength = 10

		// setup
		output := NewTestOutput(t, nil)

		// execute
		err := DuplicateCommand(output, dbFile, reducedSearchMinLength)
		require.NoError(t, err)

		// verify
		assert.Equal(t, "Duplicates found: 2 (1 / 1) - Search term\n", output.Get(5))
		assert.Contains(t, cleanColor(t, output.Get(6)), files[3])
		assert.Contains(t, cleanColor(t, output.Get(7)), files[0])
	})

	t.Run("failure deleting non-existent file", func(t *testing.T) {
		t.Parallel()

		dbFile := setup(t)
		defer cleanup(t, dbFile)

		// setup
		output := NewTestOutput(t, []string{"1"})

		// execute
		err := DuplicateCommand(output, dbFile, defaultMinLength)
		require.NoError(t, err)

		// verify
		assert.Equal(t, "Duplicates found: 2 (1 / 1) - Size and hash\n", output.Get(0))
		assert.Contains(t, output.Get(1), files[2])
		assert.Contains(t, output.Get(2), files[0])
	})

	t.Run("success finding and deleting duplicates by size and hashes", func(t *testing.T) {
		t.Parallel()

		dbFile := setup(t)
		defer cleanup(t, dbFile)

		// setup
		err := os.WriteFile(files[0], nil, 0o644)
		require.NoError(t, err)

		output := NewTestOutput(t, []string{"2"})

		// execute
		err = DuplicateCommand(output, dbFile, defaultMinLength)
		require.NoError(t, err)

		// verify
		assert.Equal(t, "Duplicates found: 2 (1 / 1) - Size and hash\n", output.Get(0))
		assert.Contains(t, output.Get(1), files[2])
		assert.Contains(t, output.Get(2), files[0])
		assert.Equal(t, "Delete any files? (comma separated list of numbers)\n", output.Get(4))
		assert.Contains(t, output.Get(5), "Deleting")
		assert.Contains(t, output.Get(5), files[0])

		assert.NoFileExists(t, files[2])
	})
}

func TestApp_Search(t *testing.T) {
	t.Parallel()

	files := []string{
		"bambam/foo-756381984.txt",
		"bambam/bar-1786396036.txt",
		"bambam/baz.txt",
		"bambam/quix-1786396036.txt",
	}

	setup := func(t *testing.T) string {
		t.Helper()

		random := fmt.Sprintf("%f", rand.ExpFloat64())

		lines := []string{
			fmt.Sprintf("%s,756381984,464f1ce84fed3d6837db4b810462f8de\n", files[0]),
			fmt.Sprintf("%s,1786396036,4d09a656f20fee1beb093f30c7ec504c\n", files[1]),
			fmt.Sprintf("%s,756381984,464f1ce84fed3d6837db4b810462f8de\n", files[2]),
			fmt.Sprintf("%s,123,788b62828f73d4bac70088ea91c90ef5\n", files[3]),
		}

		dbFile := fmt.Sprintf("_test_%s.csv", random)

		err := os.WriteFile(dbFile, []byte(strings.Join(lines, "\n")), 0o644)
		require.NoError(t, err)

		return dbFile
	}

	cleanup := func(t *testing.T, dbFile string) {
		t.Helper()

		err := os.Remove(dbFile)
		require.NoError(t, err)
	}

	clearColors := func(t *testing.T, str string) string {
		t.Helper()

		str = strings.ReplaceAll(str, redBold, "")
		str = strings.ReplaceAll(str, yellowBold, "")
		str = strings.ReplaceAll(str, blueBold, "")
		str = strings.ReplaceAll(str, reset, "")

		return str
	}

	t.Run("success searching by term - fast", func(t *testing.T) {
		t.Parallel()

		dbFile := setup(t)
		defer cleanup(t, dbFile)

		// setup
		output := NewTestOutput(t, nil)

		// execute
		err := TermSearchCommand(output, dbFile, fast, []string{"1786396036.txt"})
		require.NoError(t, err)

		// verify
		assert.Contains(t, clearColors(t, output.Get(0)), files[1])
		assert.Contains(t, clearColors(t, output.Get(1)), files[3])
	})

	t.Run("failure searching by term - fast", func(t *testing.T) {
		t.Parallel()

		dbFile := setup(t)
		defer cleanup(t, dbFile)

		// setup
		output := NewTestOutput(t, nil)

		// execute
		err := TermSearchCommand(output, dbFile, fast, []string{"1786396036"})
		require.NoError(t, err)

		// verify
		assert.Equal(t, "No results found for search term '1786396036'.\n", output.Get(0))
	})

	t.Run("failure searching by term - slow", func(t *testing.T) {
		t.Parallel()

		dbFile := setup(t)
		defer cleanup(t, dbFile)

		// setup
		output := NewTestOutput(t, nil)

		// execute
		err := TermSearchCommand(output, dbFile, slow, []string{"abcde"})
		require.NoError(t, err)

		// verify
		assert.Equal(t, "No results found for search term 'abcde'.\n", output.Get(0))
	})

	t.Run("success searching by term", func(t *testing.T) {
		t.Parallel()

		dbFile := setup(t)
		defer cleanup(t, dbFile)

		// setup
		output := NewTestOutput(t, nil)

		// execute
		err := TermSearchCommand(output, dbFile, slow, []string{"1786396036"})
		require.NoError(t, err)

		// verify
		assert.Contains(t, clearColors(t, output.Get(0)), files[1])
		assert.Contains(t, clearColors(t, output.Get(1)), files[3])
	})

	t.Run("success searching by multiple terms", func(t *testing.T) {
		t.Parallel()

		dbFile := setup(t)
		defer cleanup(t, dbFile)

		// setup
		output := NewTestOutput(t, nil)

		// execute
		err := TermSearchCommand(output, dbFile, slow, []string{"bar", "1786396036"})
		require.NoError(t, err)

		// verify
		assert.Contains(t, clearColors(t, output.Get(0)), files[1])
		assert.Empty(t, output.Get(1))
	})

	t.Run("success searching by file", func(t *testing.T) {
		t.Parallel()

		dbFile := setup(t)
		defer cleanup(t, dbFile)

		// setup
		output := NewTestOutput(t, nil)

		// execute
		err := FileSearchCommand(output, dbFile, slow, files[1])
		require.NoError(t, err)

		// verify
		assert.Contains(t, clearColors(t, output.Get(0)), files[1])
		assert.Empty(t, output.Get(1))
	})
}

func Test_FindHighlights(t *testing.T) {
	t.Parallel()

	type args struct {
		haystack string
		needles  []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "basic",
			args: args{
				haystack: "hello world",
				needles:  []string{"world"},
			},
			want: "hello \033[1m\033[31mworld\033[0m",
		},
		{
			name: "advanced",
			args: args{
				haystack: "hello world, hello peter",
				needles:  []string{"world", "hello"},
			},
			want: "\033[1m\033[31mhello\033[0m \033[1m\033[31mworld\033[0m, hello peter",
		},
		{
			name: "very advanced",
			args: args{
				haystack: "hElLo World, hello peter",
				needles:  []string{"world", "hello"},
			},
			want: "\033[1m\033[31mhElLo\033[0m \033[1m\033[31mWorld\033[0m, hello peter",
		},
		{
			name: "skip overlaps",
			args: args{
				haystack: "Foobar",
				needles:  []string{"foo", "oba"},
			},
			want: "\x1b[1m\x1b[43mFoobar\x1b[0m",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// execute
			got := FindHighlights(tt.args.haystack, tt.args.needles)

			// verify
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_getSmartTerms(t *testing.T) {
	t.Parallel()

	type args struct {
		haystack string
	}
	tests := []struct {
		name  string
		args  args
		want1 []uint64
		want2 string
		want3 string
	}{
		{
			name: "without anything",
			args: args{
				haystack: "hello world",
			},
			want1: []uint64{1 << 30},
			want2: unknownCategory,
			want3: unknownDimensions,
		},
		{
			name: "with dimensions",
			args: args{
				haystack: "hello world-325x233",
			},
			want1: []uint64{1 << 30},
			want2: unknownCategory,
			want3: "325x233",
		},
		{
			name: "with well-known",
			args: args{
				haystack: "helloworld-fullhd-1080p",
			},
			want1: []uint64{1 << 30},
			want2: unknownCategory,
			want3: "fullhd-1080p",
		},
		{
			name: "with well-known and description",
			args: args{
				haystack: "hello world-2k-1080p",
			},
			want1: []uint64{1 << 30},
			want2: unknownCategory,
			want3: "2k-1080p",
		},
		{
			name: "with dimensions and description",
			args: args{
				haystack: "hello world-960x350-1ffc",
			},
			want1: []uint64{1 << 30},
			want2: unknownCategory,
			want3: "960x350",
		},
		{
			name: "with dimensions and description, random-case",
			args: args{
				haystack: "heLLo wOrld-960x350-1ffc",
			},
			want1: []uint64{1 << 30},
			want2: unknownCategory,
			want3: "960x350",
		},
		{
			name: "with dimensions and category, random-case",
			args: args{
				haystack: "heLLo wOrld-960x350-1ffc-foo-bar-baz",
			},
			want1: []uint64{1 << 30},
			want2: "baz",
			want3: "960x350",
		},
		{
			name: "with dimensions and category, random-case, phone",
			args: args{
				haystack: "heLLo wOrld-960x350-1ffc-foo-phone",
			},
			want1: []uint64{1 << 30},
			want2: "foo",
			want3: "960x350",
		},
		{
			name: "with dimensions and category, random-case, low quality, cleaned up category",
			args: args{
				haystack: "heLLo wOrld-960x350-1ffc-full-low",
			},
			want1: []uint64{1 << 30},
			want2: unknownCategory,
			want3: "960x350",
		},
		{
			name: "with dimensions and category, random-case, phone, low quality",
			args: args{
				haystack: "heLLo wOrld-960x350-1ffc-foo-phone-low",
			},
			want1: []uint64{1 << 30},
			want2: "foo",
			want3: "960x350",
		},
		{
			name: "with dimensions and category, random-case, low quality, phone",
			args: args{
				haystack: "heLLo wOrld-960x350-1ffc-foo-low-phone",
			},
			want1: []uint64{1 << 30},
			want2: "foo",
			want3: "960x350",
		},
		{
			name: "with prefixes, dimensions and category, random-case, foo category, low quality, phone",
			args: args{
				haystack: "foobar-bazbar-quiz-heLLo wOrld-960x350-1ffc-foo-low-phone",
			},
			want1: []uint64{1<<30 + 3, 1<<26 + 1, 1 << 26, 1<<24 + 2},
			want2: "foo",
			want3: "960x350",
		},
		{
			name: "with prefixes, dimensions and category, random-case, foo category 3, low quality, phone",
			args: args{
				haystack: "foobar-bazbar-quiz-heLLo wOrld-960x350-1ffc-foo3-low-phone",
			},
			want1: []uint64{1<<30 + 3, 1<<26 + 1, 1 << 26, 1<<24 + 2},
			want2: "foo",
			want3: "960x350",
		},
		{
			name: "with prefixes, dimensions and category, random-case, low quality, unkonwn category, phone",
			args: args{
				haystack: "foobar-bazbar-quiz-heLLo wOrld-960x350-1ffc-foo-full2-low-phone",
			},
			want1: []uint64{1<<30 + 3, 1<<26 + 1, 1 << 26, 1<<24 + 2},
			want2: unknownCategory,
			want3: "960x350",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// prepare
			sdb := NewSmartDB()

			// execute
			got1, got2, got3, err := sdb.ProcessPath(tt.args.haystack)
			require.NoError(t, err)

			// verify
			assert.Equal(t, tt.want1, got1)
			assert.Equal(t, tt.want2, got2)
			assert.Equal(t, tt.want3, got3)
		})
	}
}

func Test_getSmartNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		haystacks []string
		wants     []uint64
	}{
		{
			name:      "empty",
			haystacks: []string{""},
			wants:     []uint64{1 << 20},
		},
		{
			name:      "foo",
			haystacks: []string{"foo"},
			wants:     []uint64{1 << 23},
		},
		{
			name:      "foo + bar",
			haystacks: []string{"foo", "bar"},
			wants:     []uint64{1 << 23, 1<<23 + 1},
		},
		{
			name:      "foo + quix",
			haystacks: []string{"foo", "quix"},
			wants:     []uint64{1 << 23, 1<<24 + 1},
		},
		{
			name:      "long",
			haystacks: []string{"abcdefghijklmnopqrstuvwxyabcdefghijklmnopqrstuvwxyzzabcdefghijklmnopqrstuvwxyz"},
			wants:     []uint64{1 << 62},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Len(t, tt.haystacks, len(tt.wants))

			sdb := NewSmartDB()

			for i, haystack := range tt.haystacks {
				// execute
				got := sdb.GetValue(haystack)

				// verify
				assert.Equal(t, tt.wants[i], got)
			}
		})
	}
}

func Test_compareSmart(t *testing.T) {
	t.Parallel()

	type args struct {
		a, b             Record
		ignoreDirectory  bool
		ignoreCategory   bool
		ignoreDimensions bool
	}
	tests := []struct {
		name  string
		args  args
		want1 float64
		want2 int
	}{
		{
			name: "miss",
			args: args{
				a: Record{
					SmartTerms: []uint64{100},
				},
				b: Record{
					SmartTerms: []uint64{99},
				},
				ignoreDirectory:  true,
				ignoreCategory:   false,
				ignoreDimensions: false,
			},
			want1: 0.0,
			want2: 0,
		},
		{
			name: "single and simple hit",
			args: args{
				a: Record{
					SmartTerms: []uint64{100},
				},
				b: Record{
					SmartTerms: []uint64{100},
				},
				ignoreDirectory:  true,
				ignoreCategory:   false,
				ignoreDimensions: false,
			},
			want1: 100.0,
			want2: 1,
		},
		{
			name: "single hit, complex",
			args: args{
				a: Record{
					SmartTerms: []uint64{102, 101, 100, 75, 45},
				},
				b: Record{
					SmartTerms: []uint64{200, 150, 140, 100, 77, 23},
				},
				ignoreDirectory:  true,
				ignoreCategory:   false,
				ignoreDimensions: false,
			},
			want1: 100.0,
			want2: 5,
		},
		{
			name: "multi hit, complex",
			args: args{
				a: Record{
					SmartTerms: []uint64{102, 101, 100, 75, 45},
				},
				b: Record{
					SmartTerms: []uint64{200, 150, 140, 100, 75, 23},
				},
				ignoreDirectory:  true,
				ignoreCategory:   false,
				ignoreDimensions: false,
			},
			want1: 175.0,
			want2: 5,
		},
		{
			name: "category mismatch",
			args: args{
				a: Record{
					Directory:  "bar/baz",
					SmartTerms: []uint64{102, 101, 100, 75, 45},
					Category:   "foo",
					Dimensions: "fullhd-1080p",
				},
				b: Record{
					Directory:  "bar/quix",
					SmartTerms: []uint64{200, 150, 140, 100, 75, 23},
					Category:   "bar",
					Dimensions: "fullhd-1080p",
				},
				ignoreDirectory:  false,
				ignoreCategory:   false,
				ignoreDimensions: true,
			},
			want1: 0.0,
			want2: 0,
		},
		{
			name: "dimension mismatch",
			args: args{
				a: Record{
					Directory:  "bar/baz",
					SmartTerms: []uint64{102, 101, 100, 75, 45},
					Category:   "foo",
					Dimensions: "fullhd-1080p",
				},
				b: Record{
					Directory:  "bar/quix",
					SmartTerms: []uint64{200, 150, 140, 100, 75, 23},
					Category:   "foo",
					Dimensions: "320x240",
				},
				ignoreDirectory:  false,
				ignoreCategory:   true,
				ignoreDimensions: false,
			},
			want1: 0.0,
			want2: 0,
		},
		{
			name: "directory match",
			args: args{
				a: Record{
					Directory:  "bar/baz",
					SmartTerms: []uint64{102, 101, 100, 75, 45},
					Category:   "foo",
					Dimensions: "fullhd-1080p",
				},
				b: Record{
					Directory:  "bar/baz",
					SmartTerms: []uint64{200, 150, 140, 100, 75, 23},
					Category:   "foo",
					Dimensions: "320x240",
				},
				ignoreDirectory:  true,
				ignoreCategory:   false,
				ignoreDimensions: false,
			},
			want1: 0.0,
			want2: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// execute
			got1, got2 := compareSmart(tt.args.a, tt.args.b, tt.args.ignoreDirectory, tt.args.ignoreCategory, tt.args.ignoreDimensions)

			// verify
			assert.Less(t, tt.want1-got1, 0.1)
			assert.Greater(t, tt.want1-got1, -0.1)
			assert.Equal(t, tt.want2, got2)
		})
	}
}

func Test_cleanSmart(t *testing.T) {
	t.Parallel()

	type args struct {
		m map[float64][][2]ID
	}
	tests := []struct {
		name            string
		args            args
		expectedMap     map[float64][][2]ID
		expectedMinimum float64
	}{
		{
			name: "small",
			args: args{
				m: map[float64][][2]ID{
					2.0: {
						{"foo", "bar"},
						{"foo", "quix"},
					},
				},
			},
			expectedMap: map[float64][][2]ID{
				2.0: {
					{"foo", "bar"},
					{"foo", "quix"},
				},
			},
			expectedMinimum: 2.0,
		},
		{
			name: "keep 1",
			args: args{
				m: map[float64][][2]ID{
					2.0: {
						{"foo", "bar"},
						{"foo", "quix"},
						{"foo", "sorp"},
						{"bar", "foo"},
						{"bar", "baz"},
						{"bar", "quix"},
						{"quix", "farq"},
						{"bar", "farq"},
						{"farq", "quix"},
						{"sorp", "quix"},
						{"sorp", "baz"},
						{"yapp", "baz"},
					},
					1.0: {
						{"foo", "baz"},
						{"yapp", "foo"},
					},
				},
			},
			expectedMap: map[float64][][2]ID{
				2.0: {
					{"foo", "bar"},
					{"foo", "quix"},
					{"foo", "sorp"},
					{"bar", "foo"},
					{"bar", "baz"},
					{"bar", "quix"},
					{"quix", "farq"},
					{"bar", "farq"},
					{"farq", "quix"},
					{"sorp", "quix"},
				},
			},
			expectedMinimum: 2.0,
		},
		{
			name: "keep 2",
			args: args{
				m: map[float64][][2]ID{
					2.0: {
						{"foo", "bar"},
						{"foo", "quix"},
						{"foo", "sorp"},
						{"bar", "foo"},
						{"bar", "baz"},
						{"bar", "quix"},
						{"quix", "farq"},
						{"bar", "farq"},
					},
					1.0: {
						{"foo", "baz"},
						{"yapp", "foo"},
						{"farq", "quix"},
						{"sorp", "quix"},
						{"sorp", "baz"},
						{"yapp", "baz"},
					},
				},
			},
			expectedMap: map[float64][][2]ID{
				2.0: {
					{"foo", "bar"},
					{"foo", "quix"},
					{"foo", "sorp"},
					{"bar", "foo"},
					{"bar", "baz"},
					{"bar", "quix"},
					{"quix", "farq"},
					{"bar", "farq"},
				},
				1.0: {
					{"foo", "baz"},
					{"yapp", "foo"},
				},
			},
			expectedMinimum: 1.0,
		},
		{
			name: "keep 10",
			args: args{
				m: map[float64][][2]ID{
					1.0: {
						{"foo", "bar"},
					},
					2.0: {
						{"foo", "quix"},
					},
					3.0: {
						{"foo", "sorp"},
					},
					4.0: {
						{"bar", "foo"},
					},
					5.0: {
						{"bar", "baz"},
					},
					6.0: {
						{"bar", "quix"},
					},
					7.0: {
						{"quix", "farq"},
					},
					8.0: {
						{"bar", "farq"},
					},
					9.0: {
						{"foo", "baz"},
					},
					10.0: {
						{"yapp", "foo"},
					},
					11.0: {
						{"farq", "quix"},
					},
					12.0: {
						{"sorp", "quix"},
					},
					13.0: {
						{"sorp", "baz"},
					},
					14.0: {
						{"yapp", "baz"},
					},
				},
			},
			expectedMap: map[float64][][2]ID{
				5.0: {
					{"bar", "baz"},
				},
				6.0: {
					{"bar", "quix"},
				},
				7.0: {
					{"quix", "farq"},
				},
				8.0: {
					{"bar", "farq"},
				},
				9.0: {
					{"foo", "baz"},
				},
				10.0: {
					{"yapp", "foo"},
				},
				11.0: {
					{"farq", "quix"},
				},
				12.0: {
					{"sorp", "quix"},
				},
				13.0: {
					{"sorp", "baz"},
				},
				14.0: {
					{"yapp", "baz"},
				},
			},
			expectedMinimum: 5.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// execute
			actualMap, actualMinimum := cleanSmart(tt.args.m)

			// verify
			assert.Equal(t, tt.expectedMap, actualMap)
			assert.Equal(t, tt.expectedMinimum, actualMinimum)
		})
	}
}
