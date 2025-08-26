package dirtest

import (
	"io/fs"
	"testing/fstest"
)

func New(options ...Option) fs.FS {
	dir := fstest.MapFS{}

	for _, option := range options {
		option(&dir)
	}

	return dir
}

type Option func(dir *fstest.MapFS)

func WithDir(name string) Option {
	return func(dir *fstest.MapFS) {
		(*dir)[name] = &fstest.MapFile{
			Mode: fs.ModeDir,
		}
	}
}

func WithFile(name string, content []byte) Option {
	return func(dirFS *fstest.MapFS) {
		(*dirFS)[name] = &fstest.MapFile{
			Data: content,
		}
	}
}
