package config

import (
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"git.sr.ht/~mariusor/lw"
	"git.sr.ht/~mariusor/motley/internal/env"
	"github.com/go-ap/errors"
	"github.com/joho/godotenv"
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

type Storage struct {
	Env  env.Type
	Type StorageType
	Path string
	Host string
}

type Options struct {
	LogLevel lw.Level
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

var allStorageTypes = []string{
	string(StorageFS), string(StorageBoltDB),
	string(StorageBadger), string(StorageSqlite),
}

const (
	varEnv     = "%env%"
	varStorage = "%storage%"
	varHost    = "%host%"
)

func normalizeConfigPath(c *Storage, e env.Type) string {
	if len(c.Path) == 0 {
		return c.Path
	}
	p := c.Path
	if p[0] == '~' {
		if u, err := user.Current(); err == nil {
			p = strings.Replace(p, "~", u.HomeDir, 1)
		} else {
			p = os.Getenv("HOME") + p[1:]
		}
	}
	if !filepath.IsAbs(p) {
		p, _ = filepath.Abs(p)
	}
	p = strings.ReplaceAll(p, varEnv, string(e))
	p = strings.ReplaceAll(p, varStorage, string(c.Type))
	if u, err := url.ParseRequestURI(c.Host); err == nil {
		p = strings.ReplaceAll(p, varHost, url.PathEscape(u.Host))
	}
	c.Path = p
	return filepath.Clean(p)
}

func fullStoragePath(o Storage, e env.Type) (string, error) {
	dir := normalizeConfigPath(&o, e)
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

func (o Storage) BaseStoragePath(e env.Type) (string, error) {
	return fullStoragePath(o, e)
}

func (o Storage) BoltDB(e env.Type) (string, error) {
	base, err := o.BaseStoragePath(e)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/fedbox.bdb", base), nil
}

func (o Storage) BoltDBOAuth2(e env.Type) (string, error) {
	base, err := o.BaseStoragePath(e)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/oauth.bdb", base), nil
}

func (o Storage) Badger(e env.Type) (string, error) {
	return o.BaseStoragePath(e)
}

func (o Storage) BadgerOAuth2(e env.Type) (string, error) {
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
		conf.LogLevel = lw.TraceLevel
	case "debug":
		conf.LogLevel = lw.DebugLevel
	case "warn":
		conf.LogLevel = lw.WarnLevel
	case "error":
		conf.LogLevel = lw.ErrorLevel
	case "info":
		fallthrough
	default:
		conf.LogLevel = lw.InfoLevel
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
	if host := loadKeyFromEnv(KeyHostname, ""); host != "" {
		conf.URLs = append(conf.URLs, fmt.Sprintf("https://%s", host))
	}

	envStorage := loadKeyFromEnv(KeyStorage, string(StorageFS))
	st := Storage{
		Env:  e,
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

func ParseStorageDSN(s string) (StorageType, string) {
	r := regexp.MustCompile(fmt.Sprintf(`(%s):\/(.+)`, strings.Join(allStorageTypes, "|")))
	found := r.FindAllSubmatch([]byte(s), -1)
	if len(found) == 0 {
		return DefaultStorage, s
	}
	sto := found[0]
	if len(sto) == 1 {
		return DefaultStorage, string(sto[1])
	}
	return StorageType(sto[1]), string(sto[2])
}
