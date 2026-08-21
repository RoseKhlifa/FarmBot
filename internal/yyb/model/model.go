// Package model contains the small set of yyb persistence models shared by
// the database facade and protocol pool. Keeping them below the parent yyb
// package prevents the protocol package from creating an import cycle.
package model

// WechatAccount is the durable WeChat identity used by yyb operations.
type WechatAccount struct {
	ID            int64          `json:"id"`
	OpenID        string         `json:"openid"`
	UIN           *int64         `json:"uin,omitempty"`
	Alias         *string        `json:"alias,omitempty"`
	Nickname      *string        `json:"nickname,omitempty"`
	Avatar        *string        `json:"avatar,omitempty"`
	UserInfo      map[string]any `json:"user_info,omitempty"`
	LoginBuffer   string         `json:"login_buffer,omitempty"`
	Credentials   map[string]any `json:"credentials,omitempty"`
	Status        *string        `json:"status,omitempty"`
	LastCheckedAt *int64         `json:"last_checked_at,omitempty"`
	CreatedAt     int64          `json:"created_at"`
	UpdatedAt     int64          `json:"updated_at"`
}

// AccountPublic is the credential-free representation suitable for APIs.
type AccountPublic struct {
	ID            int64   `json:"id"`
	OpenID        string  `json:"openid"`
	UIN           *int64  `json:"uin"`
	Alias         *string `json:"alias"`
	Nickname      *string `json:"nickname"`
	Avatar        *string `json:"avatar"`
	Status        *string `json:"status"`
	LastCheckedAt *int64  `json:"last_checked_at"`
	CreatedAt     int64   `json:"created_at"`
	UpdatedAt     int64   `json:"updated_at"`
}

// Public strips credentials and opaque profile fields before an API response.
func (a *WechatAccount) Public() AccountPublic {
	return AccountPublic{
		ID: a.ID, OpenID: a.OpenID, UIN: a.UIN, Alias: a.Alias,
		Nickname: a.Nickname, Avatar: a.Avatar, Status: a.Status,
		LastCheckedAt: a.LastCheckedAt, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

// SessionRow stores a serialized MMTLS session for one account and proxy.
type SessionRow struct {
	ID              int64
	WechatAccountID int64
	UIN             *int64
	TCPProxy        string
	SessionBlob     map[string]any
	ExpiresAt       int64
	CreatedAt       int64
	UpdatedAt       int64
}
