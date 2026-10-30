package database

import (
	"fmt"
	"io"
	"lechat-backend/config"
	"lechat-backend/internal/helpers"
	"lechat-backend/internal/observability"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var (
	DB *gorm.DB
)

func Connect(cfg *config.Config, logWriter io.Writer) {
	var err error

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.DB_HOST,
		cfg.DB_USER,
		cfg.DB_PASSWORD,
		cfg.DB_NAME,
		cfg.DB_PORT,
		cfg.SSL_MODE,
	)

	sqlLogger := gormlogger.New(log.New(logWriter, "", log.LstdFlags), gormlogger.Config{
		SlowThreshold:             cfg.DB_SLOW_QUERY_THRESHOLD,
		LogLevel:                  gormlogger.Warn,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
		Colorful:                  false,
	})
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: sqlLogger})

	if err != nil {
		helpers.IsPanic("connection not compatible! postgres", err)
		log.Println("failed to connect database (postgres): ", err)
	}

	if err := observability.InstrumentGORM(DB, logWriter, cfg.DB_SLOW_QUERY_THRESHOLD); err != nil {
		helpers.IsPanic("failed to instrument database", err)
	}

	log.Println("Connected Database!")
}
