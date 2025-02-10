package authorization

import (
	"crypto/rand"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenPathEnvVar = "KUBERNETES_SERVICE_TOKEN_PATH"

	defaultTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

var inMemoryToken *[]byte

func CreateToken() (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	secretKey, err := getSecretKey()
	if err != nil {
		return "", fmt.Errorf("error getting secret key: %v", err)
	}

	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", fmt.Errorf("error signing token: %v", err)
	}

	return tokenString, nil
}

func ValidateToken(tokenString string) (bool, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		return getSecretKey()
	})
	if err != nil {
		return false, fmt.Errorf("error parsing token: %v", err)
	}

	return token.Valid, nil
}

func getSecretKey() ([]byte, error) {
	if inMemoryToken != nil {
		return *inMemoryToken, nil
	}

	tokenPath := os.Getenv(TokenPathEnvVar)
	if tokenPath == "" {
		tokenPath = defaultTokenPath
	}

	token, err := os.ReadFile(tokenPath)
	if err != nil {
		// if ServiceAccount token is not available, generate an in-memory token
		return generateInMemoryToken()
	}

	return token, nil
}

func generateInMemoryToken() ([]byte, error) {
	tokenBytes := make([]byte, 32)

	_, err := rand.Read(tokenBytes)
	if err != nil {
		return nil, fmt.Errorf("error generating in-memory token: %v", err)
	}

	inMemoryToken = &tokenBytes
	return tokenBytes, nil
}
