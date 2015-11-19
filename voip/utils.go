package voip

import (
	"crypto/rand"
	"fmt"
)

// TODO: unique mac?
func GetMacAddress() (string, error) {
	buf := make([]byte, 3)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("00:16:3e:%02x:%02x:%02x", buf[0], buf[1], buf[2]), nil
}
