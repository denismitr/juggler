package juggler

import (
	"github.com/pkg/errors"
	"io"
	"os"
	"regexp"
	"sync"
	"time"
)

const (
	dateSuffix          = "2006-01-02"
	defaultMaxMegabytes = 50
	defaultExt          = ".log"
)

var _ io.WriteCloser = (*Juggler)(nil)

var createFormat = func(prefix string) *regexp.Regexp {
	return regexp.MustCompile("^" + prefix + `-(?P<date>\d{4}-\d{2}-\d{2})\.(?P<version>\d{1,4})\.log$`)
}

var (
	currentTime = time.Now
	osStat      = os.Stat
	megabyte    = 1024 * 1024
)

type Juggler struct {
	directory string
	prefix    string

	maxFilesize int
	maxBackups  int
	timezone    *time.Location
	compression bool
	uploader    uploader

	closeCh        chan struct{}
	errCh          chan error
	errorObservers []chan error
	nextTick       time.Duration
	format         *regexp.Regexp

	cmu sync.RWMutex

	currentFilepath string
	currentSize     int64
	currentTime     time.Time
	currentFile     *os.File
	currentVersion  int
}

func New(prefix string, dir string, cfgs ...Configurator) *Juggler {
	j := &Juggler{
		prefix:         prefix,
		directory:      dir,
		currentVersion: 1,
		maxFilesize:    defaultMaxMegabytes,
		maxBackups:     5,
		closeCh:        make(chan struct{}),
		errCh:          make(chan error),
		nextTick:       5 * time.Second,
		timezone:       time.UTC,
		compression:    false,
		format:         createFormat(prefix),
		errorObservers: make([]chan error, 0),
	}

	for _, cfg := range cfgs {
		cfg(j)
	}

	go j.watch()

	j.currentFilepath = resolveFilepath(j.prefix, j.directory, currentTime(), j.currentVersion, j.timezone)

	return j
}

func (j *Juggler) NotifyOnError(errCh chan error) {
	j.errorObservers = append(j.errorObservers, errCh)
}

func (j *Juggler) Write(p []byte) (int, error) {
	ln := len(p)
	if int64(ln) > j.maxSize() {
		return 0, errors.Errorf("cannot write %d bytes at once", ln)
	}

	if err := j.juggle(ln); err != nil {
		return 0, err
	}

	n, err := j.currentFile.Write(p)

	j.cmu.Lock()
	j.currentSize += int64(n)
	j.cmu.Unlock()

	return n, err
}

func (j *Juggler) juggle(n int) error {
	currentFilepath, size, exists, err := j.resolveCurrentFile()
	if err != nil {
		return errors.Wrapf(err, "error getting stats for %s", currentFilepath)
	}

	if !exists {
		return j.create(currentFilepath)
	}

	j.cmu.RLock()
	needsJuggling := size+int64(n) >= j.maxSize() || j.currentSize+int64(n) > j.maxSize()
	j.cmu.RUnlock()

	if needsJuggling {
		if err := j.close(); err != nil {
			return err
		}

		j.cmu.Lock()
		j.currentVersion += 1
		j.cmu.Unlock()
		return j.juggle(n)
	}

	if j.currentFilepath == currentFilepath && j.currentFile != nil && j.currentSize == size {
		return nil
	}

	j.cmu.Lock()
	f, err := os.OpenFile(currentFilepath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrapf(err, "could not open file %s", currentFilepath)
	}

	j.currentFilepath = currentFilepath
	j.currentFile = f
	j.currentSize = size

	j.cmu.Unlock()

	return nil
}

func (j *Juggler) resolveCurrentFile() (currentFilepath string, size int64, exists bool, err error) {
	j.cmu.RLock()
	defer j.cmu.RUnlock()

	currentFilepath = resolveFilepath(j.prefix, j.directory, currentTime(), j.currentVersion, j.timezone)
	info, statErr := osStat(currentFilepath)

	if statErr != nil {
		if os.IsNotExist(statErr) {
			exists = false
		} else {
			err = statErr
		}
	} else {
		exists = true
		size = info.Size()
	}

	return
}

func (j *Juggler) create(filepath string) error {
	j.cmu.Lock()
	defer j.cmu.Unlock()

	err := os.MkdirAll(j.directory, 0755)
	if err != nil {
		return errors.Wrapf(err, "cannot create new directory %s", j.directory)
	}

	mode := os.FileMode(0600)
	f, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_TRUNC, mode)
	if err != nil {
		return errors.Wrapf(err, "cannot create currentFile %s at %s", filepath, j.directory)
	}

	j.currentFilepath = filepath
	j.currentFile = f
	j.currentSize = 0

	return nil
}

func (j *Juggler) maxSize() int64 {
	return int64(j.maxFilesize) * int64(megabyte)
}

func (j *Juggler) close() error {
	if j.currentFile == nil {
		return nil
	}

	if err := j.currentFile.Close(); err != nil {
		return errors.Wrapf(err, "could not close currentFile %s", j.currentFilepath)
	}

	j.currentFile = nil

	return nil
}

func (j *Juggler) watch() {
	tick := time.NewTicker(j.nextTick)
	backupRunCh := make(chan struct{})

	storage := j.createStorage()

	go storage.start(backupRunCh, j.errCh)

loop:
	for {
		select {
		case <-tick.C:
			backupRunCh <- struct{}{}
		case <-j.closeCh:
			close(backupRunCh)
			break loop
		case err := <-j.errCh:
			for _, c := range j.errorObservers {
				select {
				case c <- err:
				}
			}
		}
	}

	tick.Stop()
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
