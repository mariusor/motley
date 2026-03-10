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
	"strings"

	"git.sr.ht/~mariusor/lw"
	"git.sr.ht/~mariusor/motley/internal/cmd"
	"git.sr.ht/~mariusor/motley/internal/config"
	"git.sr.ht/~mariusor/motley/internal/env"
	"git.sr.ht/~mariusor/storage-all"
	"github.com/alecthomas/kong"
)

var version = "HEAD"

var Motley struct {
	Version kong.VersionFlag
	Path    []string `flag:"" name:"path" help:"Storage DSN strings of form type:/path/to/storage. Possible types: ${types}"`
	URL     []string `flag:"" name:"url" help:"The url used by the application."`
}

func openlog(name string) io.Writer {
	f, err := os.OpenFile(filepath.Join("/var/tmp/", name+".log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return io.Discard
	}
	return f
}

func perfEnabled() (string, bool) {
	l := os.Getenv("MOTLEY_PERF_LISTEN")
	return l, len(l) > 0
}

var AppName = "motley"

func main() {
	if build, ok := debug.ReadBuildInfo(); ok && version == "HEAD" && build.Main.Version != "(devel)" {
		version = build.Main.Version
	}

	l := lw.Dev(lw.SetOutput(openlog(AppName)), lw.SetLevel(lw.TraceLevel))

	ktx := kong.Parse(
		&Motley,
		kong.Bind(l),
		kong.Name(AppName),
		kong.Description("Helper utility to manage a FedBOX instance"),
		kong.Vars{
			"envs":    strings.Join([]string{string(env.DEV), string(env.QA), string(env.PROD)}, ", "),
			"types":   strings.Join([]string{string(config.StorageBoltDB), string(config.StorageBadger), string(config.StorageFS)}, ", "),
			"version": version,
		},
	)

	if listenOn, enabled := perfEnabled(); enabled {
		// Server for pprof
		go func() {
			fmt.Println(http.ListenAndServe(listenOn, nil))
		}()
	}

	conf := config.Options{}
	_, err := loadArguments(&conf)
	if err != nil {
		l.Errorf("%s", err)
		_, _ = fmt.Fprintln(os.Stderr, err)
		ktx.Exit(1)
	}

	l.Infof("Started")
	if err := cmd.ShowTui(conf, l); err != nil {
		l.Errorf("%s", err)
		ktx.Exit(1)
	}
	l.Infof("Exiting")
	ktx.Exit(0)
}

func loadArguments(conf *config.Options) ([]storage.FullStorage, error) {
	if len(Motley.Path) == 0 {
		return nil, fmt.Errorf("missing flags: you need to either pass a config path DSN or pairs of a storage DSN with an associated URL")
	}

	errs := make([]error, 0)
	stores := make([]storage.FullStorage, 0)
	for _, u := range Motley.URL {
		if u == "" {
			continue
		}
		_, err := url.ParseRequestURI(u)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid url passed: %s", err))
			continue
		}
		conf.URLs = append(conf.URLs, u)
	}
	for _, sto := range Motley.Path {
		if sto == "" {
			continue
		}
		typ, path := config.ParseStorageDSN(sto)
		st := config.Storage{
			Type: typ,
			Path: filepath.Clean(path),
		}
		if !validStorageType(st.Type) {
			errs = append(errs, fmt.Errorf("invalid storage type value %s", conf.Storage))
			continue
		}
		conf.Storage = append(conf.Storage, st)
	}
	if len(errs) > 0 {
		return nil, xerrors.Join(errs...)
	}

	return stores, nil
}

func validStorageType(t config.StorageType) bool {
	return t == config.StorageFS || t == config.StorageSqlite || t == config.StorageBoltDB || t == config.StorageBadger
}
