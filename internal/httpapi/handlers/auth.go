package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterAuth(r gin.IRouter) {
	r.POST("/api/login", h.login)
	r.POST("/api/logout", h.logout)
	r.POST("/api/register", h.register)
	r.GET("/api/card/info/:code", h.cardInfo)
	r.POST("/api/user/renew", h.renew)
	r.POST("/api/public/renew", h.publicRenew)
	r.POST("/api/public/reset-password/verify", h.resetPasswordVerify)
	r.POST("/api/public/reset-password/confirm", h.resetPasswordConfirm)
	r.POST("/api/user/change-password", h.changePassword)
}

func (h *Handler) logout(c *gin.Context) {
	if h.app().Sessions != nil {
		if err := h.app().Sessions.Invalidate(c.Request.Context(), middleware.CurrentToken(c)); err != nil {
			writeError(c, err)
			return
		}
	}
	writeOK(c, nil)
}

func (h *Handler) login(c *gin.Context) {
	var body struct{ Username, Password string }
	if !bindJSON(c, &body) {
		return
	}
	if strings.TrimSpace(body.Username) == "" || body.Password == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "请输入用户名和密码"})
		return
	}
	var user *store.User
	var err error
	if h.app().Auth != nil {
		value, e := h.app().Auth.Login(c.Request.Context(), body.Username, body.Password, c.ClientIP())
		user = &value
		err = e
	} else if h.app().Users != nil {
		user, err = h.app().Users.Authenticate(c.Request.Context(), body.Username, body.Password, c.ClientIP())
	} else {
		writeNotConfigured(c)
		return
	}
	if err != nil {
		status := http.StatusUnauthorized
		errorType := "invalid_credentials"
		message := "用户名或密码错误"
		switch {
		case errors.Is(err, store.ErrRateLimited):
			status, errorType, message = http.StatusTooManyRequests, "rate_limit", "登录请求过于频繁，请稍后再试"
		case errors.Is(err, store.ErrUserLocked):
			status, errorType, message = http.StatusLocked, "locked", "账号已锁定，请稍后再试"
		case errors.Is(err, store.ErrUserExpired):
			status, errorType, message = http.StatusForbidden, "expired", "账号已过期，请续费后重新登录"
		case errors.Is(err, store.ErrUserDisabled):
			status, errorType, message = http.StatusForbidden, "banned", "账号已被封禁，请联系管理员"
		case !errors.Is(err, store.ErrInvalidCredentials):
			message = err.Error()
		}
		c.JSON(status, gin.H{"ok": false, "error": message, "errorType": errorType})
		return
	}
	response := gin.H{
		"role":               user.Role,
		"card":               publicCard(user.CardJSON),
		"accountLimit":       publicAccountLimit(user.AccountLimit),
		"user":               gin.H{"username": user.Username},
		"mustChangePassword": user.MustChangePassword,
	}
	if h.app().Sessions != nil {
		token, e := h.app().Sessions.Create(c.Request.Context(), *user)
		if e != nil {
			writeError(c, e)
			return
		}
		response["token"] = token
		response["adminToken"] = token
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": response})
}
func (h *Handler) register(c *gin.Context) {
	var body struct{ Username, Password, CardCode string }
	if !bindJSON(c, &body) {
		return
	}
	if h.app().Cards == nil {
		writeNotConfigured(c)
		return
	}
	user, err := h.app().Cards.RegisterWithCard(c.Request.Context(), body.Username, body.Password, body.CardCode)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, publicUser(*user))
}
func (h *Handler) cardInfo(c *gin.Context) {
	if h.app().Cards == nil {
		writeNotConfigured(c)
		return
	}
	card, err := h.app().Cards.Get(c.Request.Context(), strings.TrimSpace(c.Param("code")))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, card)
}
func (h *Handler) renew(c *gin.Context) {
	if h.app().Cards == nil {
		writeNotConfigured(c)
		return
	}
	user, _ := middleware.CurrentUser(c)
	var body struct {
		CardCode string `json:"cardCode"`
		Code     string `json:"code"`
	}
	if !bindJSON(c, &body) {
		return
	}
	code := body.CardCode
	if strings.TrimSpace(code) == "" {
		code = body.Code
	}
	if strings.TrimSpace(user.Username) == "" || strings.TrimSpace(code) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "请提供卡密"})
		return
	}
	updated, err := h.app().Cards.Renew(c.Request.Context(), user.Username, code)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, publicRenewal(updated))
}
func (h *Handler) publicRenew(c *gin.Context) {
	if h.app().Cards == nil {
		writeNotConfigured(c)
		return
	}
	var body struct {
		Username string `json:"username"`
		Code     string `json:"cardCode"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if strings.TrimSpace(body.Username) == "" || strings.TrimSpace(body.Code) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "请提供用户名和卡密"})
		return
	}
	updated, err := h.app().Cards.Renew(c.Request.Context(), strings.TrimSpace(body.Username), body.Code)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, publicRenewal(updated))
}

func (h *Handler) resetPasswordVerify(c *gin.Context) {
	var body struct {
		Username string `json:"username"`
		CardCode string `json:"cardCode"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := h.validateResetCard(c, body.Username, body.CardCode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "valid": false, "error": err.Error()})
		return
	}
	writeOK(c, gin.H{"valid": true, "username": strings.TrimSpace(body.Username)})
}

