package cmd

import (
	"fmt"
	"io"
	"net/url"
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

	l.SetOutput(openlog(c.App.Name))
	l.SetFormatter(&logrus.TextFormatter{DisableQuote: true, DisableTimestamp: true})

	conf := config.Options{
		LogLevel: logrus.DebugLevel,
		Env:      environ,
	}
	var db processing.Store

	if envFile := c.String("config"); envFile != "" {
		var err error
		if conf, err = config.LoadFromEnv(envFile, environ, time.Second); err != nil {
			l.Errorf("Unable to load config file for environment %s: %s", environ, err)
			return nil, fmt.Errorf("unable to load config in path: %s", envFile)
		}
		db, err = Storage(conf, l)
		if err != nil {
			l.Errorf("Unable to access storage: %s", err)
			return nil, fmt.Errorf("unable to access %q storage: %s", conf.Storage, conf.StoragePath)
		}
	} else {
		if u := c.String("url"); u != "" {
			o, err := url.ParseRequestURI(u)
			if err != nil {
				return nil, fmt.Errorf("invalid url passed: %w", err)
			}
			conf.Host = o.Host
			conf.BaseURL = u
		}
		if dir := c.String("path"); dir != "" {
			conf.StoragePath = dir
		}
		if typ := c.String("type"); typ != "" {
			conf.Storage = config.StorageType(typ)
		}
		var err error
		db, err = StorageFromDirectPath(conf, l)
		if err != nil {
			l.Errorf("Unable to access storage: %s", err)
			return nil, fmt.Errorf("unable to access %q storage: %s", conf.Storage, conf.StoragePath)
		}
	}
	//if conf.StoragePath == "" {
	//	return nil, fmt.Errorf("storage path was not passed to the binary")
	//}
	//if conf.Storage == "" {
	//	return nil, fmt.Errorf("storage type was not passed to the binary")
	//}
	//if conf.BaseURL == "" {
	//	return nil, fmt.Errorf("base url was not passed to the binary")
	//}

	return New(db.(osin.Storage), db, conf), nil
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
