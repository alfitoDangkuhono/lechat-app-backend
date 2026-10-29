package utils

import "github.com/gin-gonic/gin"

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func ErrorResponse(ctx *gin.Context, code int, message error, data interface{}) {
	ctx.JSON(code, Response{
		Code:    code,
		Message: message.Error(),
		Data:    data,
	})
}

func SuccessResponse(ctx *gin.Context, code int, data interface{}) {
	ctx.JSON(code, Response{
		Code:    code,
		Message: "success",
		Data:    data,
	})
}
