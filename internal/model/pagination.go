package model

type PaginationParams struct {
	Page     int `json:"page" form:"page"`
	PageSize int `json:"page_size" form:"page_size"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	Total      int64       `json:"total"`
	TotalPages int         `json:"total_pages"`
}

func (p *PaginationParams) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
}

func (p *PaginationParams) Offset() int {
	return (p.Page - 1) * p.PageSize
}

func NewPaginatedResponse(data interface{}, page, pageSize int, total int64) *PaginatedResponse {
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}
	return &PaginatedResponse{
		Data:       data,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}
}
