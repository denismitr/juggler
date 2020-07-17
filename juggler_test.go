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

func setTestNow() time.Time {
	return time.Now()
}

func TestCreateNewFile(t *testing.T) {
	dir := makeTestDir("create_new_file_test", t)
	defer os.RemoveAll(dir)

	currentTime = setTestNow
	now := setTestNow()

	j := New("test_log", dir)
	defer j.Close()

	b := []byte("test log")
	n, err := j.Write(b)
	assert.NoError(t, err)
	assert.Equal(t, len(b), n)
	expectedFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.log", now.Format(logFileSuffix)))
	assert.FileExists(t, expectedFile)
	expectFileToContain(t, expectedFile, b)
}

func TestAppendToExistingFile(t *testing.T) {
	currentTime = setTestNow
	now := setTestNow()

	dir := makeTestDir("create_new_file_test", t)
	defer os.RemoveAll(dir)

	existingFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.log", now.Format(logFileSuffix)))
	entry := []byte("logEntry\n")
	err := ioutil.WriteFile(existingFile, entry, 0644)
	if err != nil {
		t.Fatal(err)
	}

	expectFileToContain(t, existingFile, entry)

	l := New("test_log", dir)
	defer l.Close()

	nextEntry := []byte("nextEntry\n")
	n, err := l.Write(nextEntry)

	assert.NoError(t, err)
	assert.Equal(t, n, len(nextEntry))
	expectFileToContain(t, existingFile, append(entry, nextEntry...))
}

func TestJugglingDuringWrite(t *testing.T) {
	currentTime = setTestNow
	now := setTestNow()
	mb = 1

	dir := makeTestDir("create_new_file_test", t)
	defer os.RemoveAll(dir)

	existingFile := filepath.Join(dir, fmt.Sprintf("test_log-%s.log", now.Format(logFileSuffix)))
	entry := []byte("logEntry\n")
	err := ioutil.WriteFile(existingFile, entry, 0644)
	if err != nil {
		t.Fatal(err)
	}

	expectFileToContain(t, existingFile, entry)

	l := New("test_log", dir, WithMaxMegabytes(17))
	defer l.Close()

	nextFile := filepath.Join(dir, fmt.Sprintf("test_log-%s-2.log", now.Format(logFileSuffix)))
	nextEntry := []byte("next log too big")
	n, err := l.Write(nextEntry)
	assert.NoError(t, err)
	assert.Equal(t, len(nextEntry), n)
	expectFileToContain(t, nextFile, nextEntry)
}

func expectFileToContain(t *testing.T, file string, content []byte) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, b, content)
}

func makeTestDir(name string, tb testing.TB) string {
	dir := filepath.Join(os.TempDir(), name)
	if err := os.Mkdir(dir, 0700); err != nil {
		if os.IsExist(err) {
			tb.Logf("Already exists %s", dir)
		} else {
			panic(err)
		}
	}
	return dir
}
