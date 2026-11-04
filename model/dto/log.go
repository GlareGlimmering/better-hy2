package dto

type LogDto struct {
	NumLine *int `json:"numLine" form:"numLine" validate:"omitempty,min=1,max=300"`
}

type LogExportDto struct {
	Option *int `json:"option" form:"option" validate:"required,oneof=0 1 2"`
}

type LogStreamDto struct {
	Type   *string `json:"type" form:"type" validate:"required,oneof=system hysteria2 access"`
	Cursor *string `json:"cursor" form:"cursor" validate:"omitempty,max=512"`
	Limit  *int    `json:"limit" form:"limit" validate:"omitempty,min=1,max=200"`
}

type LogClearDto struct {
	Type *string `json:"type" form:"type" validate:"required,oneof=system hysteria2 access all"`
}
