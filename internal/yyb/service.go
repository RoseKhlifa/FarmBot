package yyb

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/yyb/protocol"
	"github.com/RoseKhlifa/FarmBot/internal/yyb/qr"
)

const defaultAppID = "wx5306c5978fdb76e4"

// ServiceConfig contains the in-process yyb dependencies. Pool and QRClient
// can be injected by tests or by the application root; omitted values use the
// production protocol clients.
type ServiceConfig struct {
	Pool           Pool
	QRClient       QRClient
	RequestTimeout time.Duration
	QRSessionTTL   time.Duration
	TCPProxy       string
	DefaultAppID   string
}

// Pool is the protocol surface used by the service. Keeping this interface
// small makes account and QR workflows unit-testable without network calls.
type Pool interface {
	GetCode(context.Context, string, string, int64, string) (map[string]any, error)
	GetPhoneNumber(context.Context, string, string, int64, string) (map[string]any, error)
	OperateWXData(context.Context, string, string, map[string]any, int64, string) (map[string]any, error)
}

// QRClient is the QR/login-buffer surface used by the service.
type QRClient interface {
	GetQRCodeImage(context.Context) (qr.ImageResult, error)
	PollQRCode(context.Context, *qr.Session) (qr.PollResult, error)
	GetLoginBuffer(context.Context, *qr.Session) (protocol.LoginBufferResult, error)
	RefreshLoginBuffer(context.Context, protocol.LoginBufferCredentials) (protocol.LoginBufferResult, error)
	LoginBuffers() *protocol.LoginBufferClient
}

// QRCreateResult is the transport-neutral result of starting a scan. The
// image is returned as bytes for HTTP handlers and as a data URI for clients
// that do not want to expose a file-system path.
type QRCreateResult struct {
	SessionID   string `json:"session_id"`
	Status      string `json:"status"`
	ImageBytes  []byte `json:"-"`
	ImageBase64 string `json:"image_base64,omitempty"`
}

