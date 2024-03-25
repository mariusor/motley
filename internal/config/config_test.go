package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"git.sr.ht/~mariusor/motley/internal/env"
)

const (
	hostname = "testing.git"
	logLvl   = "panic"
	pgSQL    = "postgres"
	boltDB   = "boltdb"
	dbHost   = "127.0.0.6"
	dbPort   = 54321
	dbName   = "test"
	dbUser   = "test"
	dbPw     = "pw123+-098"
)

func TestLoadFromEnv(t *testing.T) {
	{
		t.Skipf("we're no longer loading SQL db config env variables")
		os.Setenv(KeyDBHost, dbHost)
		os.Setenv(KeyDBPort, fmt.Sprintf("%d", dbPort))
		os.Setenv(KeyDBName, dbName)
		os.Setenv(KeyDBUser, dbUser)
		os.Setenv(KeyDBPw, dbPw)

		os.Setenv(KeyLogLevel, logLvl)
		os.Setenv(KeyStorage, pgSQL)

		var baseURL = fmt.Sprintf("https://%s", hostname)
		c, err := LoadFromEnv(".", env.TEST, time.Second)
		if err != nil {
			t.Errorf("Error loading env: %s", err)
		}
		// @todo(marius): we're no longer loading SQL db config env variables
		db := BackendConfig{}
		if db.Host != dbHost {
			t.Errorf("Invalid loaded value for %s: %s, expected %s", KeyDBHost, db.Host, dbHost)
		}
		if db.Port != dbPort {
			t.Errorf("Invalid loaded value for %s: %d, expected %d", KeyDBPort, db.Port, dbPort)
		}
		if db.Name != dbName {
			t.Errorf("Invalid loaded value for %s: %s, expected %s", KeyDBName, db.Name, dbName)
		}
		if db.User != dbUser {
			t.Errorf("Invalid loaded value for %s: %s, expected %s", KeyDBUser, db.User, dbUser)
		}
		if db.Pw != dbPw {
			t.Errorf("Invalid loaded value for %s: %s, expected %s", KeyDBPw, db.Pw, dbPw)
		}

		for _, st := range c.Storage {
			if st.Type != pgSQL {
				t.Errorf("Invalid loaded value for %s: %s, expected %s", KeyStorage, st.Type, pgSQL)
			}
		}
		for _, u := range c.URLs {
			if u != baseURL {
				t.Errorf("Invalid loaded BaseURL value: %s, expected %s", u, baseURL)
			}
		}
	}
	{
		os.Setenv(KeyStorage, boltDB)
		c, err := LoadFromEnv(".", env.TEST, time.Second)
		if err != nil {
			t.Errorf("Error loading env: %s", err)
		}
		var tmp = strings.TrimRight(os.TempDir(), "/")
		for i, st := range c.Storage {
			if strings.TrimRight(st.Path, "/") != tmp {
				t.Errorf("Invalid loaded boltdb dir value: %s, expected %s", st.Path, tmp)
			}
			var expected = fmt.Sprintf("%s/%s-%s.bdb", tmp, strings.Replace(hostname, ".", "-", 1), env.TEST)
			u, _ := url.ParseRequestURI(c.URLs[i])
			p, err := st.BoltDB(c.Env, u.Host)
			if err != nil {
				t.Errorf("BoltDB() errored: %s", err)
			}
			if p != expected {
				t.Errorf("Invalid loaded boltdb file value: %s, expected %s", p, expected)
			}
		}
	}
}
