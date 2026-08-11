package handler

import (
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/pkg/response"
)

// maxAvatarSize 头像大小上限 2MB
const maxAvatarSize = 2 << 20

// UploadAvatar 公开头像上传 (POST /upload/avatar, multipart/form-data, 字段名 file)
// 注册前调用, 不经过 JWTAuth; 仅做大小/格式校验后转交图床
func (h *AuthHandler) UploadAvatar(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少 file 文件"))
		return
	}
	if file.Size <= 0 || file.Size > maxAvatarSize {
		response.Fail(c, apperrors.ErrImageInvalid.WithMessage("头像大小需不超过 2MB"))
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
		response.Fail(c, apperrors.ErrImageInvalid.WithMessage("仅支持 jpg/png/webp 格式"))
		return
	}

	f, err := file.Open()
	if err != nil {
		response.FailError(c, err)
		return
	}
	defer f.Close()

	result, err := h.imgBed.Upload(c.Request.Context(), file.Filename, f)
	if err != nil {
		response.Fail(c, apperrors.ErrOSSUpload.WithMessage("头像上传失败"))
		return
	}
	response.OK(c, gin.H{"url": result.URL})
}