func (h *Handler) resetPasswordConfirm(c *gin.Context) {
	var body struct {
		Username    string `json:"username"`
		CardCode    string `json:"cardCode"`
		NewPassword string `json:"newPassword"`
		Password    string `json:"password"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if body.NewPassword == "" {
		body.NewPassword = body.Password
	}
	if err := h.validateResetCard(c, body.Username, body.CardCode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
		return
	}
	if h.app().Users == nil {
		writeNotConfigured(c)
		return
	}
	user, err := h.app().Users.Get(c.Request.Context(), strings.TrimSpace(body.Username))
	if err != nil {
		writeError(c, err)
		return
	}
	if err := setUserPassword(user, body.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
		return
	}
	if err := h.app().Users.Update(c.Request.Context(), *user); err != nil {
		writeError(c, err)
		return
	}
	if h.app().Sessions != nil {
		_ = h.app().Sessions.InvalidateAll(c.Request.Context(), func(session middleware.Session) bool {
			return strings.EqualFold(session.User.Username, user.Username)
		})
	}
	writeOK(c, gin.H{"username": user.Username})
}

func (h *Handler) validateResetCard(c *gin.Context, username, code string) error {
	if h.app().Cards == nil || h.app().Users == nil {
		return ErrApplicationDependency
	}
	if strings.TrimSpace(username) == "" || strings.TrimSpace(code) == "" {
		return errors.New("username and cardCode are required")
	}
	if _, err := h.app().Users.Get(c.Request.Context(), username); err != nil {
		return err
	}
	card, err := h.app().Cards.Get(c.Request.Context(), code)
	if err != nil {
		return err
	}
	if !card.Enabled || card.Status != "used" || !strings.EqualFold(strings.TrimSpace(card.BoundUser), strings.TrimSpace(username)) {
		return errors.New("card is unavailable")
	}
	return nil
}

func setUserPassword(user *store.User, password string) error {
	if user == nil {
		return errors.New("user is required")
	}
	if err := store.ValidatePassword(password); err != nil {
		return err
	}
	hashed, err := store.HashPassword(password)
	if err != nil {
		return err
	}
	pwdHash, salt, ok := strings.Cut(hashed, ":")
	if !ok {
		return errors.New("invalid password hash")
	}
	user.Password, user.PwdHash, user.Salt = hashed, pwdHash, salt
	user.MustChangePassword = false
	return nil
}

func (h *Handler) changePassword(c *gin.Context) {
	if h.app().Users == nil {
		writeNotConfigured(c)
		return
	}
	current, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "Unauthorized"})
		return
	}
	var body struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if !store.VerifyPasswordParts(body.OldPassword, current.PwdHash, current.Salt) && !store.VerifyPassword(body.OldPassword, current.Password) {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "旧密码不正确"})
		return
	}
	if err := setUserPassword(&current, body.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
		return
	}
	if err := h.app().Users.Update(c.Request.Context(), current); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, nil)
}
