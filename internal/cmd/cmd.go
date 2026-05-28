package cmd

import (
	"log/slog"

	tui "git.sr.ht/~mariusor/motley"
	"git.sr.ht/~mariusor/motley/internal/config"
)

var ctl Control

type Control struct {
	Conf config.Options
}

func New(conf config.Options) *Control {
	return &Control{Conf: conf}
}

func ShowTui(conf config.Options, l *slog.Logger) error {
	ctl = *New(conf)
	return tui.Launch(ctl.Conf, l)
}
