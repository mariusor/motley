package main

import (
	"git.sr.ht/~marius/motley/internal/config"
	authbadger "github.com/go-ap/auth/badger"
	authboltdb "github.com/go-ap/auth/boltdb"
	authfs "github.com/go-ap/auth/fs"
	authpgx "github.com/go-ap/auth/pgx"
	"github.com/go-ap/errors"
	"github.com/go-ap/fedbox/storage/badger"
	"github.com/go-ap/fedbox/storage/boltdb"
	"github.com/go-ap/fedbox/storage/fs"
	"github.com/go-ap/fedbox/storage/pgx"
	"github.com/go-ap/fedbox/storage/sqlite"
	st "github.com/go-ap/storage"
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

func getBadgerStorage(c config.Options, l logrus.FieldLogger) (st.Repository, osin.Storage, error) {
	l.Debugf("Initializing badger storage at %s", c.Badger())
	db := badger.New(badger.Config{
		Path:  c.Badger(),
		LogFn: InfoLogFn(l),
		ErrFn: ErrLogFn(l),
	}, c.BaseURL)
	oauth := authbadger.New(authbadger.Config{
		Path:  c.BadgerOAuth2(),
		Host:  c.Host,
		LogFn: InfoLogFn(l),
		ErrFn: ErrLogFn(l),
	})
	return db, oauth, nil
}

func getBoltStorage(c config.Options, l logrus.FieldLogger) (st.Repository, osin.Storage, error) {
	l.Debugf("Initializing boltdb storage at %s", c.BoltDB())
	db := boltdb.New(boltdb.Config{
		Path:  c.BoltDB(),
		LogFn: InfoLogFn(l),
		ErrFn: ErrLogFn(l),
	}, c.BaseURL)

	oauth := authboltdb.New(authboltdb.Config{
		Path:       c.BoltDBOAuth2(),
		BucketName: c.Host,
		LogFn:      InfoLogFn(l),
		ErrFn:      ErrLogFn(l),
	})
	return db, oauth, nil
}

func getFsStorage(c config.Options, l logrus.FieldLogger) (st.Repository, osin.Storage, error) {
	l.Debugf("Initializing fs storage at %s", c.BaseStoragePath())
	oauth := authfs.New(authfs.Config{
		Path:  c.BaseStoragePath(),
		LogFn: InfoLogFn(l),
		ErrFn: ErrLogFn(l),
	})
	db, err := fs.New(fs.Config{
		StoragePath: c.StoragePath,
		Env:         string(c.Env),
		BaseURL:     c.BaseURL,
	})
	if err != nil {
		return nil, oauth, err
	}
	return db, oauth, nil
}

func getSqliteStorage(c config.Options, l logrus.FieldLogger) (st.Repository, osin.Storage, error) {
	l.Debugf("Initializing sqlite storage at %s", c.StoragePath)
	db, err := sqlite.New(sqlite.Config{})
	if err != nil {
		return nil, nil, err
	}
	return db, nil, errors.NotImplementedf("sqlite storage backend is not implemented yet")
}

func getPgxStorage(c config.Options, l logrus.FieldLogger) (st.Repository, osin.Storage, error) {
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

func Storage(c config.Options, l logrus.FieldLogger) (st.Repository, osin.Storage, error) {
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
