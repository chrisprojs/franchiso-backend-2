package utils

import (
    "crypto/md5"
    "encoding/hex"
    "encoding/json"
    "fmt"
)

func GenerateCacheKey[T any](prefix string, req T) (string, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	
	hash := md5.Sum(b)
	return fmt.Sprintf("%s:%s", prefix, hex.EncodeToString(hash[:])), nil
}