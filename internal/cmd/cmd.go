package cmd

import (
	"time"

	tui "git.sr.ht/~marius/motley"
	"git.sr.ht/~marius/motley/internal/config"
	"git.sr.ht/~marius/motley/internal/env"
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/storage"
	"github.com/openshift/osin"
	"github.com/sirupsen/logrus"
	"gopkg.in/urfave/cli.v2"
)

var (
	ctl    Control
	logger = logrus.New()
)

type Control struct {
	Conf        config.Options
	AuthStorage osin.Storage
	Storage     storage.Store
}

func New(authDB osin.Storage, actorDb storage.Store, conf config.Options) *Control {
	return &Control{
		Conf:        conf,
		AuthStorage: authDB,
		Storage:     actorDb,
	}
}

func Before(c *cli.Context) error {
	ct, err := setup(c, logger)
	if err != nil {
		// Ensure we don't print the default help message, which is not useful here
		c.App.CustomAppHelpTemplate = "Failed"
		logger.Errorf("Error: %s", err)
		return err
	}
	ctl = *ct
	return nil
}

func setup(c *cli.Context, l *logrus.Logger) (*Control, error) {
	environ := env.Type(c.String("env"))
	if environ == "" {
		environ = env.DEV
	}
	dir := c.String("path")
	conf, err := config.LoadFromEnv(dir, environ, time.Second)
	if err != nil {
		l.Errorf("Unable to load config files for environment %s: %s", environ, err)
	}
	if typ := c.String("type"); typ != "" {
		conf.Storage = config.StorageType(typ)
	}
	if u := c.String("url"); u != "" {
		conf.BaseURL = u
	}
	l.SetLevel(conf.LogLevel)
	db, aDb, err := Storage(conf, l)
	if err != nil {
		l.Errorf("Unable to access storage: %s", err)
		return nil, err
	}
	return New(aDb, db, conf), nil
}

var TuiAction = func(*cli.Context) error {
	return tui.Launch(pub.IRI(ctl.Conf.BaseURL), ctl.Storage, ctl.AuthStorage)
}
