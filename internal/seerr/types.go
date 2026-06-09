package seerr

// Overseerr Permission bit values (server/lib/permissions.ts).
const (
	PermRequest       = 32
	PermAutoApprove   = 128
	PermRequest4K     = 1024
	PermAutoApprove4K = 32768 // AUTO_APPROVE_4K (verified against Overseerr server/lib/permissions.ts)
)

// CreateRequestBody is the POST /api/v1/request body. serverId/profileId/
// rootFolder are intentionally omitted: Seerr default-routes those.
// UserID is set when attributing the request to a mapped Seerr user.
type CreateRequestBody struct {
	MediaType string `json:"mediaType"`        // "movie" | "tv"
	MediaID   int    `json:"mediaId"`           // TMDB id
	Is4K      bool   `json:"is4k"`
	Seasons   any    `json:"seasons,omitempty"` // "all" for tv; omitted for movie
	UserID    int    `json:"userId,omitempty"`
}

// MediaInfo is the nested media record on a MediaRequest.
type MediaInfo struct {
	Status int `json:"status"` // MediaStatus
	TMDBID int `json:"tmdbId"`
}

// MediaRequest is the Seerr request object returned by create/get/list.
type MediaRequest struct {
	ID     int       `json:"id"`
	Status int       `json:"status"` // MediaRequestStatus
	Is4K   bool      `json:"is4k"`
	Media  MediaInfo `json:"media"`
}

// requestPage is the GET /api/v1/request list envelope.
type requestPage struct {
	Results []MediaRequest `json:"results"`
}

// User is a Seerr account (GET/POST /api/v1/user).
type User struct {
	ID          int    `json:"id"`
	Email       string `json:"email"`
	Permissions int    `json:"permissions"`
}

type userPage struct {
	Results []User `json:"results"`
}

// MediaRequestStatus values (Overseerr server/constants/media.ts).
const (
	StatusRequestPending   = 1
	StatusRequestApproved  = 2
	StatusRequestDeclined  = 3
	StatusRequestFailed    = 4
	StatusRequestCompleted = 5
)

// MediaStatus values (Overseerr server/constants/media.ts).
const (
	MediaStatusUnknown            = 1
	MediaStatusPending            = 2
	MediaStatusProcessing         = 3
	MediaStatusPartiallyAvailable = 4
	MediaStatusAvailable          = 5
	MediaStatusDeleted            = 6
)

// MapStatus maps a Seerr (request.status, media.status) pair onto Silo's target
// status vocabulary. Order matters: terminal request failures win, then
// availability, then in-progress, else queued.
func MapStatus(requestStatus, mediaStatus int) string {
	switch {
	case requestStatus == StatusRequestDeclined || requestStatus == StatusRequestFailed:
		return "failed"
	case requestStatus == StatusRequestCompleted || mediaStatus == MediaStatusAvailable:
		return "completed"
	case mediaStatus == MediaStatusProcessing || mediaStatus == MediaStatusPartiallyAvailable:
		return "downloading"
	default:
		return "queued"
	}
}
