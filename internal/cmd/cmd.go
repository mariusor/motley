package cmd

import (
	tui "git.sr.ht/~marius/motley"
	"git.sr.ht/~marius/motley/internal/config"
	"github.com/sirupsen/logrus"
)

var (
	ctl Control
)

type Control struct {
	Conf config.Options
}

func New(conf config.Options) *Control {
	return &Control{Conf: conf}
}

func ShowTui(conf config.Options, l *logrus.Logger, stores ...config.FullStorage) error {
	ctl = *New(conf)
	return tui.Launch(ctl.Conf, l)
}
