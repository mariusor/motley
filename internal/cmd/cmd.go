package cmd

import (
	"io"
	"os"
	"path/filepath"

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

func openlog(name string) io.Writer {
	f, err := os.OpenFile(filepath.Join("/var/tmp/", name+".log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return io.Discard
	}
	return f
}

func ShowTui(conf config.Options, l *logrus.Logger, stores ...fedbox.FullStorage) error {
	l.SetOutput(openlog("motley"))
	l.SetFormatter(&logrus.TextFormatter{DisableQuote: true, DisableTimestamp: true})

	ctl = *New(conf, stores...)

	return tui.Launch(ctl.Conf, ctl.Storage[0], l)
}
