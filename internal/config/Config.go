package config

import (
	"log/slog"
	"os"
	"strings"
)

var Heritage string
var ProgramLevel = new(slog.LevelVar)

const VipV4Annotation string = "fgt.sncs-uk.io/ipv4-vip"
const VipV6Annotation string = "fgt.sncs-uk.io/ipv6-vip"
const PoolAnnotation string = "fgt.sncs-uk.io/ip-pool"

func LoadConfig() {
	setLogLevel()
	loadHeritage()
}

func loadHeritage() {
	var ok bool
	Heritage, ok = os.LookupEnv("VIP_HERITAGE")
	if !ok {
		Heritage = "kubernetes"
	}
}

func setLogLevel() {
	level := os.Getenv("LOG_LEVEL")

	switch strings.ToLower(level) {
	case "debug":
		ProgramLevel.Set(slog.LevelDebug)
	case "info":
		ProgramLevel.Set(slog.LevelInfo)
	case "warn":
		ProgramLevel.Set(slog.LevelWarn)
	case "error":
		ProgramLevel.Set(slog.LevelError)
	default:
		ProgramLevel.Set(slog.LevelInfo)
	}
}
