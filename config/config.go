package config

import (
	"lechat-backend/utils"
	"log"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DB_HOST                 string
	DB_PORT                 string
	DB_PASSWORD             string
	DB_USER                 string
	DB_NAME                 string
	SSL_MODE                string
	DB_SLOW_QUERY_THRESHOLD time.Duration
	APP_CONFIG              AppConfig
	CORS_CONFIG             CORSConfig
	SECURITY_CONFIG         ConfigSecurity
	S3_CONFIG               S3Config
}
type AppConfig struct {
	APP_PORT string
}
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           time.Duration
}

type ConfigSecurity struct {
	MaxAttempLogin     int
	BlockTime          int
	AttemptReset       int
	ExpireToken        time.Duration
	ExpireRefreshToken time.Duration
	JwtSecret          string
	IssuerSecretJwt    string
}

type S3Config struct {
	EndpointURL     string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
}

func LoadConfig() *Config {
	err := godotenv.Load(".env")

	if err != nil {
		log.Println("warning: .env file not found, using default value")
	}

	return &Config{
		DB_HOST:                 utils.GetEnv("DATABASE_HOST", "127.0.0.1"),
		DB_PORT:                 utils.GetEnv("DATABASE_PORT", "5432"),
		DB_PASSWORD:             utils.GetEnv("DATABASE_PASSWORD", ""),
		DB_USER:                 utils.GetEnv("DATABASE_USER", "postgres"),
		DB_NAME:                 utils.GetEnv("DATABASE_NAME", ""),
		SSL_MODE:                utils.GetEnv("SSL_MODE", "disable"),
		DB_SLOW_QUERY_THRESHOLD: utils.GetEnvAsDuration("DB_SLOW_QUERY_THRESHOLD", 200*time.Millisecond),
		APP_CONFIG: AppConfig{
			APP_PORT: utils.GetEnv("APP_PORT", "8989"),
		},
		CORS_CONFIG: CORSConfig{
			AllowOrigins:     utils.GetEnvSlice("CORS_ALLOW_ORIGINS", []string{"http://localhost:5173"}),
			AllowMethods:     utils.GetEnvSlice("CORS_ALLOW_METHODS", []string{"GET", "POST", "PUT", "DELETE"}),
			AllowHeaders:     utils.GetEnvSlice("CORS_ALLOW_HEADERS", []string{"Origin", "Content-Type", "Accept", "Authorization"}),
			ExposeHeaders:    utils.GetEnvSlice("CORS_EXPOSE_HEADERS", []string{"Content-Length"}),
			AllowCredentials: utils.GetEnvAsBoolean("CORS_ALLOW_CREDENTIALS", true),
			MaxAge:           utils.GetEnvAsDuration("CORS_MAX_AGE", 12*time.Hour),
		},
		SECURITY_CONFIG: ConfigSecurity{
			MaxAttempLogin:     utils.GetEnvAsInt("MAX_ATTEMPT_LOGIN", 3),
			BlockTime:          utils.GetEnvAsInt("BLOCK_TIME", 15),
			AttemptReset:       utils.GetEnvAsInt("ATTEMPT_RESET", 10),
			ExpireToken:        utils.GetEnvAsDuration("EXPIRE_TOKEN", 40),
			ExpireRefreshToken: utils.GetEnvAsDuration("EXPIRE_REFRESH_TOKEN", 24),
			JwtSecret:          utils.GetEnv("JWT_SECRET_KEY", "oDkdW2klaug+ehDRH2CxfQ=="),
			IssuerSecretJwt:    utils.GetEnv("ISSUER_SECRET_JWT", "xxx-xxx--xxx--xx"),
		},
		S3_CONFIG: S3Config{
			EndpointURL:     utils.GetEnv("S3_ENDPOINT_URL", "http://127.0.0.1:9003"),
			Region:          utils.GetEnv("S3_REGION", "us-east-1"),
			AccessKeyID:     utils.GetEnv("S3_ACCESS_KEY_ID", "xxxx-xxxx"),
			SecretAccessKey: utils.GetEnv("S3_SECRET_ACCESS_KEY", "xxx-xxx"),
			Bucket:          utils.GetEnv("S3_BUCKET", "xxxx-xxxx"),
		},
	}
}
