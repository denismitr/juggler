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
	now := time.Now().Add(-48 * time.Hour) // log file from two days ago

	dir := makeTestDir("create_new_file_test", t)
	defer os.RemoveAll(dir)

	existingFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.log", now.Format(logFileSuffix)))
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

	lf, ok := parseFile(fi, "test_log", createFormat("prefix"), time.UTC)

	assert.True(t, ok, "regex could not match filename")
	assert.Equal(t, 0, lf.version)
	assert.Equal(t, 2, lf.daysAgo)
}
