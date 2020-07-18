package juggler

import (
	"compress/gzip"
	"github.com/pkg/errors"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type logFile struct {
	daysAgo int
	version int
	f os.FileInfo
}

type orderedLogFiles []logFile

func (f orderedLogFiles) Less(i, j int) bool {
	if f[i].daysAgo < f[j].daysAgo {
		return true
	}

	if f[i].daysAgo == f[j].daysAgo {
		return f[i].version < f[j].version
	}

	return false
}

func (f orderedLogFiles) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

func (f orderedLogFiles) Len() int {
	return len(f)
}

func parseFile(f os.FileInfo, prefix string, format *regexp.Regexp, tz *time.Location) (logFile, bool) {
	if ! strings.HasSuffix(f.Name(), ".log") {
		return logFile{}, false
	}

	if ! strings.HasPrefix(f.Name(), prefix) {
		return logFile{}, false
	}

	matches := format.FindStringSubmatch(f.Name())
	result := logFile{f: f}

	if len(matches) == 0 {
		return logFile{}, false
	}

	for i, name := range format.SubexpNames() {
		if i != 0 && name == "version" && matches[i] != "" {
			v, _ := strconv.Atoi(matches[i])
			if v != 0 {
				result.version = v
			}
		}

		if i != 0 && name == "date" && matches[i] != "" {
			t, err := time.Parse(logFileSuffix, matches[i])
			if err != nil {
				panic(err) // todo: remove
			}

			now := time.Now().In(tz)
			result.daysAgo = int(now.Sub(t).Hours() / 24)
		}
	}

	return result, true
}

func compress(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return errors.Wrapf(err, "failed to open log file: %s", file)
	}

	defer f.Close()

	stats, err := osStat(file)
	if err != nil {
		return errors.Wrapf(err, "failed to read stats from file %s", file)
	}

	dst := file + ".gz"

	gzf, err := os.OpenFile(dst, os.O_APPEND | os.O_TRUNC | os.O_WRONLY, stats.Mode())
	if err != nil {
		return errors.Wrapf(err, "failed to create file %s", dst)
	}

	defer func() {
		os.Remove(file)
		gzf.Close()
		os.Remove(dst)
	}()

	gz := gzip.NewWriter(gzf)

	if _, err := io.Copy(gz, f); err != nil {
		return errors.Wrapf(err, "could not copy compressed content from %s to %s", file, dst)
	}

	if err := gz.Close(); err != nil {
		return err
	}

	return nil
}
