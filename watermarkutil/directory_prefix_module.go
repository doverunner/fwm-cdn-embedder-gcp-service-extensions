package watermarkutil

import (
	"slices"
	"strings"
)

var flagFolders = []string{"dash", "hls", "cmaf"}

type directoryPrefixModule struct{}

var DirectoryPrefixModule = directoryPrefixModule{}

func (directoryPrefixModule) CreateWatermarkURL(contentPath, prefixFolder string, watermarkInfo WatermarkInfo) string {
	variantDirDepth := struct{ mediaSegment, subtitle, manifest int }{mediaSegment: 5, subtitle: 4, manifest: 2}

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
	directoryPath := contentPath[:min(max(fileNameIndex, 0), len(contentPath))]

	firstSegmentNumber := watermarkInfo["first_segment_number"]

	switch extension {
	case "mpd":
		responseURL += BuildVariantDirectoryPath(contentPath, variantDirDepth.manifest, "0")
	case "m3u8":
		switch {
		case strings.Contains(fileName, "master"):
			responseURL += BuildVariantDirectoryPath(contentPath, variantDirDepth.manifest, "0")
		case fileName == "stream.m3u8" || fileName == "iframes.m3u8":
			responseURL += BuildVariantDirectoryPath(contentPath, variantDirDepth.mediaSegment, "0")
		case fileName == "subtitle.m3u8":
			responseURL += BuildVariantDirectoryPath(contentPath, variantDirDepth.subtitle, "0")
		}
	case "ts":
		if segmentNumber := GetSequenceNumber(baseName); segmentNumber >= 0 {
			renditionSymbol := SelectRenditionSymbol(firstSegmentNumber, segmentNumber, watermarkInfo)
			responseURL += BuildVariantDirectoryPath(contentPath, variantDirDepth.mediaSegment, renditionSymbol)
		}
	case "m4s", "mp4":
		switch {
		case strings.HasSuffix(baseName, "init"):
			responseURL += BuildVariantDirectoryPath(contentPath, variantDirDepth.mediaSegment, "0")
		case isSubtitlePath(directoryPath):
			responseURL += BuildVariantDirectoryPath(contentPath, variantDirDepth.subtitle, "0")
		default:
			if segmentNumber := GetSequenceNumber(baseName); segmentNumber >= 0 {
				renditionSymbol := SelectRenditionSymbol(firstSegmentNumber, segmentNumber, watermarkInfo)
				responseURL += BuildVariantDirectoryPath(contentPath, variantDirDepth.mediaSegment, renditionSymbol)
			}
		}
	case "vtt":
		responseURL += BuildVariantDirectoryPath(contentPath, variantDirDepth.subtitle, "0")
	case "aac":
		responseURL += BuildVariantDirectoryPath(contentPath, variantDirDepth.mediaSegment, "0")
	}

	if responseURL == "/"+prefixFolder {
		responseURL += buildDefaultPath(contentPath)
	}
	return responseURL + queryString
}

func buildDefaultPath(contentPath string) string {
	var responseURL strings.Builder
	pathSegments := strings.Split(contentPath, "/")

	for i := 1; i < len(pathSegments); i++ {
		if slices.Contains(flagFolders, pathSegments[i]) {
			responseURL.WriteString("/0")
		}
		responseURL.WriteString("/" + pathSegments[i])
	}
	return responseURL.String()
}

func isSubtitlePath(directoryPath string) bool {
	pathSegments := strings.Split(directoryPath, "/")
	return len(pathSegments) >= 2 && pathSegments[len(pathSegments)-2] == "subtitle"
}
