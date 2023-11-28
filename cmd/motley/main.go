package main

import (
	xerrors "errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.sr.ht/~marius/motley/internal/cmd"
	"git.sr.ht/~marius/motley/internal/config"
	"git.sr.ht/~marius/motley/internal/env"
	"github.com/alecthomas/kong"
	"github.com/go-ap/fedbox"
	"github.com/sirupsen/logrus"
)

var version = "HEAD"

var Motley struct {
	Path   []string `flag:"" name:"path" help:"Storage DSN strings of form type:/path/to/storage. Possible types: ${types}"`
	URL    []string `flag:"" name:"url" help:"The url used by the application."`
	Config []string `flag:"" name:"config" help:"DSN for the folder(s) containing .env config files of form: env:/path/to/config. Possible types: ${envs}"`
}

var l = logrus.New()

func openlog(name string) io.Writer {
	f, err := os.OpenFile(filepath.Join("/var/tmp/", name+".log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return io.Discard
	}
	return f
}

func main() {
	l.SetOutput(openlog("motley"))
	l.SetFormatter(&logrus.TextFormatter{DisableQuote: true, DisableTimestamp: true})

	ktx := kong.Parse(
		&Motley,
		kong.Bind(l),
		kong.Name("motley"),
		kong.Description("Helper utility to manage a FedBOX instance"),
		kong.Vars{
			"envs":  strings.Join([]string{string(env.DEV), string(env.PROD)}, ", "),
			"types": strings.Join([]string{string(config.StorageBoltDB), string(config.StorageBadger), string(config.StorageFS)}, ", "),
		},
	)

	conf := config.Options{}
	stores, err := loadArguments(&conf)
	if err != nil {
		l.Errorf("%s", err)
		fmt.Fprintln(os.Stderr, err)
		ktx.Exit(1)
	}

	l.Infof("Started")
	if err := cmd.ShowTui(conf, l, stores...); err != nil {
		l.Errorf("%s", err)
		ktx.Exit(1)
	}
	l.Infof("Exiting")
	ktx.Exit(0)
}

type multiError []error

func (m multiError) Error() string {
	s := strings.Builder{}
	for i, e := range m {
		s.WriteString(e.Error())
		if i < len(m)-1 {
			s.WriteString("\n")
		}
	}
	return s.String()
}

func loadArguments(conf *config.Options) ([]fedbox.FullStorage, error) {
	if len(Motley.Config)+len(Motley.Path) == 0 {
		return nil, fmt.Errorf("missing flags: you need to either pass a config path DSN or pairs of a storage DSN with an associated URL")
	}
	if len(Motley.URL)+len(Motley.Path) > 0 && len(Motley.URL) != len(Motley.Path) {
		return nil, fmt.Errorf("invalid flags: when passing storage DSN you need an associated URL for each of them")
	}

	m := make([]error, 0)
	stores := make([]fedbox.FullStorage, 0)
	for _, dsnConfig := range Motley.Config {
		if dsnConfig == "" {
			continue
		}
		typ, envFile, found := strings.Cut(dsnConfig, ":")
		if !found {
			m = append(m, fmt.Errorf("invalid config DSN value, expected env:/path/to/config"))
			continue
		}
		envTyp := env.Type(typ)
		if !env.ValidType(envTyp) {
			m = append(m, fmt.Errorf("invalid env in the DSN value %q", envTyp))
			continue
		}
		c, err := config.LoadFromEnv(envFile, envTyp, time.Second)
		if err != nil {
			m = append(m, fmt.Errorf("unable to load config file for environment %s: %s", c.Env, err))
			continue
		}
		db, err := cmd.Storage(c, l)
		if err != nil {
			m = append(m, fmt.Errorf("unable to access storage: %s", err))
			continue
		}

		o, err := url.ParseRequestURI(c.BaseURL)
		if err != nil {
			m = append(m, fmt.Errorf("invalid url passed: %s", err))
			continue
		}
		conf.Host = o.Host

		*conf = c
		stores = append(stores, db)
	}
	for i, sto := range Motley.Path {
		if sto == "" {
			continue
		}
		typ, path, found := strings.Cut(sto, ":")
		if !found {
			m = append(m, fmt.Errorf("invalid storage value, expected DSN of type:/path/to/storage"))
			continue
		}

		conf.Storage = config.StorageType(typ)
		conf.StoragePath = path

		if !validStorageType(conf.Storage) {
			m = append(m, fmt.Errorf("invalid storage type value %s", conf.Storage))
			continue
		}
		if u := Motley.URL[i]; u != "" {
			o, err := url.ParseRequestURI(u)
			if err != nil {
				m = append(m, fmt.Errorf("invalid url passed: %s", err))
				continue
			}
			conf.Host = o.Host
			conf.BaseURL = u
			conf.StoragePath = filepath.Clean(strings.Replace(conf.StoragePath, conf.Host, "", -1))
		}

		db, err := cmd.StorageFromDirectPath(*conf, l)
		if err != nil {
			m = append(m, fmt.Errorf("unable to access storage: %s", err))
			continue
		}

		if err != nil {
			m = append(m, fmt.Errorf("unable to initialize storage backend: %s", err))
			continue
		}
		stores = append(stores, db)
	}
	if len(m) > 0 {
		return nil, xerrors.Join(m...)
	}
	return stores, nil
}

func validStorageType(t config.StorageType) bool {
	return t == config.StorageFS || t == config.StorageSqlite || t == config.StorageBoltDB || t == config.StorageBadger
}
