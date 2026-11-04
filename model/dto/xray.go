package dto

type XraySettingsDto struct {
	Version   *string                   `json:"version" validate:"required"`
	Outbounds *[]XrayManagedOutboundDto `json:"outbounds" validate:"required"`
}

type XrayManagedOutboundDto struct {
	ID          *string `json:"id" validate:"omitempty,max=64"`
	Name        *string `json:"name" validate:"required,max=64"`
	Protocol    *string `json:"protocol" validate:"required,oneof=vless http socks5"`
	Enabled     *bool   `json:"enabled" validate:"required"`
	Address     *string `json:"address" validate:"required"`
	Port        *uint16 `json:"port" validate:"required,gt=0"`
	UUID        *string `json:"uuid" validate:"omitempty"`
	Username    *string `json:"username" validate:"omitempty,max=255"`
	Password    *string `json:"password" validate:"omitempty,max=1024"`
	Network     *string `json:"network" validate:"omitempty"`
	TLSSNI      *string `json:"tlsSni" validate:"omitempty"`
	UseCustomCA *bool   `json:"useCustomCa" validate:"omitempty"`
	SOCKSListen *string `json:"socksListen" validate:"omitempty"`
	SOCKSPort   *uint16 `json:"socksPort" validate:"omitempty"`
	OutboundTag *string `json:"outboundTag" validate:"omitempty,max=128"`
}

type XrayOutboundTestDto struct {
	ID       *string `json:"id" validate:"required"`
	Protocol *string `json:"protocol" validate:"required,oneof=tcp udp"`
}

type XrayVersionDto struct {
	Version *string `json:"version" validate:"required"`
}
