package watermarkutil

import "strings"

type unlabeledAVariantModule struct{}

var UnlabeledAVariantModule = unlabeledAVariantModule{}

func (unlabeledAVariantModule) CreateWatermarkURL(contentPath, prefixFolder string, watermarkInfo WatermarkInfo) string {
	responseURL := ""
	if prefixFolder != "" {
		responseURL = "/" + prefixFolder
	}

	queryIndex := strings.LastIndex(contentPath, "?")
	fileNameIndex := strings.LastIndex(contentPath, "/")
	directoryPath := contentPath[:fileNameIndex+1]

	fileName, queryString := contentPath[fileNameIndex+1:], ""
	if queryIndex > -1 {
		fileName = contentPath[min(fileNameIndex+1, queryIndex):max(fileNameIndex+1, queryIndex)]
		queryString = contentPath[queryIndex:]
	}

	extensionIndex := strings.LastIndex(fileName, ".")
	baseName := fileName[:max(extensionIndex, 0)]
	extension := fileName[extensionIndex+1:]

	markedFileURL := ""
	firstSegmentNumber := watermarkInfo["first_segment_number"]

	switch extension {
	case "ts":
		if segmentNumber := GetSequenceNumber(baseName); segmentNumber >= 0 {
			renditionSymbol := SelectRenditionSymbol(firstSegmentNumber, segmentNumber, watermarkInfo)
			markedFileURL = BuildVariantFilePath(directoryPath, fileName, renditionSymbol)
		}
	case "mp4", "m4s":
		if strings.HasSuffix(baseName, "_init") {
			break
		}
		if segmentNumber := GetSequenceNumber(baseName); segmentNumber >= 0 {
			renditionSymbol := SelectRenditionSymbol(firstSegmentNumber, segmentNumber, watermarkInfo)
			markedFileURL = BuildVariantFilePath(directoryPath, fileName, renditionSymbol)
		}
	}

	if markedFileURL == "" {
		markedFileURL = directoryPath + fileName
	}
	return responseURL + markedFileURL + queryString
}
