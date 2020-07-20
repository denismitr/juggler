package juggler

import "time"

type Configurator func(j *Juggler)

func WithMaxMegabytes(maxMegabytes int) Configurator {
	return func(j *Juggler) {
		j.maxMegabytes = maxMegabytes
	}
}

func WithTimezone(tz *time.Location) Configurator {
	return func(j *Juggler) {
		j.timezone = tz
	}
}

func WithMaxBackups(backups int) Configurator {
	return func(j *Juggler) {
		j.backupDays = backups
	}
}

func WithCompression() Configurator {
	return func(j *Juggler) {
		j.compression = true
	}
}

func WithNextTick(nextTick time.Duration) Configurator {
	return func(j *Juggler) {
		j.nextTick = nextTick
	}
}
