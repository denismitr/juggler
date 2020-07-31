package juggler

import "time"

type Configurator func(j *Juggler)

func WithMaxMegabytes(maxMegabytes int) Configurator {
	return func(j *Juggler) {
		j.maxFilesize = maxMegabytes
	}
}

func WithTimezone(tz *time.Location) Configurator {
	return func(j *Juggler) {
		j.timezone = tz
	}
}

func WithMaxBackups(backups int) Configurator {
	return func(j *Juggler) {
		j.maxBackups = backups
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

func WithCompressionAndCloudUploader(uploader uploader) Configurator {
	return func(j *Juggler) {
		j.compression = true
		j.uploader = uploader
	}
}

func withNowFunc(nowFunc nowFunc) Configurator {
	return func(j *Juggler) {
		j.nowFunc = nowFunc
	}
}
