package watermarkutil

import "strings"

type mediaConvertModule struct{}

var MediaConvertModule = mediaConvertModule{}

func (mediaConvertModule) CreateWatermarkURL(contentPath, prefixFolder string, watermarkInfo WatermarkInfo) string {
	const variantDirDepth = 2

	responseURL := "/" + prefixFolder

	queryIndex := strings.LastIndex(contentPath, "?")
	fileNameIndex := strings.LastIndex(contentPath, "/")

	fileName, queryString := contentPath[fileNameIndex+1:], ""
	if queryIndex > -1 {
		fileName = contentPath[min(fileNameIndex+1, queryIndex):max(fileNameIndex+1, queryIndex)]
		queryString = contentPath[queryIndex:]
		contentPath = contentPath[:queryIndex]
	}

	extensionIndex := strings.LastIndex(fileName, ".")
	baseName := fileName[:max(extensionIndex, 0)]
	extension := fileName[extensionIndex+1:]

	firstSegmentNumber := watermarkInfo["first_segment_number"]

	switch extension {
	case "ts":
		if segmentNumber := GetSequenceNumber(baseName); segmentNumber >= 0 {
			renditionSymbol := SelectRenditionSymbol(firstSegmentNumber, segmentNumber, watermarkInfo)
			responseURL += BuildVariantDirectoryPath(contentPath, variantDirDepth, renditionSymbol)
		}
	case "mp4", "m4s":
		if strings.HasSuffix(strings.ToLower(baseName), "init") {
			responseURL += BuildVariantDirectoryPath(contentPath, variantDirDepth, "0")
		} else if segmentNumber := GetSequenceNumber(baseName); segmentNumber >= 0 {
			renditionSymbol := SelectRenditionSymbol(firstSegmentNumber, segmentNumber, watermarkInfo)
			responseURL += BuildVariantDirectoryPath(contentPath, variantDirDepth, renditionSymbol)
		}
	default:
		responseURL += BuildVariantDirectoryPath(contentPath, variantDirDepth, "0")
	}

	if responseURL == "/"+prefixFolder {
		responseURL += BuildVariantDirectoryPath(contentPath, variantDirDepth, "0")
	}
	return responseURL + queryString
}
