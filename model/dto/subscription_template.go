package dto

type SubscriptionTemplateDto struct {
	NodeNameTemplate      *string `json:"nodeNameTemplate" validate:"required,min=1,max=256"`
	ServerAddressTemplate *string `json:"serverAddressTemplate" validate:"required,min=1,max=256"`
}
