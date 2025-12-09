package service

import (
	crand "crypto/rand"
	mrand "math/rand"
	"time"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_"
const alphabetSize = byte(len(alphabet))

func generateCode(n int) string {
	if n <= 0 {
		return ""
	}

	buf := make([]byte, n)

	if _, err := crand.Read(buf); err == nil {
		for i, b := range buf {
			buf[i] = alphabet[int(b)%int(alphabetSize)]
		}
		return string(buf)
	}

	src := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	for i := range buf {
		buf[i] = alphabet[src.Intn(int(alphabetSize))]
	}
	return string(buf)
}
