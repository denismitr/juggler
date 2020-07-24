package juggler

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestParseLogFileMeta(t *testing.T) {
	tt := []struct {
		name         string
		dirName      string
		ago          time.Duration
		prefix       string
		daysAgo      int
		version      int
		existingFile func(dir, prefix string, now time.Time, version int) string
		err          error
	}{
		{
			name:    "2 days ago single file",
			dirName: "create_new_file_test",
			ago:     -48 * time.Hour,
			existingFile: func(dir, prefix string, now time.Time, version int) string {
				return filepath.Join(dir, fmt.Sprintf("test_log-%s.1.log", now.Format(dateSuffix)))
			},
			version: 1,
			daysAgo: 2,
			prefix:  "test_log",
			err:     nil,
		},
		{
			name:    "1 days ago single file with version",
			dirName: "create_new_file_test",
			ago:     -24 * time.Hour,
			existingFile: func(dir, prefix string, now time.Time, version int) string {
				return filepath.Join(dir, fmt.Sprintf("test_log-%s.%d.log", now.Format(dateSuffix), version))
			},
			version: 3,
			daysAgo: 1,
			prefix:  "test_log",
			err:     nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().Add(tc.ago) // log file from some time ago
			dir := makeTestDir(tc.dirName, t)
			defer os.RemoveAll(dir)
			existingFile := tc.existingFile(dir, tc.prefix, now, tc.version)
			entry := []byte("logEntry\n")

			err := ioutil.WriteFile(existingFile, entry, 0644)
			if err != nil {
				t.Fatal(err)
			}

			ok, err := expectFileToContain(existingFile, entry)
			assert.NoError(t, err)
			assert.True(t, ok)

			fi, err := osStat(existingFile)
			if err != nil {
				t.Fatal(err)
			}

			lf, ok := parseLogFileMeta(dir, fi, tc.prefix, createFormat(tc.prefix), time.UTC)

			assert.True(t, ok, "regex could not match filename")
			assert.Equal(t, tc.version, lf.version)
			assert.Equal(t, tc.daysAgo, lf.daysAgo)
		})
	}
}

func TestParseDiffInDays(t *testing.T) {
	tt := []struct {
		in    string
		today string
		tz    *time.Location
		diff  int
		err   error
	}{
		{
			in:    "2001-10-11",
			today: "2001-10-11T15-04-05.000",
			tz:    location("UTC"),
			diff:  0,
			err:   nil,
		},
		{
			in:    "2001-10-11",
			today: "2001-10-12T11-04-05.000",
			tz:    location("UTC"),
			diff:  1,
			err:   nil,
		},
		{
			in:    "2001-10-11",
			today: "2001-10-14T00-00-01.000",
			tz:    location("UTC"),
			diff:  3,
			err:   nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.in, func(t *testing.T) {
			currentTime = func() time.Time {
				t, err := time.Parse(testTimeFormat, tc.today)
				if err != nil {
					panic(err)
				}
				return t
			}

			days, err := parseDayDiff(tc.in, tc.tz)
			if tc.err == nil {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.diff, days)
		})
	}
}

