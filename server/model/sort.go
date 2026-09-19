package model

type RSort uint8

const (
	RSortTimeAsc RSort = iota
	RSortTimeDesc
	RSortViewsAsc
	RSortViewsDesc
	RSortDownloadsAsc
	RSortDownloadsDesc
	RSortReleaseDateAsc
	RSortReleaseDateDesc
	RSortRelevance
)

func IsValidListSort(v int) bool {
	return v >= 0 && v <= int(RSortReleaseDateDesc)
}

func IsValidSearchSort(v int) bool {
	return v >= 0 && v <= int(RSortRelevance)
}
