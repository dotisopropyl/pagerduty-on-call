package main

import (
	"runtime"
	"time"
)

var pauseTimeout int
var clearOnUnpause bool

func main() {
	runtime.LockOSThread()
	appInit()
	cfg, err := cfgInit()
	pdInit(cfg)
	interval, err := cfg.Section("pagerduty").Key("interval").Int()
	if err != nil {
		interval = 30
	}
	pauseTimeout, err = cfg.Section("main").Key("pause.timeout").Int()
	if err != nil {
		pauseTimeout = 0
	}
	clearOnUnpause, err = cfg.Section("main").Key("clear.on.unpause").Bool()
	if err != nil {
		clearOnUnpause = true
	}
	go func() {
		for {
			if pause {
				if pauseTimeout > 0 {
					if time.Now().After(pauseStopTime) {
						togglePause()
					}
				}
			} else {
				for _, incident := range pdGetIncidents(cfg) {
					pdNotify(incident)
				}
			}
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}()
	appEnterLoop()
}