// Package imagebed 提供自建图床的上传/删除能力。
//
// 上传协议:
//	POST {endpoint}  (multipart/form-data)
//	字段: image (file) + token (text)
//	成功返回: { "result":"success", "code":200, "url":"...", "thumb":"...", "del":"...", "id":1 }
package imagebed

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"encoding/json"
)

// Config 图床客户端配置
type Config struct {
	Endpoint string        // 上传接口地址
	Token    string        // 鉴权 token
	Timeout  time.Duration // 请求超时
}

// Client 图床客户端
type Client struct {
	endpoint string
	token    string
	http     *http.Client
}

// New 创建图床客户端
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		endpoint: cfg.Endpoint,
		token:    cfg.Token,
		http:     &http.Client{Timeout: timeout},
	}
}

// UploadResult 图床上传结果
type UploadResult struct {
	URL    string // 图片访问地址
	Thumb  string // 缩略图地址
	DelURL string // 删除接口地址 (供定时任务清理使用)
	ID     int    // 图床图片 ID
}

// Upload 上传单张图片
//
// filename 为文件名, content 为图片内容 (建议为 *multipart.FileHeader 打开的流)。
func (c *Client) Upload(ctx context.Context, filename string, content io.Reader) (*UploadResult, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		return nil, fmt.Errorf("imagebed: create form file: %w", err)
	}
	if _, err := io.Copy(part, content); err != nil {
		return nil, fmt.Errorf("imagebed: copy image content: %w", err)
	}
	if err := writer.WriteField("token", c.token); err != nil {
		return nil, fmt.Errorf("imagebed: write token field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("imagebed: close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("imagebed: build request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("imagebed: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("imagebed: unexpected status %d", resp.StatusCode)
	}

	var result struct {
		Result   string `json:"result"`
		Code     int    `json:"code"`
		Message  string `json:"message"`
		URL      string `json:"url"`
		Thumb    string `json:"thumb"`
		Del      string `json:"del"`
		ID       int    `json:"id"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("imagebed: decode response: %w", err)
	}

	if result.Code != 200 || result.Result != "success" || result.URL == "" {
		return nil, fmt.Errorf("imagebed: upload failed (code=%d, message=%q)", result.Code, result.Message)
	}

	return &UploadResult{
		URL:    result.URL,
		Thumb:  result.Thumb,
		DelURL: result.Del,
		ID:     result.ID,
	}, nil
}

// Delete 通过图床删除接口删除图片 (delURL 由 Upload 返回, 供定时任务清理失效图片)
func (c *Client) Delete(ctx context.Context, delURL string) error {
	if delURL == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, delURL, nil)
	if err != nil {
		return fmt.Errorf("imagebed: build delete request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("imagebed: delete request failed: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
