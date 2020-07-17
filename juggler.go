package juggler

import (
	"fmt"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"
)

const (
	logFileSuffix       = "2006-01-02"
	defaultMaxMegabytes = 50
	defaultExt          = ".log"
)

var _ io.WriteCloser = (*Juggler)(nil)

var createFormat = func(prefix string) *regexp.Regexp {
	return regexp.MustCompile(`-(?P<date>\d{4}-\d{2}-\d{2}).log$`)
}

var (
	currentTime = time.Now
	osStat      = os.Stat
	mb          = 1024 * 1024
)

type Juggler struct {
	filename       string
	directory      string
	filenamePrefix string

	runChecksEvery time.Duration
	maxMegabytes   int
	backupDays     int
	timezone       *time.Location
	compression    bool

	closeCh chan struct{}
	format  *regexp.Regexp

	cmu         sync.Mutex
	currentSize int64
	currentTime time.Time
	currentFile *os.File
	version     int
}

func New(filenamePrefix string, dir string, cfgs ...Configurator) *Juggler {
	j := &Juggler{
		filenamePrefix: filenamePrefix,
		directory:      dir,
		version:        1,
		maxMegabytes:   defaultMaxMegabytes,
		backupDays:     5,
		closeCh:        make(chan struct{}),
		runChecksEvery: 5 * time.Second,
		timezone:       time.UTC,
		compression:    false,
		format:         createFormat(filenamePrefix),
	}

	for i := range cfgs {
		cfgs[i](j)
	}

	go j.watcher()

	j.updateFilename()

	return j
}

func (j *Juggler) Write(p []byte) (int, error) {
	j.cmu.Lock()
	defer j.cmu.Unlock()

	ln := len(p)
	if int64(ln) > j.maxSize() {
		return 0, errors.Errorf("cannot write %d bytes at once", ln)
	}

	if j.currentFile == nil {
		if err := j.openOrCreateFor(ln); err != nil {
			return 0, err
		}
	} else if j.currentSize+int64(ln) > j.maxSize() {
		if err := j.juggle(true); err != nil {
			return 0, err
		}
	}

	n, err := j.currentFile.Write(p)
	j.currentSize += int64(n)

	return n, err
}

func (j *Juggler) openOrCreateFor(n int) error {
	j.updateFilename()
	info, err := osStat(j.filename)
	if os.IsNotExist(err) {
		return j.create()
	}

	if err != nil {
		return errors.Wrapf(err, "error getting log currentFile %s info", j.filename)
	}

	if info.Size()+int64(n) >= j.maxSize() {
		if err := j.juggle(true); err != nil {
			panic(err)
		}
	}

	f, err := os.OpenFile(j.filename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return j.create()
	}

	j.currentFile = f
	j.currentSize = info.Size()
	return nil
}

func (j *Juggler) create() error {
	err := os.MkdirAll(j.directory, 0755)
	if err != nil {
		return errors.Wrapf(err, "cannot make new directory %s", j.directory)
	}

	j.updateFilename()

	mode := os.FileMode(0600)
	f, err := os.OpenFile(j.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_TRUNC, mode)
	if err != nil {
		return errors.Wrapf(err, "cannot create currentFile %s at %s", j.filename, j.directory)
	}

	j.currentFile = f
	j.currentSize = 0
	return nil
}

func (j *Juggler) updateFilename() {
	t := currentTime()

	if j.timezone != nil {
		t = t.In(j.timezone)
	} else {
		t = t.UTC()
	}

	j.currentTime = t

	date := t.Format(logFileSuffix)

	if j.version == 1 {
		j.filename = filepath.Join(j.directory, fmt.Sprintf("%s-%s%s", j.filenamePrefix, date, defaultExt))
	} else {
		j.filename = filepath.Join(j.directory, fmt.Sprintf("%s-%s.%d%s", j.filenamePrefix, date, j.version, defaultExt))
	}
}

func (j *Juggler) maxSize() int64 {
	return int64(j.maxMegabytes) * int64(mb)
}

func (j *Juggler) juggle(newVersion bool) error {
	if newVersion {
		j.version += 1
	} else {
		j.version = 1
	}

	if err := j.close(); err != nil {
		return err
	}

	if err := j.create(); err != nil {
		return err
	}

	return nil
}

func (j *Juggler) close() error {
	if j.currentFile == nil {
		return nil
	}

	if err := j.currentFile.Close(); err != nil {
		return errors.Wrapf(err, "could not close currentFile %s", j.filename)
	}

	j.currentFile = nil

	return nil
}

func (j *Juggler) watcher() {
	if j.backupDays == 0 && j.compression == false {
		return
	}

	tick := time.NewTicker(j.runChecksEvery)
	runCheck := make(chan struct{})

	go j.checkCompression(runCheck)

loop:
	for {
		select {
		case <-tick.C:
			runCheck <- struct{}{}
		case <-j.closeCh:
			close(runCheck)
			break loop
		}
	}

	tick.Stop()
}

func (j *Juggler) checkCompression(runCheck <-chan struct{}) {
	for range runCheck {
		files, err := j.inactiveLogFiles()
		if err != nil {
			panic(err) // todo: remove
		}

		for _, f := range files {
			if err := compress(f.f.Name()); err != nil {
				panic(err) // todo: remove
			}
		}
	}
}

func (j *Juggler) inactiveLogFiles() ([]logFile, error) {
	if j.directory == "" {
		return nil, errors.Errorf("Directory is not set")
	}

	files, err := ioutil.ReadDir(j.directory)
	if err != nil {
		return nil, err
	}

	var result []logFile

	for i := range files {
		if files[i].IsDir() {
			continue
		}

		if logFile, ok := parseFile(files[i], j.filenamePrefix, j.format, j.timezone); ok {
			result = append(result, logFile)
		}
	}

	sort.Sort(orderedLogFiles(result))

	return result, nil
}

func (j *Juggler) Close() error {
	j.cmu.Lock()
	defer func() {
		j.currentFile = nil
		j.currentSize = 0
		j.cmu.Unlock()
	}()

	if j.currentFile != nil {
		return j.currentFile.Close()
	}

	close(j.closeCh)

	return nil
}
