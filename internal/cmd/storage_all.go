package cmd

import (
	badger "github.com/go-ap/storage-badger"
	boltdb "github.com/go-ap/storage-boltdb"
	fs "github.com/go-ap/storage-fs"
	sqlite "github.com/go-ap/storage-sqlite"
	"path/filepath"

	"git.sr.ht/~marius/motley/internal/config"
	"github.com/go-ap/errors"
	"github.com/go-ap/fedbox"
	"github.com/sirupsen/logrus"
)

var (
	emptyFieldsLogFn = func(string, ...interface{}) {}
	emptyLogFn       = func(string, ...interface{}) {}
	InfoLogFn        = func(l logrus.FieldLogger) func(string, ...interface{}) {
		if l == nil {
			return emptyFieldsLogFn
		}
		return func(s string, p ...interface{}) { l.Infof(s, p...) }
	}
	ErrLogFn = func(l logrus.FieldLogger) func(string, ...interface{}) {
		if l == nil {
			return emptyFieldsLogFn
		}
		return func(s string, p ...interface{}) { l.Errorf(s, p...) }
	}
)

func getBadgerStorage(c config.Options, l logrus.FieldLogger) (fedbox.FullStorage, error) {
	path, err := c.Badger()
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

func getBoltStorageAtPath(dir, _ string, l logrus.FieldLogger) (fedbox.FullStorage, error) {
	return boltdb.New(boltdb.Config{
		Path:  dir,
		LogFn: InfoLogFn(l),
		ErrFn: ErrLogFn(l),
	})
}

func getBoltStorage(c config.Options, l logrus.FieldLogger) (fedbox.FullStorage, error) {
	path, err := c.BoltDB()
	if err != nil {
		return nil, err
	}
	l.Debugf("Initializing boltdb storage at %s", path)
	db, err := getBoltStorageAtPath(path, c.BaseURL, l)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func getFsStorageAtPath(dir, url string, l logrus.FieldLogger) (fedbox.FullStorage, error) {
	if dir, err := config.FullStoragePath(dir); err != nil {
		return nil, err
	} else {
		return fs.New(fs.Config{
			Path:  dir,
			LogFn: InfoLogFn(l),
			ErrFn: ErrLogFn(l),
		})
	}
}

func getFsStorage(c config.Options, l logrus.FieldLogger) (fedbox.FullStorage, error) {
	path, err := c.BaseStoragePath()
	if err != nil {
		return nil, err
	}
	l.Debugf("Initializing fs storage at %s", path)
	db, err := getFsStorageAtPath(filepath.Dir(path), c.BaseURL, l)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func getSqliteStorage(c config.Options, l logrus.FieldLogger) (fedbox.FullStorage, error) {
	l.Debugf("Initializing sqlite storage at %s", c.StoragePath)
	db, err := sqlite.New(sqlite.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil

}

func Storage(c config.Options, l logrus.FieldLogger) (fedbox.FullStorage, error) {
	switch c.Storage {
	case config.StorageBoltDB:
		return getBoltStorage(c, l)
	case config.StorageBadger:
		return getBadgerStorage(c, l)
	case config.StorageSqlite:
		return getSqliteStorage(c, l)
	case config.StorageFS:
		return getFsStorage(c, l)
	}
	return nil, errors.NotImplementedf("Invalid storage type %s", c.Storage)
}

func StorageFromDirectPath(c config.Options, l logrus.FieldLogger) (fedbox.FullStorage, error) {
	switch c.Storage {
	case config.StorageFS:
		db, err := getFsStorageAtPath(c.StoragePath, c.BaseURL, l)
		return db, err
	}
	return nil, errors.NotImplementedf("Invalid storage type %s", c.Storage)
}
