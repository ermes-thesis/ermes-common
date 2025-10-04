package model

type WasmResponse struct {
	HttpCode            int                          `json:"http_code"`
	Response            any                          `json:"response"`
	CollectionsToCommit map[string]map[string]string `json:"collections_to_commit,omitempty"`
	DeletedRecords      map[string][]string          `json:"deleted_records,omitempty"`
	DeletedCollections  []string                     `json:"deleted_collections,omitempty"`
}
