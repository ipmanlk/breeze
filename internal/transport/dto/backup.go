package dto

// BackupRestorePendingResponse reports whether a staged restore is pending.
type BackupRestorePendingResponse struct {
	Pending bool   `json:"pending"`
	Path    string `json:"path,omitempty"`
	Size    int64  `json:"size,omitempty"`
}

// BackupRestoreResponse is the response after staging a restore.
type BackupRestoreResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
