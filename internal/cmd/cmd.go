package cmd

import (
	tui "git.sr.ht/~marius/motley"
	"git.sr.ht/~marius/motley/internal/config"
	"github.com/go-ap/fedbox"
	"github.com/sirupsen/logrus"
)

var (
	ctl Control
)

type Control struct {
	Conf    config.Options
	Storage []fedbox.FullStorage
}

func New(conf config.Options, db ...fedbox.FullStorage) *Control {
	return &Control{
		Conf:    conf,
		Storage: db,
	}
}

func ShowTui(conf config.Options, l *logrus.Logger, stores ...fedbox.FullStorage) error {
	ctl = *New(conf, stores...)

	return tui.Launch(ctl.Conf, ctl.Storage[0], l)
}
