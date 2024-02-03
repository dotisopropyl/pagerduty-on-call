package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"github.com/shurcooL/trayhost"
)

var menuItems = []trayhost.MenuItem{
	{
		Title: "Add to Login Items",
		Handler: func() {
			toggleStartup()
		},
	},
	{
		Title: "Pause Notifications",
		Handler: func() {
			togglePause()
		},
	},
	{
		Title: "Open on GitHub",
		Handler: func() {
			openBrowser("https://github.com/dotisopropyl/pagerduty-on-call")
		},
	},
	trayhost.SeparatorMenuItem(),
	{
		Title:   "Quit",
		Handler: trayhost.Exit,
	},
}

var menuItemsCopy = []trayhost.MenuItem{}
var pause = false
var pauseStopTime time.Time

func appInit() {
	ep, err := os.Executable()
	err = os.Chdir(filepath.Join(filepath.Dir(ep), "..", "Resources"))
	iconData, err := ioutil.ReadFile("menubar.png")
	if existsLaunchConf() {
		for i, m := range menuItems {
			if m.Title == "Add to Login Items" {
				menuItems[i].Title = "Remove from Login Items"
			}
		}
	}
	menuItemsCopy = append(menuItemsCopy, menuItems...)
	trayhost.Initialize("On-Call", iconData, menuItems)
}

func togglePause() {
	if pause {
		appNotify("On-Call", "Resuming Notifications", "", nil, 10*time.Second)
		for i, m := range menuItemsCopy {
			if m.Title == "Resume Notifications" {
				menuItemsCopy[i].Title = "Pause Notifications"
			}
		}
		trayhost.UpdateMenu(menuItemsCopy)
		pause = false
		if clearOnUnpause {
			writeTimestamp(time.Now())
		}
	} else {
		msg := "Pausing Notifications"
		if pauseTimeout > 0 {
			msg = fmt.Sprintf("%s for %d minutes", msg, pauseTimeout)
			pauseStopTime = time.Now().Add(time.Duration(pauseTimeout) * time.Minute)
		}
		appNotify("On-Call", msg, "", nil, 10*time.Second)
		for i, m := range menuItemsCopy {
			if m.Title == "Pause Notifications" {
				menuItemsCopy[i].Title = "Resume Notifications"
			}
		}
		trayhost.UpdateMenu(menuItemsCopy)
		pause = true
	}
}

func toggleStartup() {
	if existsLaunchConf() {
		if err := deleteLaunchConf(); err != nil {
			appNotify("On-Call", fmt.Sprintf("Unable to Remove from Login Items: %v", err), "", nil, 10*time.Second)
		}
		appNotify("On-Call", "Removed from Login Items", "", nil, 10*time.Second)
		for i, m := range menuItemsCopy {
			if m.Title == "Remove from Login Items" {
				menuItemsCopy[i].Title = "Add to Login Items"
			}
		}
		trayhost.UpdateMenu(menuItemsCopy)
	} else {
		if err := writeLaunchConf(); err != nil {
			appNotify("On-Call", fmt.Sprintf("Unable to Add to Login Items: %v", err), "", nil, 10*time.Second)
			return
		}
		appNotify("On-Call", "Added to Login Items", "", nil, 10*time.Second)
		for i, m := range menuItemsCopy {
			if m.Title == "Add to Login Items" {
				menuItemsCopy[i].Title = "Remove from Login Items"
			}
		}
		trayhost.UpdateMenu(menuItemsCopy)
	}
}

func appEnterLoop() {
	trayhost.EnterLoop()
}

func appNotify(title string, message string, url string, image *trayhost.Image, timeout time.Duration) {
	notification := trayhost.Notification{
		Title:   title,
		Body:    message,
		Timeout: timeout,
	}
	if url != "" {
		notification.Handler = func() { openBrowser(url) }
	}
	if image != nil {
		notification.Image = *image
	}
	notification.Display()
}

func removeCharacters(input string, characters string) string {
	filter := func(r rune) rune {
		if !strings.ContainsRune(characters, r) {
			return r
		}
		return -1
	}
	return strings.Map(filter, input)
}

func getIcon(s string) []byte {
	b, err := ioutil.ReadFile(s)
	if err != nil {
		fmt.Print(err)
	}
	return b
}

func openBrowser(url string) {
	var err error
	err = exec.Command("open", url).Start()
}