package web

type Pagination struct {
	Page      int               `json:"page"`
	PageSize  int               `json:"page_size"`
	SortBy    string            `json:"sort_by"`
	SortOrder string            `json:"sort_order"`
	Filters   map[string]string `json:"filters"` // For advanced filtering
}
