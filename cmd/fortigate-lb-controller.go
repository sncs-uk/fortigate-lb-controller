package main

import (
	"log/slog"
	"os"

	"github.com/sncs-uk/fortigate-lb-controller/internal/config"
	"github.com/sncs-uk/fortigate-lb-controller/internal/mainlogic"
)

func main() {
	// Sort out the logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: config.ProgramLevel}))
	slog.SetDefault(logger)

	mainlogic.Run()
}
