package juggler

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseFile(t *testing.T) {
	tt := []struct{
	    name string
	    dirName string
	    ago time.Duration
	    prefix string
	    daysAgo int
	    version int
	    existingFile func(dir, prefix string, now time.Time, version int) string
	    err error
	}{
	    {
	        name: "2 days ago single file",
	        dirName: "create_new_file_test",
	        ago: -48 * time.Hour,
	        existingFile: func(dir, prefix string, now time.Time, version int) string {
	        	return filepath.Join(dir, fmt.Sprintf("test_log-%s.log", now.Format(logFileSuffix)))
			},
			version: 0,
			daysAgo: 2,
			prefix: "test_log",
	        err: nil,
	    },
		{
			name: "1 days ago single file",
			dirName: "create_new_file_test",
			ago: -24 * time.Hour,
			existingFile: func(dir, prefix string, now time.Time, version int) string {
				return filepath.Join(dir, fmt.Sprintf("test_log-%s.%d.log", now.Format(logFileSuffix), version))
			},
			version: 3,
			daysAgo: 1,
			prefix: "test_log",
			err: nil,
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

			expectFileToContain(t, existingFile, entry)

			fi, err := osStat(existingFile)
			if err != nil {
				t.Fatal(err)
			}

			lf, ok := parseFile(fi, tc.prefix, createFormat(tc.prefix), time.UTC)

			assert.True(t, ok, "regex could not match filename")
			assert.Equal(t, tc.version, lf.version)
			assert.Equal(t, tc.daysAgo, lf.daysAgo)
	    })
	}
}
