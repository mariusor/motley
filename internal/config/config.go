package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"git.sr.ht/~marius/motley/internal/env"
	"github.com/go-ap/errors"
	"github.com/joho/godotenv"
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

type Options struct {
	Env         env.Type
	LogLevel    logrus.Level
	TimeOut     time.Duration
	Secure      bool
	CertPath    string
	KeyPath     string
	Host        string
	Listen      string
	BaseURL     string
	Storage     StorageType
	StoragePath string
}

type StorageType string

const (
	KeyENV         = "ENV"
	KeyTimeOut     = "TIME_OUT"
	KeyLogLevel    = "LOG_LEVEL"
	KeyHostname    = "HOSTNAME"
	KeyHTTPS       = "HTTPS"
	KeyCertPath    = "CERT_PATH"
	KeyKeyPath     = "KEY_PATH"
	KeyListen      = "LISTEN"
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

func (o Options) BaseStoragePath() (string, error) {
	return FullStoragePath(filepath.Join(o.StoragePath, string(o.Storage), string(o.Env), o.Host))
}

func (o Options) BoltDB() (string, error) {
	base, err := o.BaseStoragePath()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/fedbox.bdb", base), nil
}

func (o Options) BoltDBOAuth2() (string, error) {
	base, err := o.BaseStoragePath()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/oauth.bdb", base), nil
}

func (o Options) Badger() (string, error) {
	return o.BaseStoragePath()
}

func (o Options) BadgerOAuth2() (string, error) {
	return fmt.Sprintf("%s/%s/%s", o.StoragePath, o.Env, "oauth"), nil
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
	if conf.Host == "" {
		conf.Host = loadKeyFromEnv(KeyHostname, conf.Host)
	}
	conf.TimeOut = timeOut
	if to, _ := time.ParseDuration(loadKeyFromEnv(KeyTimeOut, "")); to > 0 {
		conf.TimeOut = to
	}
	if conf.Host != "" {
		conf.Secure, _ = strconv.ParseBool(loadKeyFromEnv(KeyHTTPS, "false"))
		if conf.Secure {
			conf.BaseURL = fmt.Sprintf("https://%s", conf.Host)
		} else {
			conf.BaseURL = fmt.Sprintf("http://%s", conf.Host)
		}
	}
	conf.KeyPath = loadKeyFromEnv(KeyKeyPath, "")
	conf.CertPath = loadKeyFromEnv(KeyCertPath, "")

	conf.Listen = loadKeyFromEnv(KeyListen, "")
	envStorage := loadKeyFromEnv(KeyStorage, string(StorageFS))
	conf.Storage = StorageType(strings.ToLower(envStorage))
	conf.StoragePath = loadKeyFromEnv(KeyStoragePath, "")
	if conf.StoragePath == "" {
		conf.StoragePath = base
	}
	conf.StoragePath = filepath.Clean(conf.StoragePath)

	return conf, nil
}
