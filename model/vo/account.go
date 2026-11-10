package vo

type AccountOutboundOption struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Source string `json:"source"`
	Reason string `json:"reason,omitempty"`
}

type AccountOutboundOptions struct {
	Options     []AccountOutboundOption `json:"options"`
	Unavailable []AccountOutboundOption `json:"unavailable"`
}

type AccountVo struct {
	BaseVo
	Username     string `json:"username"`
	UploadMbps   int64  `json:"uploadMbps"`
	DownloadMbps int64  `json:"downloadMbps"`
	Quota        int64  `json:"quota"`
	Download     int64  `json:"download"`
	Upload       int64  `json:"upload"`
	ExpireTime   int64  `json:"expireTime"`
	KickUtilTime int64  `json:"kickUtilTime"` // Offline remaining time
	DeviceNo     int64  `json:"deviceNo"`     // Limit the number of devices
	Role         string `json:"role"`
	Deleted      int64  `json:"deleted"`

	Online bool  `json:"online"` // online status
	Device int64 `json:"device"` // Number of online devices

	LoginAt  int64  `json:"loginAt"`
	ConAt    int64  `json:"conAt"`
	Remark   string `json:"remark"`
	Outbound string `json:"outbound"`
}
type AccountPageVo struct {
	AccountVos []AccountVo `json:"records"`
	Total      int64       `json:"total"`
}

type AccountInfoVo struct {
	Id       int64    `json:"id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}
