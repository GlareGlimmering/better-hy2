package controller

import (
	"crypto/rand"
	"io"
	"math/big"

	"github.com/gin-gonic/gin"
	"h-ui/model/vo"
)

func generateConnectionPassword(source io.Reader) (string, error) {
	const length = 24
	classes := []string{"ABCDEFGHIJKLMNOPQRSTUVWXYZ", "abcdefghijklmnopqrstuvwxyz", "0123456789", "!@#$&*_"}
	var alphabet string
	var password [length]byte
	for _, class := range classes {
		alphabet += class
	}
	pick := func(n int) (int, error) {
		value, err := rand.Int(source, big.NewInt(int64(n)))
		if err != nil {
			return 0, err
		}
		return int(value.Int64()), nil
	}
	for i := range password {
		options := alphabet
		if i < len(classes) {
			options = classes[i]
		}
		index, err := pick(len(options))
		if err != nil {
			return "", err
		}
		password[i] = options[index]
	}
	for i := len(password) - 1; i > 0; i-- {
		j, err := pick(i + 1)
		if err != nil {
			return "", err
		}
		password[i], password[j] = password[j], password[i]
	}
	return string(password[:]), nil
}

func GenerateConnectionPassword(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	password, err := generateConnectionPassword(rand.Reader)
	if err != nil {
		vo.Fail("could not generate connection password", c)
		return
	}
	vo.Success(password, c)
}
