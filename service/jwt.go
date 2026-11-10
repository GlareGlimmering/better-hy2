package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/constant"
	"strings"
	"time"
)

const TokenExpireDuration = time.Hour * 24

type MyClaims struct {
	AccountBo bo.AccountBo `json:"account"`
	jwt.StandardClaims
}

func genToken(accountBo bo.AccountBo, passwordHash, generation string) (string, error) {
	c := MyClaims{
		AccountBo: accountBo,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(TokenExpireDuration).Unix(),
			Issuer:    "h-ui",
		},
	}
	config, err := dao.GetConfig("key = ?", constant.JwtSecret)
	if err != nil {
		return "", errors.New(constant.SysError)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	if config.Value == nil || *config.Value == "" {
		return "", errors.New(constant.SysError)
	}
	return token.SignedString(adminSigningKey(*config.Value, accountBo.Id, accountBo.Username, passwordHash, generation))
}

func adminSigningKey(secret string, id int64, username, passwordHash, generation string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	// Bind the session to the exact credential snapshot verified at login.
	fmt.Fprintf(mac, "better-hy2/admin-session/v1\x00%d\x00%s\x00%s", id, username, passwordHash)
	if generation != "" {
		fmt.Fprintf(mac, "\x00%s", generation)
	}
	return mac.Sum(nil)
}

func ParseToken(tokenString string) (*MyClaims, error) {
	config, err := dao.GetConfig("key = ?", constant.JwtSecret)
	if err != nil || config.Value == nil || *config.Value == "" {
		return nil, errors.New(constant.SysError)
	}
	token, err := jwt.ParseWithClaims(tokenString, &MyClaims{}, func(token *jwt.Token) (i interface{}, err error) {
		claims, ok := token.Claims.(*MyClaims)
		if !ok || token.Method != jwt.SigningMethodHS256 || claims.ExpiresAt == 0 || claims.Issuer != "h-ui" {
			return nil, errors.New(constant.IllegalTokenError)
		}
		account, err := dao.GetAccount("id = ? and role = 'admin' and deleted = 0", claims.AccountBo.Id)
		if err != nil || account.Username == nil || account.Pass == nil {
			return nil, errors.New(constant.IllegalTokenError)
		}
		return adminSigningKey(*config.Value, claims.AccountBo.Id, *account.Username, *account.Pass, account.AuthGeneration), nil
	})
	if err != nil {
		return nil, errors.New(constant.IllegalTokenError)
	}
	if claims, ok := token.Claims.(*MyClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New(constant.TokenExpiredError)
}

func GetToken(c *gin.Context) string {
	scheme, token, ok := strings.Cut(c.Request.Header.Get("Authorization"), " ")
	if !ok || scheme != "Bearer" {
		return ""
	}
	return token
}
