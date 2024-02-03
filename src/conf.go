package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"
	"github.com/go-ini/ini"
)

var launchconf = "org.dotisopropyl.pagerduty-on-call.plist"

type Filter struct {
	property string
	match    string
	notmatch bool
	filter   *regexp.Regexp
}

func filterInit(filtertype string, cfg *ini.File) []Filter {
	var list []Filter
	for _, key := range cfg.Section(filtertype).KeyStrings() {
		filter, err := regexp.Compile(cfg.Section(filtertype).Key(key).String())
		s := strings.Split(key, ".")
		property, match := s[0], s[1]
		notmatch := strings.HasPrefix(match, "!")
		if notmatch {
			match = strings.Replace(match, "!", "", 1)
		}
		list = append(list, Filter{property: property, match: match, notmatch: notmatch, filter: filter})
	}
	return list
}

func cfgInit() (*ini.File, error) {
	var configFile string
	var timestampFile string
	configFile = fmt.Sprintf("%s/.on-call.config", os.Getenv("HOME"))
	timestampFile = fmt.Sprintf("%s/.on-call.time", os.Getenv("HOME"))
	_, err := ini.Load(configFile)
	if err != nil {
		input, err := ioutil.ReadFile("default.config")
		if err != nil {
			return nil, fmt.Errorf("Unable to load configuration %s; unable to load default configuration", configFile)
		}
		err = ioutil.WriteFile(configFile, input, 0644)
		if err != nil {
			return nil, fmt.Errorf("Unable to create default configuration %s: %w", configFile, err)
		}
		appNotify(
			"HOME/.on-call.config", "Saved default configuration; please edit and add PagerDuty API token",
			"https://github.com/dotisopropy/pagerduty-on-call", nil, 0)
		os.Exit(0)
	}
	cfg, err := ini.Load(configFile)
	if err != nil {
		appNotify(
			configFile, fmt.Sprintf("Unable to load configuration %s: %v", configFile, err),
			"https://github.com/dotisopropy/pagerduty-on-call", nil, 0)
		return nil, fmt.Errorf("Unable to load configuration %s: %w", configFile, err)
	}
	return cfg, nil
}

func readTimestamp() time.Time {
	var lastdate time.Time
	timestamp, err := ioutil.ReadFile(timestampFile)
	if err == nil {
		lastdate, err = time.Parse(time.RFC3339, string(timestamp))
		if err != nil {
			return time.Now().Add(time.Duration(-12) * time.Hour)
		}
		return lastdate
	}
	return time.Now().Add(time.Duration(-12) * time.Hour)
}

func writeTimestamp(timestamp time.Time) {
	err := ioutil.WriteFile(timestampFile, []byte(timestamp.Format(time.RFC3339)), 0644)
}

func writeLaunchConf() error {
	src := launchconf
	dst := fmt.Sprintf("%s/Library/LaunchAgents/%s", os.Getenv("HOME"), launchconf)
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}

func existsLaunchConf() bool {
	dst := fmt.Sprintf("%s/Library/LaunchAgents/%s", os.Getenv("HOME"), launchconf)
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return false
	}
	return true
}

func deleteLaunchConf() error {
	dst := fmt.Sprintf("%s/Library/LaunchAgents/%s", os.Getenv("HOME"), launchconf)
	return os.Remove(dst)
}