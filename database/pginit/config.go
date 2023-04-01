package pginit

import "time"

// Config allow you to set database credential to connect to database
type Config struct {
	User         string
	Password     string
	Host         string
	Port         string
	Database     string
	MaxConns     int32
	MaxIdleConns int32
	MaxLifeTime  time.Duration
}
