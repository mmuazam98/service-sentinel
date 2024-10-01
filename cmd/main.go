package main

import (
	"flag"

	"github.com/mmuazam98/service-sentinel/config"
	"github.com/mmuazam98/service-sentinel/pkg/checker"
)

func main() {
	interval := flag.Int("interval", 1, "Interval in minutes for health checks")
	flag.Parse()

	serviceChecker := checker.NewServiceChecker(config.LoadConfig())
	serviceChecker.Run(*interval)
}
