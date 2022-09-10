package cmd

import (
	"path/filepath"

	"git.sr.ht/~marius/motley/internal/config"
	authbadger "github.com/go-ap/auth/badger"
	authboltdb "github.com/go-ap/auth/boltdb"
	authfs "github.com/go-ap/auth/fs"
	authpgx "github.com/go-ap/auth/pgx"
	authsqlite "github.com/go-ap/auth/sqlite"
	"github.com/go-ap/errors"
	"github.com/go-ap/fedbox/storage/badger"
	"github.com/go-ap/fedbox/storage/boltdb"
	"github.com/go-ap/fedbox/storage/fs"
	"github.com/go-ap/fedbox/storage/pgx"
	"github.com/go-ap/fedbox/storage/sqlite"
	st "github.com/go-ap/processing"
	"github.com/openshift/osin"
	"github.com/sirupsen/logrus"
)

var (
	emptyFieldsLogFn = func(logrus.Fields, string, ...interface{}) {}
	emptyLogFn       = func(string, ...interface{}) {}
	InfoLogFn        = func(l logrus.FieldLogger) func(logrus.Fields, string, ...interface{}) {
		if l == nil {
			return emptyFieldsLogFn
		}
		return func(f logrus.Fields, s string, p ...interface{}) { l.WithFields(f).Infof(s, p...) }
	}
	ErrLogFn = func(l logrus.FieldLogger) func(logrus.Fields, string, ...interface{}) {
		if l == nil {
			return emptyFieldsLogFn
		}
		return func(f logrus.Fields, s string, p ...interface{}) { l.WithFields(f).Errorf(s, p...) }
	}
)

func getBadgerStorage(c config.Options, l logrus.FieldLogger) (st.Store, osin.Storage, error) {
	path, err := c.Badger()
	if err != nil {
		return nil, nil, err
	}
	l.Debugf("Initializing badger storage at %s", path)
	db, err := badger.New(badger.Config{
		Path:    path,
		BaseURL: c.BaseURL,
		LogFn:   InfoLogFn(l),
		ErrFn:   ErrLogFn(l),
	})
	if err != nil {
		return nil, nil, err
	}
	path, err = c.BadgerOAuth2()
	if err != nil {
		return nil, nil, err
	}
	oauth := authbadger.New(authbadger.Config{
		Path:  path,
		Host:  c.Host,
		LogFn: InfoLogFn(l),
		ErrFn: ErrLogFn(l),
	})
	return db, oauth, nil
}

func getBoltStorage(c config.Options, l logrus.FieldLogger) (st.Store, osin.Storage, error) {
	path, err := c.BoltDB()
	if err != nil {
		return nil, nil, err
	}
	l.Debugf("Initializing boltdb storage at %s", path)
	db, err := boltdb.New(boltdb.Config{
		Path:    path,
		BaseURL: c.BaseURL,
		LogFn:   InfoLogFn(l),
		ErrFn:   ErrLogFn(l),
	})
	if err != nil {
		return nil, nil, err
	}

	path, err = c.BoltDBOAuth2()
	if err != nil {
		return nil, nil, err
	}
	oauth := authboltdb.New(authboltdb.Config{
		Path:       path,
		BucketName: c.Host,
		LogFn:      InfoLogFn(l),
		ErrFn:      ErrLogFn(l),
	})
	return db, oauth, nil
}

func getFsStorage(c config.Options, l logrus.FieldLogger) (st.Store, osin.Storage, error) {
	path, err := c.BaseStoragePath()
	if err != nil {
		return nil, nil, err
	}
	l.Debugf("Initializing fs storage at %s", path)
	oauth := authfs.New(authfs.Config{
		Path:  path,
		LogFn: InfoLogFn(l),
		ErrFn: ErrLogFn(l),
	})
	db, err := fs.New(fs.Config{
		StoragePath: filepath.Dir(path),
		BaseURL:     c.BaseURL,
	})
	if err != nil {
		return nil, oauth, err
	}
	return db, oauth, nil
}

func getSqliteStorage(c config.Options, l logrus.FieldLogger) (st.Store, osin.Storage, error) {
	l.Debugf("Initializing sqlite storage at %s", c.StoragePath)
	db, err := sqlite.New(sqlite.Config{})
	if err != nil {
		return nil, nil, err
	}
	path, err := c.BaseStoragePath()
	if err != nil {
		return nil, nil, err
	}
	oauth := authsqlite.New(authsqlite.Config{
		Path:  path,
		LogFn: InfoLogFn(l),
		ErrFn: ErrLogFn(l),
	})
	return db, oauth, nil

}

func getPgxStorage(c config.Options, l logrus.FieldLogger) (st.Store, osin.Storage, error) {
	// @todo(marius): we're no longer loading SQL db config env variables
	l.Debugf("Initializing pgx storage at %s", c.StoragePath)
	conf := pgx.Config{}
	db, err := pgx.New(conf, c.BaseURL, l)
	if err != nil {
		return nil, nil, err
	}

	oauth := authpgx.New(authpgx.Config{
		Enabled: true,
		Host:    conf.Host,
		Port:    int64(conf.Port),
		User:    conf.User,
		Pw:      conf.Password,
		Name:    conf.Database,
		LogFn:   InfoLogFn(l),
		ErrFn:   ErrLogFn(l),
	})
	return db, oauth, errors.NotImplementedf("sqlite storage backend is not implemented yet")
}

func Storage(c config.Options, l logrus.FieldLogger) (st.Store, osin.Storage, error) {
	switch c.Storage {
	case config.StorageBoltDB:
		return getBoltStorage(c, l)
	case config.StorageBadger:
		return getBadgerStorage(c, l)
	case config.StoragePostgres:
		return getPgxStorage(c, l)
	case config.StorageSqlite:
		return getSqliteStorage(c, l)
	case config.StorageFS:
		return getFsStorage(c, l)
	}
	return nil, nil, errors.NotImplementedf("Invalid storage type %s", c.Storage)
}
