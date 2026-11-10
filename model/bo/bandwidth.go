package bo

type AccountBandwidth struct {
	Version      uint64 `json:"version"`
	UploadMbps   int64  `json:"uploadMbps"`
	DownloadMbps int64  `json:"downloadMbps"`
}
