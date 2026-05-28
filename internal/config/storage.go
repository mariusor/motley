package config

import (
	"git.sr.ht/~mariusor/motley/internal/env"
	"git.sr.ht/~mariusor/storage-all"
)

const DefaultStorage = StorageFS

func Open(c Storage, env env.Type) (storage.FullStorage, error) {
	return storage.New(
		storage.WithPath(c.Path),
		storage.WithType(storage.Type(c.Type)),
		storage.WithEnv(string(env)),
	)
}
