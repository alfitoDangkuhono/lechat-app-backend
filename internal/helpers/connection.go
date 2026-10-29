package helpers

import (
	"lechat-backend/utils"
)

func IsPanic(message string, errors error) {
	utils.NewLogger().Error(errors)
	panic(message)
}
