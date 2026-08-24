package handler

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// UploadedFile 统一文件表示, 来自 multipart/form-data 或 JSON base64
type UploadedFile struct {
	Filename  string
	Size      int64
	Reader    io.ReadCloser
	SortOrder int
}

// Close 释放资源
func (f *UploadedFile) Close() error {
	if f.Reader != nil {
		return f.Reader.Close()
	}
	return nil
}

// Ext 返回小写扩展名 (含点号), 如 ".jpg"
func (f *UploadedFile) Ext() string {
	return strings.ToLower(filepath.Ext(f.Filename))
}

// ParseUploadFile 智能解析上传文件:
//   - Content-Type: multipart/form-data → 读取 "file" 字段 (二进制文件流)
//   - Content-Type: application/json     → 读取 JSON { "file": "<base64>", "filename": "xxx.jpg", "sort_order": 0 }
//
// 支持的 base64 格式:
//   - 纯 base64 字符串
//   - data URI: data:image/jpeg;base64,<base64>
func ParseUploadFile(c *gin.Context) (*UploadedFile, error) {
	contentType := c.GetHeader("Content-Type")

	if strings.Contains(contentType, "multipart/form-data") {
		return parseMultipart(c)
	}
	return parseBase64JSON(c)
}

// parseMultipart 从 multipart/form-data 提取文件
func parseMultipart(c *gin.Context) (*UploadedFile, error) {
	file, err := c.FormFile("file")
	if err != nil {
		return nil, fmt.Errorf("缺少 file 文件: %w", err)
	}
	f, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("文件打开失败: %w", err)
	}
	sortOrder, _ := strconv.Atoi(c.PostForm("sort_order"))
	return &UploadedFile{
		Filename:  file.Filename,
		Size:      file.Size,
		Reader:    f,
		SortOrder: sortOrder,
	}, nil
}

// parseBase64JSON 从 JSON body 提取 base64 编码文件
func parseBase64JSON(c *gin.Context) (*UploadedFile, error) {
	var body struct {
		File      string `json:"file"`      // base64 编码的文件数据
		Filename  string `json:"filename"`  // 原始文件名
		SortOrder int    `json:"sort_order"` // 排序序号 (可选)
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		return nil, fmt.Errorf("无法解析 JSON 请求体: %w", err)
	}
	if body.File == "" {
		return nil, fmt.Errorf("file 字段不能为空")
	}

	// 去除 data URI 前缀 (如 data:image/jpeg;base64,...)
	b64 := body.File
	if idx := strings.Index(b64, ","); idx >= 0 && strings.HasPrefix(b64, "data:") {
		b64 = b64[idx+1:]
	}

	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64 解码失败: %w", err)
	}

	// 文件名兜底
	filename := body.Filename
	if filename == "" {
		filename = "upload.jpg"
	}

	return &UploadedFile{
		Filename:  filename,
		Size:      int64(len(data)),
		Reader:    io.NopCloser(bytes.NewReader(data)),
		SortOrder: body.SortOrder,
	}, nil
}
