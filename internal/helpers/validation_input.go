package helpers

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func CustomValidationInput(ctx *gin.Context, err error) {
	var ve validator.ValidationErrors

	if errors.As(err, &ve) {
		out := map[string]string{}

		for _, fe := range ve {
			out[fe.Field()] = validationMessage(fe)
		}
		ctx.JSON(400, gin.H{
			"message": "validation error",
			"error":   out,
		})
		return
	}
	ctx.JSON(400, gin.H{
		"message": err.Error(),
	})
}

func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "This field is required " + fe.Field()
	case "email":
		return "invalid email format"
	case "min":
		return "Minimum length is " + fe.Field() + ": " + fe.Param()
	case "max":
		return "Maximum lenght is " + fe.Field() + ": " + fe.Param()
	case "phone":
		return "invalid phone number, must be 10 digits"
	}
	return fe.Error()
}
