package utils

import (
	"time"

	"golang.org/x/exp/rand"
)

func RandomString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	rand.Seed(uint64(time.Now().UnixNano()))
	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
