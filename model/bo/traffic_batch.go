package bo

type Hysteria2TrafficReceipt struct {
	Epoch    string `json:"epoch"`
	Sequence int64  `json:"sequence"`
}

type Hysteria2TrafficBatch struct {
	Hysteria2TrafficReceipt
	Users map[string]Hysteria2UserTraffic `json:"users"`
}
