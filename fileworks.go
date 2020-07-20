package juggler

import (
	"compress/gzip"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type lofFileMeta struct {
	daysAgo int
	version int
	dir string
	f os.FileInfo
}

func (f lofFileMeta) fullPath() string {
	return filepath.Join(f.dir, f.f.Name())
}

type orderedLogFilesMeta []lofFileMeta

func (f orderedLogFilesMeta) Less(i, j int) bool {
	if f[i].daysAgo > f[j].daysAgo {
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

func gzippedName(file string) string {
	return file + ".gz"
}

func parseLogFileMeta(dir string, f os.FileInfo, prefix string, format *regexp.Regexp, tz *time.Location) (lofFileMeta, bool) {
	if ! strings.HasSuffix(f.Name(), ".log") {
		return lofFileMeta{}, false
	}

	if ! strings.HasPrefix(f.Name(), prefix) {
		return lofFileMeta{}, false
	}

	matches := format.FindStringSubmatch(f.Name())
	result := lofFileMeta{f: f, dir: dir}

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

		if logFile, ok := parseLogFileMeta(dir, files[i], prefix, format, tz); ok {
			result = append(result, logFile)
		}
	}

	sort.Sort(orderedLogFilesMeta(result))

	// if last entry is today exclude it from backup list
	if len(result) > 0 && result[len(result) - 1].daysAgo == 0 {
		result = result[:len(result) - 1]
	}

	return result, nil
}

func compress(src string, wg *sync.WaitGroup, errCh chan error) {
	defer wg.Done()
	f, err := os.Open(src)
	if err != nil {
		errCh <- errors.Wrapf(err, "failed to open log file: %s", src)
		return
	}

	defer func() {
		if err := f.Close(); err != nil {
			errCh <- err
		}
	}()

	fi, err := osStat(src)
	if err != nil {
		errCh <- errors.Wrapf(err, "failed to read stats from file %s", src)
		return
	}

	dst := gzippedName(src)

	gzf, err := os.OpenFile(dst, os.O_CREATE | os.O_TRUNC | os.O_WRONLY, fi.Mode())
	if err != nil {
		errCh <- errors.Wrapf(err, "failed to create file %s", dst)
		return
	}

	defer func() {
		if err := os.Remove(src); err != nil {
			errCh <- err
		}

		if err := gzf.Close(); err != nil {
			errCh <- err
		}
	}()

	if err := chown(dst, fi); err != nil {
		errCh <- fmt.Errorf("failed to chown compressed log file: %v", err)
		return
	}

	gz := gzip.NewWriter(gzf)

	if _, err := io.Copy(gz, f); err != nil {
		errCh <- errors.Wrapf(err, "could not copy compressed content from %s to %s", src, dst)
		return
	}

	if err := gz.Close(); err != nil {
		errCh <- err
		return
	}
}
