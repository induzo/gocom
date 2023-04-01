package database

import "embed"

//go:embed migrations/*
var TestMigrationFiles embed.FS
