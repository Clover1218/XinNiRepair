package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"xin-ni-repair/internal/config"
	apperrors "xin-ni-repair/internal/errors"
)

// code2Session 接口地址
const code2SessionURL = "https://api.weixin.qq.com/sns/jscode2session"

// accessTokenURL 获取全局 access_token 接口地址
const accessTokenURL = "https://api.weixin.qq.com/cgi-bin/token"

// getPhoneNumberURL 手机号解密接口地址
const getPhoneNumberURL = "https://api.weixin.qq.com/wxa/business/getuserphonenumber"

// WechatSession code2session 返回的会话信息
type WechatSession struct {
	Openid     string
	Unionid    string
	SessionKey string
}

// WechatService 封装微信接口调用
type WechatService struct {
	appID      string
	appSecret  string
	httpClient *http.Client

	mu            sync.Mutex // 保护 access_token 缓存
	accessToken   string
	accessTokenAt time.Time
}

// NewWechatService 创建 WechatService
func NewWechatService(cfg config.WechatConfig) *WechatService {
	return &WechatService{
		appID:      cfg.AppID,
		appSecret:  cfg.AppSecret,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// GetAccessToken 获取全局 access_token, 内存缓存至过期前 5 分钟
func (s *WechatService) GetAccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accessToken != "" && time.Since(s.accessTokenAt) < 7000*time.Second {
		return s.accessToken, nil
	}

	params := url.Values{}
	params.Set("grant_type", "client_credential")
	params.Set("appid", s.appID)
	params.Set("secret", s.appSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, accessTokenURL+"?"+params.Encode(), nil)
	if err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Errcode     int    `json:"errcode"`
		Errmsg      string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}
	if result.AccessToken == "" {
		return "", apperrors.ErrWechatAPI.WithMessage(fmt.Sprintf("获取 access_token 失败(%d): %s", result.Errcode, result.Errmsg))
	}

	s.accessToken = result.AccessToken
	s.accessTokenAt = time.Now()
	return s.accessToken, nil
}

// GetPhoneNumber 用 getPhoneNumber 返回的 code 解密手机号 (微信新版规范)
func (s *WechatService) GetPhoneNumber(ctx context.Context, code string) (string, error) {
	token, err := s.GetAccessToken(ctx)
	if err != nil {
		return "", err
	}

	payload, _ := json.Marshal(map[string]string{"code": code})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, getPhoneNumberURL+"?access_token="+url.QueryEscape(token), bytes.NewReader(payload))
	if err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}

	var result struct {
		Errcode   int    `json:"errcode"`
		Errmsg    string `json:"errmsg"`
		PhoneInfo struct {
			PhoneNumber string `json:"phoneNumber"`
		} `json:"phone_info"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}
	if result.Errcode != 0 || result.PhoneInfo.PhoneNumber == "" {
		return "", apperrors.ErrWechatAPI.WithMessage(fmt.Sprintf("手机号解密失败(%d): %s", result.Errcode, result.Errmsg))
	}
	return result.PhoneInfo.PhoneNumber, nil
}

// Code2Session 使用登录 code 换取 openid/unionid
func (s *WechatService) Code2Session(ctx context.Context, code string) (*WechatSession, error) {
	params := url.Values{}
	params.Set("appid", s.appID)
	params.Set("secret", s.appSecret)
	params.Set("js_code", code)
	params.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, code2SessionURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, apperrors.ErrWechatAPI.WithError(err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, apperrors.ErrWechatAPI.WithError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, apperrors.ErrWechatAPI.WithError(err)
	}

	var result struct {
		Openid     string `json:"openid"`
		Unionid    string `json:"unionid"`
		SessionKey string `json:"session_key"`
		Errcode    int    `json:"errcode"`
		Errmsg     string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, apperrors.ErrWechatAPI.WithError(err)
	}

	if result.Errcode != 0 {
		return nil, apperrors.ErrWechatAuthFailed.WithMessage(fmt.Sprintf("微信授权失败(%d): %s", result.Errcode, result.Errmsg))
	}
	if result.Openid == "" {
		return nil, apperrors.ErrWechatAuthFailed
	}

	return &WechatSession{
		Openid:     result.Openid,
		Unionid:    result.Unionid,
		SessionKey: result.SessionKey,
	}, nil
}
