package main

import (
	xerrors "errors"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"git.sr.ht/~mariusor/lw"
	"git.sr.ht/~mariusor/motley/internal/cmd"
	"git.sr.ht/~mariusor/motley/internal/config"
	"git.sr.ht/~mariusor/motley/internal/env"
	"github.com/alecthomas/kong"
)

var version = "HEAD"

var Motley struct {
	Version kong.VersionFlag
	Path    []string `flag:"" name:"path" help:"Storage DSN strings of form type:/path/to/storage. Possible types: ${types}"`
	URL     []string `flag:"" name:"url" help:"The url used by the application."`
	Config  []string `flag:"" name:"config" help:"DSN for the folder(s) containing .env config files of form: env:/path/to/config. Possible types: ${envs}"`
}

func openlog(name string) io.Writer {
	f, err := os.OpenFile(filepath.Join("/var/tmp/", name+".log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return io.Discard
	}
	return f
}

func perfEnabled() bool {
	val, err := strconv.ParseBool(os.Getenv("MOTLEY_ENABLE_PERF"))
	if err != nil {
		return false
	}
	return val
}

func main() {
	if build, ok := debug.ReadBuildInfo(); ok && version == "HEAD" && build.Main.Version != "(devel)" {
		version = build.Main.Version
	}

	l := lw.Dev(lw.SetOutput(openlog("motley")), lw.SetLevel(lw.TraceLevel))

	ktx := kong.Parse(
		&Motley,
		kong.Bind(l),
		kong.Name("motley"),
		kong.Description("Helper utility to manage a FedBOX instance"),
		kong.Vars{
			"envs":    strings.Join([]string{string(env.DEV), string(env.PROD)}, ", "),
			"types":   strings.Join([]string{string(config.StorageBoltDB), string(config.StorageBadger), string(config.StorageFS)}, ", "),
			"version": version,
		},
	)

	if perfEnabled() {
		// Server for pprof
		go func() {
			fmt.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

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

func loadArguments(conf *config.Options) ([]config.FullStorage, error) {
	if len(Motley.Config)+len(Motley.Path) == 0 {
		return nil, fmt.Errorf("missing flags: you need to either pass a config path DSN or pairs of a storage DSN with an associated URL")
	}

	m := make([]error, 0)
	stores := make([]config.FullStorage, 0)
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
		*conf = c
	}
	for _, u := range Motley.URL {
		if u == "" {
			continue
		}
		_, err := url.ParseRequestURI(u)
		if err != nil {
			m = append(m, fmt.Errorf("invalid url passed: %s", err))
			continue
		}
		conf.URLs = append(conf.URLs, u)
	}
	for _, sto := range Motley.Path {
		if sto == "" {
			continue
		}
		typ, path, found := strings.Cut(sto, ":")
		if !found {
			m = append(m, fmt.Errorf("invalid storage value, expected DSN of type:/path/to/storage"))
			continue
		}
		st := config.Storage{
			Type: config.StorageType(typ),
			Path: path,
		}
		if !validStorageType(st.Type) {
			m = append(m, fmt.Errorf("invalid storage type value %s", conf.Storage))
			continue
		}
		conf.Storage = append(conf.Storage, st)
	}
	if len(m) > 0 {
		return nil, xerrors.Join(m...)
	}
	return stores, nil
}

func validStorageType(t config.StorageType) bool {
	return t == config.StorageFS || t == config.StorageSqlite || t == config.StorageBoltDB || t == config.StorageBadger
}
