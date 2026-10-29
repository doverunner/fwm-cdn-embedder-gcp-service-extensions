package watermarkutil

import (
	"math"
	"strconv"
	"strings"
)

const indicatorPattern = "10001000011000000110000000000000011111111000000000000001100000011"

const (
	indicatorInterval = 3
	indicatorIndex    = 2
)

var WMPrefixes = []string{"", "b.", "c.", "d."}

const maxSegmentNumber = 1 << 53

type WatermarkInfo map[string]any

func (watermarkInfo WatermarkInfo) number(key string, fallback int) int {
	if number, ok := toInt(watermarkInfo[key]); ok {
		return number
	}
	return fallback
}

func toInt(value any) (int, bool) {
	switch value := value.(type) {
	case float64:
		return int(value), true
	case int:
		return value, true
	}
	return 0, false
}

func isDigit(character byte) bool {
	return '0' <= character && character <= '9'
}

func hasSeparatorBeforeDigit(fileName string) bool {
	for i := 0; i+1 < len(fileName); i++ {
		if (fileName[i] == '_' || fileName[i] == '-') && isDigit(fileName[i+1]) {
			return true
		}
	}
	return false
}

func GetSequenceNumber(fileName string) int {
	if !hasSeparatorBeforeDigit(fileName) {
		return -1
	}
	separatorIndex := max(strings.LastIndex(fileName, "_"), strings.LastIndex(fileName, "-"))
	return ParseInteger(fileName[separatorIndex+1:])
}

func ParseInteger(str string) int {
	value, err := strconv.Atoi(str)
	if err != nil || strings.HasPrefix(str, "+") || value > maxSegmentNumber {
		return -1
	}
	return value
}

func SelectRenditionSymbol(firstSegmentNumber any, segmentNumber int, watermarkInfo WatermarkInfo) string {
	watermarkData, _ := watermarkInfo["watermark_data"].(string)
	syncSkipBit := watermarkInfo.number("sync_skip_bit", -1)
	renditions := watermarkInfo.number("renditions", 2)
	leadIn := watermarkInfo.number("lead_in", 4)

	firstSegment, ok := toInt(firstSegmentNumber)
	if !ok {
		return "0"
	}

	alignedSegmentNumber := segmentNumber
	if syncSkipBit > -1 {
		alignedSegmentNumber += syncSkipBit
	}
	firstMarkedSegment := leadIn + firstSegment
	if alignedSegmentNumber < firstMarkedSegment {
		return "0"
	}

	bitsPerSegment := math.Log2(float64(renditions))
	symbolPattern := string(Base64ToBytes(watermarkData))
	if float64(len(symbolPattern)) < bitsPerSegment {
		return "0"
	}

	slot := alignedSegmentNumber - firstMarkedSegment
	if syncSkipBit > -1 && len(symbolPattern) == len(indicatorPattern) &&
		(slot/len(symbolPattern))%indicatorInterval == indicatorIndex {
		symbolPattern = indicatorPattern
	}

	if math.IsNaN(bitsPerSegment) || bitsPerSegment < 1 {
		return "NaN"
	}
	base := (slot * int(bitsPerSegment)) % len(symbolPattern)
	if bitsPerSegment == 1 {
		return symbolPattern[base : base+1]
	}
	symbol, err := strconv.ParseInt(symbolPattern[base:min(base+int(bitsPerSegment), len(symbolPattern))], 2, 0)
	if err != nil {
		return "NaN"
	}
	return strconv.FormatInt(symbol, 10)
}

func BuildVariantDirectoryPath(contentPath string, trailingSegmentCount int, renditionSymbol string) string {
	var responseURL strings.Builder
	pathSegments := strings.Split(contentPath, "/")
	pathSegmentCount := len(pathSegments)
	for i := 1; i < pathSegmentCount; i++ {
		if i == pathSegmentCount-trailingSegmentCount {
			responseURL.WriteString("/" + renditionSymbol)
		}
		responseURL.WriteString("/" + pathSegments[i])
	}
	return responseURL.String()
}

func BuildVariantFilePath(path, fileName, renditionSymbol string) string {
	prefix := ""
	if index, err := strconv.Atoi(renditionSymbol); err == nil && 0 <= index && index < len(WMPrefixes) {
		prefix = WMPrefixes[index]
	}
	return path + prefix + fileName
}

func FindBlockedPrefix(fileName string, renditions any) string {
	blocked := WMPrefixes[1:2]
	if toNumber(renditions) == 4 {
		blocked = WMPrefixes[1:4]
	}
	for _, prefix := range blocked {
		if strings.HasPrefix(fileName, prefix) {
			return prefix
		}
	}
	return ""
}

func RemoveRevokeToken(uriSegments []string) (modifiedURISegments []string, hasRevokeToken bool) {
	watermarkData := strings.Split(uriSegments[2], ".")
	hasRevokeToken = len(watermarkData) >= 2

	if hasRevokeToken {
		uriSegments[2] = watermarkData[1]
	}
	return uriSegments, hasRevokeToken
}
