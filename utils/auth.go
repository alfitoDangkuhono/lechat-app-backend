package utils

import "golang.org/x/crypto/bcrypt"

func ComparePassword(hashed string, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}

func HashPassword(textPassword string) string {
	bytes, _ := bcrypt.GenerateFromPassword([]byte(textPassword), bcrypt.DefaultCost)
	return string(bytes)
}