// QRPollResult is the stable service contract around the protocol package's
// QR response. Code is populated only after WeChat authorizes the session.
type QRPollResult struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	ErrCode   *int   `json:"errcode,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
}

// QRConfirmResult contains the persisted account and the credentials created
// by the scan. Callers normally only need Account; the other fields are
// useful to an account manager that wants to start a session immediately.
type QRConfirmResult struct {
	Account     *WechatAccount
	LoginBuffer string
	Credentials protocol.LoginBufferCredentials
}

// Service is the in-process yyb contract consumed by account managers and
// HTTP handlers. It deliberately exposes no HTTP or bearer-token concerns.
type Service interface {
	GetCode(context.Context, string, string) (string, error)
	GetPhoneNumber(context.Context, string, string) (map[string]any, error)
	OperateWxData(context.Context, string, string, map[string]any) (map[string]any, error)
	RefreshLoginBuffer(context.Context, string) (*WechatAccount, error)
	QRCreate(context.Context) (QRCreateResult, error)
	QRPoll(context.Context, string) (QRPollResult, error)
	QRConfirm(context.Context, string) (QRConfirmResult, error)
	ListAccounts(context.Context) ([]*WechatAccount, error)
	DeleteAccount(context.Context, string) (*WechatAccount, error)
}

// service owns the in-process yyb operations. QR sessions are deliberately
// memory-only and expire; durable identity and MMTLS sessions live in DB.
type service struct {
	db           *DB
	pool         Pool
	qrClient     QRClient
	qrSessionTTL time.Duration
	tcpProxy     string
	defaultAppID string

	mu         sync.Mutex
	qrSessions map[string]*qr.Session
}

// NewService builds an in-process yyb facade over the shared FarmBot DB.
func NewService(db *DB, cfg ServiceConfig) (Service, error) {
	if db == nil || db.DB == nil {
		return nil, errors.New("yyb service requires a shared database")
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 8 * time.Second
	}
	if cfg.QRSessionTTL <= 0 {
		cfg.QRSessionTTL = 5 * time.Minute
	}
	if strings.TrimSpace(cfg.DefaultAppID) == "" {
		cfg.DefaultAppID = defaultAppID
	}
	if cfg.QRClient == nil {
		cfg.QRClient = qr.NewClient(cfg.RequestTimeout)
	}
	if cfg.Pool == nil {
		poolCfg := protocol.DefaultConfig()
		poolCfg.TCPProxy = cfg.TCPProxy
		poolCfg.ShortlinkTimeout = cfg.RequestTimeout
		cfg.Pool = protocol.NewPool(poolCfg, db)
	}
	return &service{
		db: db, pool: cfg.Pool, qrClient: cfg.QRClient,
		qrSessionTTL: cfg.QRSessionTTL, tcpProxy: cfg.TCPProxy,
		defaultAppID: cfg.DefaultAppID, qrSessions: make(map[string]*qr.Session),
	}, nil
}

// GetCode obtains a fresh mini-program login code for an account reference.
func (s *service) GetCode(ctx context.Context, openID, appID string) (string, error) {
	result, err := s.callWXApp(ctx, openID, appID, nil, func(ctx context.Context, account *WechatAccount, id string, _ map[string]any) (map[string]any, error) {
		return s.pool.GetCode(ctx, account.LoginBuffer, id, account.ID, s.tcpProxy)
	})
	if err != nil {
		return "", err
	}
	code, ok := result["code"].(string)
	if !ok || strings.TrimSpace(code) == "" {
		return "", fmt.Errorf("getCode response did not contain a code")
	}
	return code, nil
}

// GetPhoneNumber forwards the yyb phone-number operation for an account.
func (s *service) GetPhoneNumber(ctx context.Context, openID, appID string) (map[string]any, error) {
	return s.callWXApp(ctx, openID, appID, nil, func(ctx context.Context, account *WechatAccount, id string, _ map[string]any) (map[string]any, error) {
		return s.pool.GetPhoneNumber(ctx, account.LoginBuffer, id, account.ID, s.tcpProxy)
	})
}

// OperateWxData forwards an arbitrary wxapp payload for an account.
func (s *service) OperateWxData(ctx context.Context, openID, appID string, payload map[string]any) (map[string]any, error) {
	return s.callWXApp(ctx, openID, appID, payload, func(ctx context.Context, account *WechatAccount, id string, body map[string]any) (map[string]any, error) {
		return s.pool.OperateWXData(ctx, account.LoginBuffer, id, body, account.ID, s.tcpProxy)
	})
}

// RefreshLoginBuffer refreshes and persists an account's yyb credentials.
func (s *service) RefreshLoginBuffer(ctx context.Context, ref string) (*WechatAccount, error) {
	account, err := s.db.ResolveAccount(ctx, strings.TrimSpace(ref))
	if err != nil {
		return nil, err
	}
	if account.Credentials == nil {
		return nil, fmt.Errorf("account %q has no refresh credentials", account.OpenID)
	}
	credentials := protocol.CredentialsFromMap(account.Credentials)
	result, err := s.qrClient.RefreshLoginBuffer(ctx, credentials)
	if err != nil {
		_ = s.db.SetAccountStatus(ctx, account.ID, "expired")
		return nil, fmt.Errorf("refresh login buffer: %w", err)
	}
	if err := s.db.SetAccountCredential(ctx, account.ID, result.LoginBuffer, result.Credentials.ToMap()); err != nil {
		return nil, err
	}
	if err := s.db.SetAccountStatus(ctx, account.ID, "alive"); err != nil {
		return nil, err
	}
	return s.db.GetAccount(ctx, account.ID)
}

// QRCreate starts a QR scan and keeps its session in memory until confirmation
// or expiration. No QR image is written by this package.
func (s *service) QRCreate(ctx context.Context) (QRCreateResult, error) {
	s.pruneQRSessions()
	image, err := s.qrClient.GetQRCodeImage(ctx)
	if err != nil {
		return QRCreateResult{}, err
	}
	s.mu.Lock()
	s.qrSessions[image.Session.ID] = image.Session
	s.mu.Unlock()
	return QRCreateResult{
		SessionID:   image.Session.ID,
		Status:      image.Session.Status,
		ImageBytes:  append([]byte(nil), image.ImageBytes...),
		ImageBase64: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(image.ImageBytes),
	}, nil
}

// QRPoll checks a previously-created scan session.
func (s *service) QRPoll(ctx context.Context, sessionID string) (QRPollResult, error) {
	sess, err := s.qrSession(sessionID)
	if err != nil {
		return QRPollResult{}, err
	}
	result, err := s.qrClient.PollQRCode(ctx, sess)
	if err != nil {
		return QRPollResult{}, err
	}
	if result.Status == "expired" || result.Status == "cancelled" || result.Status == "unknown" {
		s.dropQRSession(sessionID)
	}
	return QRPollResult{SessionID: sessionID, Status: result.Status, ErrCode: result.ErrCode, Code: result.Code, Message: result.Message}, nil
}

// QRConfirm exchanges an authorized QR session for a login buffer and stores
// the resulting identity in the shared database.
func (s *service) QRConfirm(ctx context.Context, sessionID string) (QRConfirmResult, error) {
	sess, err := s.qrSession(sessionID)
	if err != nil {
		return QRConfirmResult{}, err
	}
	result, err := s.qrClient.GetLoginBuffer(ctx, sess)
	if err != nil {
		return QRConfirmResult{}, fmt.Errorf("confirm QR session: %w", err)
	}
	var userInfo map[string]any
	if loginBuffers := s.qrClient.LoginBuffers(); loginBuffers != nil {
		userInfo, _ = loginBuffers.FetchUserInfo(ctx, result.Credentials)
	}
	nickname := result.Credentials.Nickname
	if value, ok := userInfo["nickname"].(string); ok && strings.TrimSpace(value) != "" {
		nickname = value
	}
	status := "alive"
	account, err := s.db.UpsertAccount(ctx, result.Credentials.OpenID, result.LoginBuffer, nil, stringPtrOrNil(nickname), nil, userInfo, result.Credentials.ToMap(), &status)
	if err != nil {
		return QRConfirmResult{}, err
	}
	s.dropQRSession(sessionID)
	return QRConfirmResult{Account: account, LoginBuffer: result.LoginBuffer, Credentials: result.Credentials}, nil
}

// ListAccounts returns all persisted yyb identities.
func (s *service) ListAccounts(ctx context.Context) ([]*WechatAccount, error) {
	return s.db.ListAccounts(ctx)
}

// DeleteAccount resolves an account by id, UIN, or openid and removes it and
// its MMTLS sessions. The returned account is useful to report what changed.
func (s *service) DeleteAccount(ctx context.Context, ref string) (*WechatAccount, error) {
	account, err := s.db.ResolveAccount(ctx, strings.TrimSpace(ref))
	if err != nil {
		return nil, err
	}
	if err := s.db.DeleteAccount(ctx, account.ID); err != nil {
		return nil, err
	}
	return account, nil
}

type wxAppCall func(context.Context, *WechatAccount, string, map[string]any) (map[string]any, error)

func (s *service) callWXApp(ctx context.Context, ref, appID string, payload map[string]any, call wxAppCall) (map[string]any, error) {
	if s == nil || s.db == nil || s.pool == nil {
		return nil, errors.New("yyb service is not initialized")
	}
	account, err := s.db.ResolveAccount(ctx, strings.TrimSpace(ref))
	if err != nil {
		return nil, err
	}
	appID = strings.TrimSpace(appID)
	if appID == "" {
		appID = s.defaultAppID
	}
	result, err := call(ctx, account, appID, payload)
	if err == nil {
		return result, nil
	}
	if _, refreshErr := s.RefreshLoginBuffer(ctx, account.OpenID); refreshErr != nil {
		return nil, fmt.Errorf("yyb operation failed: %w (refresh failed: %v)", err, refreshErr)
	}
	fresh, getErr := s.db.GetAccount(ctx, account.ID)
	if getErr != nil {
		return nil, getErr
	}
	return call(ctx, fresh, appID, payload)
}

func (s *service) qrSession(id string) (*qr.Session, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("QR session id is required")
	}
	s.mu.Lock()
	sess := s.qrSessions[id]
	s.mu.Unlock()
	if sess == nil {
		return nil, errors.New("QR session not found or expired")
	}
	if sess.Age() > s.qrSessionTTL {
		s.dropQRSession(id)
		return nil, errors.New("QR session not found or expired")
	}
	return sess, nil
}

func (s *service) dropQRSession(id string) {
	s.mu.Lock()
	delete(s.qrSessions, id)
	s.mu.Unlock()
}

func (s *service) pruneQRSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.qrSessions {
		if sess.Age() > s.qrSessionTTL {
			delete(s.qrSessions, id)
		}
	}
}

func stringPtrOrNil(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}
