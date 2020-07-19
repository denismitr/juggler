package juggler

import (
	"compress/gzip"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type lofFileMeta struct {
	daysAgo int
	version int
	f os.FileInfo
}

type orderedLogFilesMeta []lofFileMeta

func (f orderedLogFilesMeta) Less(i, j int) bool {
	if f[i].daysAgo < f[j].daysAgo {
		return true
	}

	if f[i].daysAgo == f[j].daysAgo {
		return f[i].version < f[j].version
	}

	return false
}

func (f orderedLogFilesMeta) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

func (f orderedLogFilesMeta) Len() int {
	return len(f)
}

func parseLogFileMeta(f os.FileInfo, prefix string, format *regexp.Regexp, tz *time.Location) (lofFileMeta, bool) {
	if ! strings.HasSuffix(f.Name(), ".log") {
		return lofFileMeta{}, false
	}

	if ! strings.HasPrefix(f.Name(), prefix) {
		return lofFileMeta{}, false
	}

	matches := format.FindStringSubmatch(f.Name())
	result := lofFileMeta{f: f}

	if len(matches) == 0 {
		return lofFileMeta{}, false
	}

	for i, name := range format.SubexpNames() {
		if i != 0 && name == "version" && matches[i] != "" {
			v, _ := strconv.Atoi(matches[i])
			if v != 0 {
				result.version = v
			}
		}

		if i != 0 && name == "date" && matches[i] != "" {
			days, err := parseDayDiff(matches[i], tz)
			if err != nil {
				panic(err) // todo: remove
			}
			result.daysAgo = days
		}
	}

	return result, true
}

func parseDayDiff(date string, tz *time.Location) (days int, err error) {
	t, err := time.Parse(logFileSuffix, date)
	if err != nil {
		err = errors.Wrapf(err, "could not parse date %s", date)
		return
	}

	now := currentTime().In(tz)
	days = int(now.Sub(t).Hours() / 24)

	return
}

func scanBackups(
	dir, prefix string,
	format *regexp.Regexp,
	tz *time.Location,
) ([]lofFileMeta, error) {
	if dir == "" {
		return nil, errors.Errorf("Directory is not set")
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read directory [%s] content", dir)
	}

	var result []lofFileMeta

	for i := range files {
		if files[i].IsDir() {
			continue
		}

		if logFile, ok := parseLogFileMeta(files[i], prefix, format, tz); ok {
			result = append(result, logFile)
		}
	}

	sort.Sort(orderedLogFilesMeta(result))

	return result, nil
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
