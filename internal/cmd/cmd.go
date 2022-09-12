package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	tui "git.sr.ht/~marius/motley"
	"git.sr.ht/~marius/motley/internal/config"
	"git.sr.ht/~marius/motley/internal/env"
	"github.com/go-ap/processing"
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
	Storage     processing.Store
}

func New(authDB osin.Storage, actorDb processing.Store, conf config.Options) *Control {
	return &Control{
		Conf:        conf,
		AuthStorage: authDB,
		Storage:     actorDb,
	}
}

func openlog(name string) io.Writer {
	f, err := os.OpenFile(filepath.Join("/var/tmp/", name+".log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return io.Discard
	}
	return f
}

func Before(c *cli.Context) error {
	return nil
}

func setup(c *cli.Context, l *logrus.Logger) (*Control, error) {
	environ := env.Type(c.String("env"))
	if environ == "" {
		environ = env.DEV
	}
	dir := c.String("path")
	l.SetOutput(openlog(c.App.Name))
	l.SetFormatter(&logrus.TextFormatter{DisableQuote: true, DisableTimestamp: true})

	conf, err := config.LoadFromEnv(dir, environ, time.Second)
	if err != nil {
		l.Errorf("Unable to load config file for environment %s: %s", environ, err)
		return nil, fmt.Errorf("unable to load config in path: %s", dir)
	}

	l.SetLevel(conf.LogLevel)
	if typ := c.String("type"); typ != "" {
		conf.Storage = config.StorageType(typ)
	}
	if u := c.String("url"); u != "" {
		conf.BaseURL = u
	}

	db, aDb, err := Storage(conf, l)
	l.SetLevel(conf.LogLevel)
	if err != nil {
		l.Errorf("Unable to access storage: %s", err)
		return nil, fmt.Errorf("unable to access %q storage: %s", conf.Storage, conf.StoragePath)
	}
	return New(aDb, db, conf), nil
}

var TuiAction = func(c *cli.Context) error {
	ct, err := setup(c, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return err
	}
	ctl = *ct

	return tui.Launch(ctl.Conf, ctl.Storage, ctl.AuthStorage, logger)
}
