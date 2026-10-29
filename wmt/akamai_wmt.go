package wmt

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"

	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"

	"github.com/doverunner/fwm-cdn-embedder-gcp-service-extensions/watermarkutil"
)

type akamaiWMT struct{}

var AkamaiWMT = akamaiWMT{}

func (akamaiWMT) GetContentURL(requestPath string, uriSegments []string, config watermarkutil.Config) (finalRequestPath, statusDescription string) {
	wmtPublicKey, _ := config["wmtPublicKey"].(string)
	wmtPassword, _ := config["wmtPassword"].(string)
	if wmtPublicKey == "" || wmtPassword == "" {
		proxywasm.LogError("Please set wmtPublicKey or wmtPassword in config.json.")
		return "", "Watermark token support is not configured"
	}
	composingType, ok := config["type"].(string)
	if !ok {
		proxywasm.LogErrorf("watermark request rejected: config type is %v, not a string", config["type"])
		return "", "Invalid watermark request"
	}
	if strings.ToLower(composingType) != "unlabeled_a_variant" {
		proxywasm.LogError("AkamaiWmt supports only unlabeled_a_variant.")
		return "", "Watermark token support is not configured"
	}

	jwt := uriSegments[1][len("wmt:"):]
	watermark := parseWatermarkToken(jwt, wmtPublicKey, wmtPassword)
	if watermark == "" {
		return "", "Invalid watermark token"
	}

	requestedFileName := requestPath[strings.LastIndex(requestPath, "/")+1:]
	if watermarkutil.FindBlockedPrefix(requestedFileName, 2) != "" {
		return "", "Unsafe file path request detected."
	}

	contentPath := requestPath[len(uriSegments[1])+1:]
	firstSegmentNumber := 0
	if lowered := strings.ToLower(contentPath); strings.HasSuffix(lowered, ".ts") || strings.Contains(lowered, ".ts?") {
		firstSegmentNumber = 1
	}

	return watermarkutil.UnlabeledAVariantModule.CreateWatermarkURL(contentPath, "", watermarkutil.WatermarkInfo{
		"watermark_data":       base64.StdEncoding.EncodeToString([]byte(watermark)),
		"sync_skip_bit":        -1,
		"renditions":           2,
		"lead_in":              0,
		"first_segment_number": firstSegmentNumber,
	}), ""
}

func parseWatermarkToken(jwt, wmtPublicKey, passwd string) string {
	rejected := func(step string, err error) string {
		proxywasm.LogErrorf("akamai watermark token rejected at %s: %v", step, err)
		return ""
	}

	if err := hasValidSignature(jwt, wmtPublicKey); err != nil {
		return rejected("signature verification", err)
	}
	payload, err := getPayloadFromJWT(jwt)
	if err != nil {
		return rejected("signature verification", err)
	}

	aesKey := sha256.Sum256([]byte(passwd))
	wmidivb64, _ := payload["wmidivb64"].(string)
	wmidctb64, _ := payload["wmidctb64"].(string)
	binaryPattern, err := watermarkutil.AESCBCDecrypt(watermarkutil.Base64ToBytes(wmidctb64), aesKey[:], watermarkutil.Base64ToBytes(wmidivb64))
	if err != nil {
		return rejected("payload decryption", err)
	}

	watermarkData := strings.NewReplacer("A", "0", "a", "0", "B", "1", "b", "1").Replace(string(binaryPattern))
	proxywasm.LogInfo("watermark pattern: " + watermarkData)
	return watermarkData
}

func getSecuredInputFromJWT(jwt string) string {
	return strings.Join(strings.SplitN(jwt, ".", 3)[:2], ".")
}

func getSignatureFromJWT(jwt string) string {
	return strings.Split(jwt, ".")[2]
}

func getPayloadFromJWT(jwt string) (map[string]any, error) {
	var payload map[string]any
	err := json.Unmarshal(watermarkutil.Base64ToBytes(strings.Split(jwt, ".")[1]), &payload)
	return payload, err
}

func hasValidSignature(jwt, wmtPublicKey string) error {
	if strings.Count(jwt, ".") < 2 {
		return errors.New("token is not a signed JWT")
	}
	publicKey, err := parseRSAPublicKey(wmtPublicKey)
	if err != nil {
		return err
	}
	hashed := sha256.Sum256([]byte(getSecuredInputFromJWT(jwt)))
	if rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed[:], watermarkutil.Base64ToBytes(getSignatureFromJWT(jwt))) != nil {
		return errors.New("signature does not match the configured wmtPublicKey")
	}
	return nil
}

var errNotRSAPublicKey = errors.New("wmtPublicKey is not a PEM RSA public key")

func parseRSAPublicKey(wmtPublicKey string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(wmtPublicKey))
	if block == nil || len(block.Headers) > 0 {
		return nil, errNotRSAPublicKey
	}

	rsaPublicKeyBytes := block.Bytes
	switch block.Type {
	case "PUBLIC KEY":
		subjectPublicKeyInfo, _, err := derElement(block.Bytes, derSequence)
		if err != nil {
			return nil, err
		}
		algorithm, afterAlgorithm, err := derElement(subjectPublicKeyInfo, derSequence)
		if err != nil {
			return nil, err
		}
		algorithmIdentifier, _, err := derElement(algorithm, derObjectIdentifier)
		if err != nil || string(algorithmIdentifier) != rsaEncryption {
			return nil, errNotRSAPublicKey
		}
		subjectPublicKey, _, err := derElement(afterAlgorithm, derBitString)
		if err != nil || len(subjectPublicKey) == 0 || subjectPublicKey[0] != 0 {
			return nil, errNotRSAPublicKey
		}
		rsaPublicKeyBytes = subjectPublicKey[1:]
	case "RSA PUBLIC KEY":
	default:
		return nil, errNotRSAPublicKey
	}

	rsaPublicKey, _, err := derElement(rsaPublicKeyBytes, derSequence)
	if err != nil {
		return nil, err
	}
	modulus, afterModulus, err := derElement(rsaPublicKey, derInteger)
	if err != nil {
		return nil, err
	}
	exponent, _, err := derElement(afterModulus, derInteger)
	if err != nil {
		return nil, err
	}

	n := new(big.Int).SetBytes(modulus)
	e := new(big.Int).SetBytes(exponent)
	if n.Sign() <= 0 || e.Sign() <= 0 || !e.IsInt64() || e.Int64() > 1<<31-1 {
		return nil, errNotRSAPublicKey
	}
	return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
}

const (
	derInteger          = 0x02
	derBitString        = 0x03
	derObjectIdentifier = 0x06
	derSequence         = 0x30
)

const rsaEncryption = "\x2a\x86\x48\x86\xf7\x0d\x01\x01\x01"

func derElement(data []byte, tag byte) (content, rest []byte, err error) {
	if len(data) < 2 || data[0] != tag {
		return nil, nil, errNotRSAPublicKey
	}
	length, lengthSize := int(data[1]), 1
	if length&0x80 != 0 {
		lengthSize = length & 0x7f
		if lengthSize == 0 || lengthSize > 4 || len(data) < 2+lengthSize {
			return nil, nil, errNotRSAPublicKey
		}
		length = 0
		for _, lengthByte := range data[2 : 2+lengthSize] {
			length = length<<8 | int(lengthByte)
		}
		lengthSize++
	}
	start := 1 + lengthSize
	if length < 0 || len(data)-start < length {
		return nil, nil, errNotRSAPublicKey
	}
	return data[start : start+length], data[start+length:], nil
}
