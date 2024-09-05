package env

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func MustGet(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		log.Fatal("Could not find " + key + ", please set env")
	}

	return value
}
