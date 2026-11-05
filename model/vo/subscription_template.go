package vo

type SubscriptionTemplateVo struct {
	NodeNameTemplate      string   `json:"nodeNameTemplate"`
	ServerAddressTemplate string   `json:"serverAddressTemplate"`
	ServerIPv4            string   `json:"serverIPv4,omitempty"`
	ServerIPv4Error       string   `json:"serverIPv4Error,omitempty"`
	ServerIPv6            string   `json:"serverIPv6,omitempty"`
	ServerIPv6Error       string   `json:"serverIPv6Error,omitempty"`
	PreviewName           string   `json:"previewName,omitempty"`
	PreviewAddress        string   `json:"previewAddress,omitempty"`
	PreviewError          string   `json:"previewError,omitempty"`
	Variables             []string `json:"variables"`
}
