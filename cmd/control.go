package main

import (
	tui "git.sr.ht/~marius/motley"
	"git.sr.ht/~marius/motley/internal/config"
	"git.sr.ht/~marius/motley/internal/env"
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/storage"
	"github.com/openshift/osin"
	"github.com/sirupsen/logrus"
	"gopkg.in/urfave/cli.v2"
	"time"
)

var (
	ctl Control
	logger = logrus.New()
)

type Control struct {
	Conf        config.Options
	AuthStorage osin.Storage
	Storage     storage.Repository
}

func New(authDB osin.Storage, actorDb storage.Repository, conf config.Options) *Control {
	return &Control{
		Conf:        conf,
		AuthStorage: authDB,
		Storage:     actorDb,
	}
}

func Before(c *cli.Context) error {
	logger.Level = logrus.WarnLevel
	ct, err := setup(c, logger)
	if err != nil {
		// Ensure we don't print the default help message, which is not useful here
		c.App.CustomAppHelpTemplate = "Failed"
		logger.Errorf("Error: %s", err)
		return err
	}
	ctl = *ct
	// the level enums have same values
	logger.Level = logrus.TraceLevel

	return nil
}

func setup(c *cli.Context, l logrus.FieldLogger) (*Control, error) {
	environ := env.Type(c.String("env"))
	if environ == "" {
		environ = env.DEV
	}
	conf, err := config.LoadFromEnv(environ, time.Second)
	if err != nil {
		l.Errorf("Unable to load config files for environment %s: %s", environ, err)
	}

	if dir := c.String("path"); dir != "." {
		conf.StoragePath = dir
	}
	typ := c.String("type")
	if typ != "" {
		conf.Storage = config.StorageType(typ)
	}
	db, aDb, err := Storage(conf, l)
	if err != nil {
		return nil, err
	}
	return New(aDb, db, conf), nil
}

var TuiAction = func(*cli.Context) error {
	return tui.Launch(pub.IRI(ctl.Conf.BaseURL), ctl.Storage)
}
