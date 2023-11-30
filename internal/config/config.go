package config

import (
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"git.sr.ht/~marius/motley/internal/env"
	"github.com/go-ap/errors"
	"github.com/go-ap/fedbox/storage"
	"github.com/go-ap/processing"
	"github.com/joho/godotenv"
	"github.com/openshift/osin"
	"github.com/sirupsen/logrus"
)

var Prefix = "fedbox"

type BackendConfig struct {
	Enabled bool
	Host    string
	Port    int64
	User    string
	Pw      string
	Name    string
}

type FullStorage interface {
	ListClients() ([]osin.Client, error)
	GetClient(id string) (osin.Client, error)
	UpdateClient(c osin.Client) error
	CreateClient(c osin.Client) error
	RemoveClient(id string) error
	osin.Storage
	processing.Store
	processing.KeyLoader
	storage.PasswordChanger
}

type Storage struct {
	Type StorageType
	Path string
}

type Options struct {
	Env      env.Type
	LogLevel logrus.Level
	URLs     []string
	Storage  []Storage
}

type StorageType string

const (
	KeyENV         = "ENV"
	KeyLogLevel    = "LOG_LEVEL"
	KeyHostname    = "HOSTNAME"
	KeyDBHost      = "DB_HOST"
	KeyDBPort      = "DB_PORT"
	KeyDBName      = "DB_NAME"
	KeyDBUser      = "DB_USER"
	KeyDBPw        = "DB_PASSWORD"
	KeyStorage     = "STORAGE"
	KeyStoragePath = "STORAGE_PATH"
	StorageFS      = StorageType("fs")
	StorageSqlite  = StorageType("sqlite")
	StorageBoltDB  = StorageType("boltdb")
	StorageBadger  = StorageType("badger")
)

const defaultPerm = os.ModeDir | os.ModePerm | 0700

func FullStoragePath(dir string) (string, error) {
	if strings.Contains(dir, "~") {
		if u, err := user.Current(); err == nil {
			dir = strings.Replace(dir, "~", u.HomeDir, 1)
		}
	}
	if !filepath.IsAbs(dir) {
		d, err := filepath.Abs(dir)
		if err != nil {
			return dir, err
		}
		dir = d
	}

	fi, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !fi.IsDir() {
		return "", errors.NotValidf("path %s is invalid for storage", dir)
	}
	return dir, nil
}

func (o Storage) BaseStoragePath(e env.Type, host string) (string, error) {
	if u, err := url.ParseRequestURI(host); err == nil {
		host = u.Host
	}
	return FullStoragePath(filepath.Join(o.Path, string(o.Type), string(e), host))
}

func (o Storage) BoltDB(e env.Type, host string) (string, error) {
	base, err := o.BaseStoragePath(e, host)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/fedbox.bdb", base), nil
}

func (o Storage) BoltDBOAuth2(e env.Type, host string) (string, error) {
	base, err := o.BaseStoragePath(e, host)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/oauth.bdb", base), nil
}

func (o Storage) Badger(e env.Type, host string) (string, error) {
	return o.BaseStoragePath(e, host)
}

func (o Storage) BadgerOAuth2(e env.Type, _ string) (string, error) {
	return fmt.Sprintf("%s/%s/%s", o.Path, e, "oauth"), nil
}

func prefKey(k string) string {
	if Prefix != "" {
		return fmt.Sprintf("%s_%s", strings.ToUpper(Prefix), k)
	}
	return k
}

func loadKeyFromEnv(name, def string) string {
	if val := os.Getenv(prefKey(name)); len(val) > 0 {
		return val
	}
	if val := os.Getenv(name); len(val) > 0 {
		return val
	}
	return def
}

func LoadFromEnv(base string, e env.Type, timeOut time.Duration) (Options, error) {
	conf := Options{}
	if !env.ValidType(e) {
		e = env.Type(loadKeyFromEnv(KeyENV, ""))
	}
	lvl := loadKeyFromEnv(KeyLogLevel, "")
	switch strings.ToLower(lvl) {
	case "trace":
		conf.LogLevel = logrus.TraceLevel
	case "debug":
		conf.LogLevel = logrus.DebugLevel
	case "warn":
		conf.LogLevel = logrus.WarnLevel
	case "error":
		conf.LogLevel = logrus.ErrorLevel
	case "info":
		fallthrough
	default:
		conf.LogLevel = logrus.InfoLevel
	}

	if strings.Contains(base, "~") {
		if u, err := user.Current(); err == nil {
			base = strings.Replace(base, "~", u.HomeDir, 1)
		}
	}
	configs := []string{
		filepath.Clean(filepath.Join(base, ".env")),
	}
	appendIfFile := func(typ env.Type) {
		envFile := filepath.Clean(filepath.Join(base, fmt.Sprintf(".env.%s", typ)))
		if _, err := os.Stat(envFile); err == nil {
			configs = append(configs, envFile)
		}
	}
	if !env.ValidType(e) {
		for _, typ := range env.Types {
			appendIfFile(typ)
		}
	} else {
		appendIfFile(e)
	}
	loadedConfig := false
	for _, f := range configs {
		err := godotenv.Overload(f)
		loadedConfig = loadedConfig || err == nil
	}
	if !loadedConfig {
		return conf, fmt.Errorf("unable to find any configuration files")
	}

	if !env.ValidType(e) {
		e = env.Type(loadKeyFromEnv(KeyENV, "dev"))
	}
	conf.Env = e
	if host := loadKeyFromEnv(KeyHostname, ""); host != "" {
		conf.URLs = append(conf.URLs, fmt.Sprintf("https://%s", host))
	}

	envStorage := loadKeyFromEnv(KeyStorage, string(StorageFS))
	st := Storage{
		Type: StorageType(strings.ToLower(envStorage)),
		Path: loadKeyFromEnv(KeyStoragePath, ""),
	}
	if st.Path == "" {
		st.Path = base
	}
	st.Path = filepath.Clean(st.Path)
	conf.Storage = append(conf.Storage, st)

	return conf, nil
}