func TestScanBackups(t *testing.T) {
	t.Run("uncompressed fake storage log files", func(t *testing.T) {
		prefix := "test_log"
		f := uncompressedIdenticalTestFileFactory(prefix, "fake - log - content")

		cleanUp, dir, err := createFakeLogFiles(
			"testDir",
			f("2018-01-22", 0),
			f("2018-01-23", 0),
			f("2018-01-25", 0),
			f("2018-01-29", 0),
		)

		if err != nil {
			t.Fatal(err)
		}

		defer cleanUp()

		currentTime = func() time.Time {
			t, err := time.Parse(dateSuffix, "2018-01-30")
			if err != nil {
				panic(err)
			}
			return t
		}

		lfs, err := scanBackups(dir, prefix, createFormat(prefix), time.UTC)

		assert.NoError(t, err)
		assert.Equal(t, 4, len(lfs), "expected exactly 4 backups found")
		assert.Equal(t, 8, lfs[0].daysAgo)
		assert.Equal(t, 7, lfs[1].daysAgo)
		assert.Equal(t, 5, lfs[2].daysAgo)
		assert.Equal(t, 1, lfs[3].daysAgo)

		for _, lf := range lfs {
			assert.Equal(t, 0, lf.version)
		}
	})

	t.Run("some compressed and uncompressed fake storage log files", func(t *testing.T) {
		prefix := "test_log"
		uf := uncompressedIdenticalTestFileFactory(prefix, "uncompressed fake - log - content")
		cf := compressedIdenticalTestFileFactory(prefix, "compressed fake - log - content")

		cleanUp, dir, err := createFakeLogFiles(
			"testDir",
			cf("2018-01-20", 0),
			cf("2018-01-21", 0),
			cf("2018-01-22", 0),
			uf("2018-01-23", 0),
			uf("2018-01-25", 0),
			cf("2018-01-26", 0),
			uf("2018-01-29", 0),
		)

		if err != nil {
			t.Fatal(err)
		}

		defer cleanUp()

		currentTime = func() time.Time {
			t, err := time.Parse(dateSuffix, "2018-01-30")
			if err != nil {
				panic(err)
			}
			return t
		}

		lfs, err := scanBackups(dir, prefix, createFormat(prefix), time.UTC)

		assert.NoError(t, err)
		assert.Equal(t, 3, len(lfs), "expected exactly 4 backups found")
		assert.Equal(t, 7, lfs[0].daysAgo)
		assert.Equal(t, 5, lfs[1].daysAgo)
		assert.Equal(t, 1, lfs[2].daysAgo)

		for _, lf := range lfs {
			assert.Equal(t, 0, lf.version)
		}
	})
}

func TestCompression(t *testing.T) {
	t.Run("compress log file", func(t *testing.T) {
		prefix := "test_log"
		content := "uncompressed fake - log - content"
		uf := uncompressedIdenticalTestFileFactory(prefix, content)

		dir, err := createTestDir("test_dir")
		if err != nil {
			t.Fatal(err)
		}

		defer os.RemoveAll(dir)

		file, err := createFakeLogFile(dir, uf("2019-05-22", 0))
		if err != nil {
			t.Fatal(err)
		}

		ok, err := expectFileToContain(file, []byte(content))
		assert.NoError(t, err)
		assert.True(t, ok)

		var wg sync.WaitGroup
		errCh := make(chan error, 10)

		wg.Add(1)
		go compress(file, &wg, errCh, nil)
		wg.Wait()

		select {
			case err := <-errCh:
				t.Fatal(err)
			default:
				break
		}

		f, err := os.Open(file + ".gz")
		if err != nil {
			t.Fatal(err)
		}

		defer f.Close()

		gz, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}

		var b bytes.Buffer

		if _, err := io.Copy(&b, gz); err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, content, b.String())
		assert.NoFileExists(t, file, "old file must be deleted after compression")
		assert.FileExists(t, gzippedName(file), "compressed file must be created")
	})
}

func TestResolveFilepath(t *testing.T) {
	tt := []struct{
	    expected string
		prefix string
	    dir string
	    currentTime time.Time
	    tz *time.Location
	    currentVersion int
	}{
	    {
	        expected: "/tmp/logs/test_log-2018-01-30.1.log",
	        prefix: "test_log",
	        dir: "/tmp/logs",
	        currentTime: parseTime(dateSuffix, "2018-01-30"),
	        currentVersion: 1,
	        tz: nil,
	    },
		{
			expected: "/tmp/logs/test_log-2018-01-30.2.log",
			prefix: "test_log",
			dir: "/tmp/logs",
			currentTime: parseTime(dateSuffix, "2018-01-30"),
			currentVersion: 2,
			tz: nil,
		},
		{
			expected: "/tmp/logs/test_log-2018-01-30.2.log",
			prefix: "test_log",
			dir: "/tmp/logs",
			currentTime: parseTime(dateSuffix, "2018-01-30"),
			currentVersion: 2,
			tz: parseLocation("Europe/Moscow"),
		},
	}

	for _, tc := range tt {
	    t.Run(tc.expected, func(t *testing.T) {
			filepath := resolveFilepath(tc.prefix, tc.dir, tc.currentTime, tc.currentVersion, tc.tz)
			assert.Equal(t, tc.expected, filepath)
	    })
	}
}
