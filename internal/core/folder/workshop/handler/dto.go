package handler

import (
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
)

type UpdateConfigRequest struct {
	ExpectedVersion int64                     `json:"expected_version"`
	Config          folderconfig.FolderConfig `json:"config"`
}

type UpdateConfigResponse struct {
	Config        folderconfig.FolderConfig `json:"config"`
	ConfigVersion int64                     `json:"config_version"`
}
