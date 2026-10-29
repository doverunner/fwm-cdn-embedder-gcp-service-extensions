package watermarkutil

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
)

const aesIV = "0123456789abcdef"

var Now = time.Now

func AESDecrypt(watermarkData, aesKey string) (string, error) {
	result, err := AESCBCDecrypt(Base64ToBytes(watermarkData), []byte(aesKey), []byte(aesIV))
	if err != nil {
		return "", err
	}
	return string(result), nil
}

func IsTokenFresh(timestamp string, interval any) bool {
	if interval == 0.0 {
		return true
	}
	checkTime, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return false
	}
	period := float64(Now().UnixMilli()-checkTime.UnixMilli()) / 1000
	return toNumber(interval)*60 > period && period > 0
}

func toNumber(value any) float64 {
	switch value := value.(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case string:
		text := strings.Trim(value, " \t\n\v\f\r")
		if text == "" {
			return 0
		}
		if strings.Trim(text, "0123456789+-.eE") != "" && strings.TrimLeft(text, "+-") != "Infinity" {
			return math.NaN()
		}
		if number, err := strconv.ParseFloat(text, 64); err == nil {
			return number
		}
	}
	return math.NaN()
}

func AESCBCDecrypt(ciphertext, key, iv []byte) ([]byte, error) {
	if len(key) != aesKeyLength {
		return nil, errors.New("invalid key length")
	}
	block, _ := aes.NewCipher(key)
	if len(iv) != aes.BlockSize {
		return nil, errors.New("invalid initialization vector length")
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("wrong final block length")
	}

	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, ciphertext)

	padding := int(plaintext[len(plaintext)-1])
	if padding == 0 || padding > aes.BlockSize {
		return nil, errors.New("bad decrypt")
	}
	for _, paddingByte := range plaintext[len(plaintext)-padding:] {
		if int(paddingByte) != padding {
			return nil, errors.New("bad decrypt")
		}
	}
	return plaintext[:len(plaintext)-padding], nil
}

func Base64ToBytes(b64 string) []byte {
	b64, _, _ = strings.Cut(b64, "=")
	cleaned := strings.Map(func(character rune) rune {
		switch {
		case character == '-':
			return '+'
		case character == '_':
			return '/'
		case character >= 'A' && character <= 'Z', character >= 'a' && character <= 'z',
			character >= '0' && character <= '9', character == '+', character == '/':
			return character
		}
		return -1
	}, b64)
	if len(cleaned)%4 == 1 {
		cleaned = cleaned[:len(cleaned)-1]
	}
	decoded, err := base64.RawStdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil
	}
	return decoded
}
