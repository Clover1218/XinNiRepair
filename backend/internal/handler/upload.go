package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/pkg/response"
)

// maxAvatarSize 头像大小上限 2MB
const maxAvatarSize = 2 << 20

// UploadAvatar 公开头像上传 (POST /upload/avatar)
// 智能识别两种 Content-Type:
//   - multipart/form-data: 字段名 file (二进制文件流)
//   - application/json: { "file": "<base64>", "filename": "xxx.jpg" }
//
// 注册前调用, 不经过 JWTAuth; 仅做大小/格式校验后转交图床
func (h *AuthHandler) UploadAvatar(c *gin.Context) {
	uf, err := ParseUploadFile(c)
	if err != nil {
		h.logger.Warn("UploadAvatar: parse file failed", zap.Error(err))
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少 file 文件或 base64 数据"))
		return
	}
	defer uf.Close()

	if uf.Size <= 0 || uf.Size > maxAvatarSize {
		h.logger.Warn("UploadAvatar: file size invalid", zap.Int64("size", uf.Size))
		response.Fail(c, apperrors.ErrImageInvalid.WithMessage("头像大小需不超过 2MB"))
		return
	}

	ext := uf.Ext()
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
		h.logger.Warn("UploadAvatar: invalid file extension", zap.String("ext", ext))
		response.Fail(c, apperrors.ErrImageInvalid.WithMessage("仅支持 jpg/png/webp 格式"))
		return
	}

	result, err := h.imgBed.Upload(c.Request.Context(), uf.Filename, uf.Reader)
	if err != nil {
		h.logger.Error("UploadAvatar: img bed upload failed", zap.Error(err), zap.String("filename", uf.Filename))
		response.Fail(c, apperrors.ErrOSSUpload.WithMessage("头像上传失败"))
		return
	}
	response.OK(c, gin.H{"url": result.URL})
}
