package wmt

import (
	"encoding/json"
	"strings"

	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"

	"github.com/doverunner/fwm-cdn-embedder-gcp-service-extensions/watermarkutil"
)

type doveRunnerAES struct{}

var DoveRunnerAES = doveRunnerAES{}

func (doveRunnerAES) GetContentURL(requestPath string, uriSegments []string, prefixFolder string, config watermarkutil.Config, hasRevokeToken bool) (finalRequestPath, statusDescription string) {
	watermarkData := uriSegments[2]

	aesKey, _ := config["aesKey"].(string)
	decWatermarkData, err := watermarkutil.AESDecrypt(watermarkData, aesKey)
	var watermarkInfo watermarkutil.WatermarkInfo
	if err == nil {
		err = json.Unmarshal([]byte(decWatermarkData), &watermarkInfo)
	}
	if err != nil {
		proxywasm.LogErrorf("watermark token rejected : %v", err)
		return "", "Invalid watermark token"
	}
	proxywasm.LogInfo("watermark payload: " + decWatermarkData)

	requestedFileName := requestPath[strings.LastIndex(requestPath, "/")+1:]
	if watermarkutil.FindBlockedPrefix(requestedFileName, watermarkInfo["renditions"]) != "" {
		return "", "Unsafe file path request detected."
	}
	if toBoolean(watermarkInfo["revoke_flag"]) && !hasRevokeToken {
		return "", "Missing required revoke token"
	}
	timestamp, _ := watermarkInfo["timestamp"].(string)
	if !watermarkutil.IsTokenFresh(timestamp, config["availableInterval"]) {
		return "", "The watermark token usage period has expired"
	}
	if _, ok := watermarkInfo["first_segment_number"].(float64); !ok {
		return "", "Missing required first segment number"
	}

	composingType, ok := config["type"].(string)
	if !ok {
		proxywasm.LogErrorf("watermark request rejected: config type is %v, not a string", config["type"])
		return "", "Invalid watermark request"
	}
	var fwmModule interface {
		CreateWatermarkURL(contentPath, prefixFolder string, watermarkInfo watermarkutil.WatermarkInfo) string
	}
	switch strings.ToLower(composingType) {
	case "aws":
		fwmModule = watermarkutil.MediaConvertModule
	case "unlabeled_a_variant":
		fwmModule = watermarkutil.UnlabeledAVariantModule
	default:
		fwmModule = watermarkutil.DirectoryPrefixModule
	}

	contentPath := requestPath[len(watermarkData)+2+len(prefixFolder):]
	return fwmModule.CreateWatermarkURL(contentPath, prefixFolder, watermarkInfo), ""
}

func toBoolean(value any) bool {
	switch value := value.(type) {
	case nil:
		return false
	case bool:
		return value
	case float64:
		return value != 0
	case string:
		return value != ""
	}
	return true
}
