package storage

import (
	iofs "io/fs"
	"net/url"
	"strings"

	"git.sr.ht/~mariusor/lw"
	"git.sr.ht/~mariusor/motley/internal/config"
	"git.sr.ht/~mariusor/motley/internal/env"
	"github.com/go-ap/errors"
	badger "github.com/go-ap/storage-badger"
	boltdb "github.com/go-ap/storage-boltdb"
	fs "github.com/go-ap/storage-fs"
	sqlite "github.com/go-ap/storage-sqlite"
)

var (
	emptyFieldsLogFn = func(string, ...interface{}) {}
	emptyLogFn       = func(string, ...interface{}) {}
	InfoLogFn        = func(l lw.Logger) func(string, ...interface{}) {
		if l == nil {
			return emptyFieldsLogFn
		}
		return func(s string, p ...interface{}) { l.Infof(s, p...) }
	}
	ErrLogFn = func(l lw.Logger) func(string, ...interface{}) {
		if l == nil {
			return emptyFieldsLogFn
		}
		return func(s string, p ...interface{}) { l.Errorf(s, p...) }
	}
)

func getBadgerStorage(c config.Storage, e env.Type, u string, l lw.Logger) (config.FullStorage, error) {
	path, err := c.Badger(e, u)
	if err != nil {
		return nil, err
	}
	l.Debugf("Initializing badger storage at %s", path)
	db, err := badger.New(badger.Config{
		Path:  path,
		LogFn: InfoLogFn(l),
		ErrFn: ErrLogFn(l),
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func getBoltStorageAtPath(dir, _ string, l lw.Logger) (config.FullStorage, error) {
	return boltdb.New(boltdb.Config{
		Path:  dir,
		LogFn: InfoLogFn(l),
		ErrFn: ErrLogFn(l),
	})
}

func getBoltStorage(c config.Storage, e env.Type, u string, l lw.Logger) (config.FullStorage, error) {
	path, err := c.BoltDB(e, u)
	if err != nil {
		return nil, err
	}
	l.Debugf("Initializing boltdb storage at %s", path)
	db, err := getBoltStorageAtPath(path, u, l)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func getFsStorageAtPath(dir string, e env.Type, rawURL string, l lw.Logger) (config.FullStorage, error) {
	if u, err := url.ParseRequestURI(rawURL); err == nil {
		rawURL = u.Host
	}
	if strings.Contains(dir, rawURL) {
		dir = strings.Replace(dir, rawURL, "", 1)
	}
	return fs.New(fs.Config{
		Path:   dir,
		Logger: l,
	})
}

func getFsStorage(c config.Storage, e env.Type, u string, l lw.Logger) (config.FullStorage, error) {
	path, err := c.BaseStoragePath(e, u)
	if err != nil {
		var pathError *iofs.PathError
		if !errors.As(err, &pathError) {
			return nil, err
		}
		path = c.Path
	}
	l.Debugf("Initializing fs storage at %s", path)
	db, err := getFsStorageAtPath(path, e, u, l)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func getSqliteStorage(c config.Storage, e env.Type, u string, l lw.Logger) (config.FullStorage, error) {
	l.Debugf("Initializing sqlite storage at %s", c.Path)
	db, err := sqlite.New(sqlite.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil

}

func Storage(c config.Storage, e env.Type, u string, l lw.Logger) (config.FullStorage, error) {
	switch c.Type {
	case config.StorageBoltDB:
		return getBoltStorage(c, e, u, l)
	case config.StorageBadger:
		return getBadgerStorage(c, e, u, l)
	case config.StorageSqlite:
		return getSqliteStorage(c, e, u, l)
	case config.StorageFS:
		return getFsStorage(c, e, u, l)
	}
	return nil, errors.NotImplementedf("Invalid storage type %s", c.Type)
}

func StorageFromDirectPath(c config.Storage, e env.Type, u string, l lw.Logger) (config.FullStorage, error) {
	switch c.Type {
	case config.StorageFS:
		db, err := getFsStorageAtPath(c.Path, e, u, l)
		return db, err
	}
	return nil, errors.NotImplementedf("Invalid storage type %s", c.Type)
}
