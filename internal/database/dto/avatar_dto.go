package dto

type AvatarUploadURLDto struct {
	UploadURL string            `json:"upload_url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	AvatarURL string            `json:"avatar_url"`
	ExpiresIn int               `json:"expires_in"`
}

type AvatarDownloadURLDto struct {
	DownloadURL string `json:"download_url"`
	ExpiresIn   int    `json:"expires_in"`
}
