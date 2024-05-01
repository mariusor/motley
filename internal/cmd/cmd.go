package cmd

import (
	"git.sr.ht/~mariusor/lw"
	tui "git.sr.ht/~mariusor/motley"
	"git.sr.ht/~mariusor/motley/internal/config"
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

func ShowTui(conf config.Options, l lw.Logger) error {
	ctl = *New(conf)
	return tui.Launch(ctl.Conf, l)
}
