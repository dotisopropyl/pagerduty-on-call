package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"text/template"
	"time"
	"github.com/go-ini/ini"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/shurcooL/trayhost"
    "golang.org/x/text/cases"
    "golang.org/x/text/language"
)

var excludeFilters = []Filter{}
var includeFilters = []Filter{}
var location = time.Local
var pd *pagerduty.Client
var serviceIDs = []string{}
var statuses = []string{"triggered", "acknowledged", "resolved"}
var teamIDs = []string{}
var titleTemplate *template.Template = nil
var userIDs = []string{}

func format(str string) (string, error) {
	date, _ := time.Parse(time.RFC3339, str)
	return date.In(location).Format("3:04 PM"), nil
}

func pdInit(cfg *ini.File) {

	includeFilters = filterInit("include", cfg)
	excludeFilters = filterInit("exclude", cfg)
	pd = pagerduty.NewClient(cfg.Section("pagerduty").Key("token").String())
	timezone := cfg.Section("main").Key("timezone").String()

	if timezone != "" {
		var err error
		location, err = time.LoadLocation(timezone)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancel()
	teamList, err := pd.ListTeamsWithContext(ctx, pagerduty.ListTeamOptions{Query: ""})

	if err != nil {
		os.Exit(1)
	}

	teamConf := make(map[string]bool)

	for _, v := range cfg.Section("pagerduty").Key("teams").Strings(",") {
		teamConf[v] = true
	}

	for _, t := range teamList.Teams {
		if teamConf[t.Name] {
			teamIDs = append(teamIDs, t.ID)
		}
	}

	userList, err := pd.ListUsersWithContext(ctx, pagerduty.ListUsersOptions{Query: ""})

	if err != nil {
		os.Exit(1)
	}

	userConf := make(map[string]bool)

	for _, v := range cfg.Section("pagerduty").Key("users").Strings(",") {
		userConf[v] = true
	}

	for _, u := range userList.Users {
		if userConf[u.Email] || userConf[u.Name] {
			userIDs = append(userIDs, u.ID)
		}
	}

	serviceConf := make(map[string]bool)

	for _, v := range cfg.Section("pagerduty").Key("services").Strings(",") {
		serviceConf[v] = true
	}

	ok := true

	opts := pagerduty.ListServiceOptions{
		Limit:  25,
		Offset: 0,
	}

	for k := range serviceConf {
		for ok {
			opts.Query = k
			serviceList, err := pd.ListServicesWithContext(ctx, opts)
			if err != nil {
				os.Exit(1)
			}
			for _, s := range serviceList.Services {
				if serviceConf[s.Name] {
					serviceIDs = append(serviceIDs, s.ID)
				}
			}
			ok = serviceList.More
			opts.Offset = opts.Offset + opts.Limit
		}
	}

	var fm = make(template.FuncMap)
	fm["format"] = format
	title := cfg.Section("pagerduty").Key("title").String()
	if title != "" {
		titleTemplate, err = template.New("title").Funcs(fm).Parse(title)
		if err != nil {
			os.Exit(1)
		}
	}
}

func pdGetIncidents(cfg *ini.File) []pagerduty.Incident {
	lastdate := readTimestamp()
	incidents := make([]pagerduty.Incident, 0)

incidents:
	for _, i := range pdGetIncidentsSince(lastdate) {
		lastdate, _ = time.Parse(time.RFC3339, i.CreatedAt)
		if len(includeFilters) == 0 {
			goto excludes
		}
		for _, filter := range includeFilters {
			switch filter.property {
			case "service":
				if (filter.notmatch && (i.Service.Summary != filter.match)) || (!filter.notmatch && (i.Service.Summary == filter.match)) {
					if filter.filter.Find([]byte(i.Summary)) != nil {
						goto excludes
					}
				}
			case "team":
				for _, t := range i.Teams {
					if (filter.notmatch && (t.Summary != filter.match)) || (!filter.notmatch && (t.Summary == filter.match)) {
						if filter.filter.Find([]byte(i.Summary)) != nil {
							goto excludes
						}
					}
				}
			default:
			}
		}
		continue incidents
	excludes:
		for _, filter := range excludeFilters {
			switch filter.property {
			case "service":
				if (filter.notmatch && i.Service.Summary != filter.match) || (!filter.notmatch && (i.Service.Summary == filter.match)) {
					if filter.filter.Find([]byte(i.Summary)) != nil {
						continue incidents
					}
				}
			case "team":
				for _, t := range i.Teams {
					if (filter.notmatch && t.Summary != filter.match) || (!filter.notmatch && (t.Summary == filter.match)) {
						if filter.filter.Find([]byte(i.Summary)) != nil {
							continue incidents
						}
					}
				}
			default:
			}
		}
		incidents = append(incidents, i)
	}
	writeTimestamp(lastdate.Add(time.Second))
	return incidents
}

func pdGetIncidentsSince(since time.Time) []pagerduty.Incident {
	incidents := make([]pagerduty.Incident, 0)
	opts := pagerduty.ListIncidentsOptions{
		Limit:      25,
		Offset:     0,
		Since:      since.Format(time.RFC3339),
		Statuses:   statuses,
		TeamIDs:    teamIDs,
		UserIDs:    userIDs,
		ServiceIDs: serviceIDs,
		SortBy:     "created_at:ASC",
		TimeZone:   "UTC",
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancel()
	ok := true
	for ok {
		resp, err := pd.ListIncidentsWithContext(ctx, opts)
		if err != nil {
			return incidents
		}
		ok = resp.More
		incidents = append(incidents, resp.Incidents...)
		opts.Offset = opts.Offset + opts.Limit
	}
	return incidents
}

func pdNotify(i pagerduty.Incident) {
	date, _ := time.Parse(time.RFC3339, i.CreatedAt)
	reg := regexp.MustCompile(`\[#(\d+)\] (.+)`)
	title := fmt.Sprintf("Incident %s at %s", i.Status, date.In(location).Format("3:04 PM"))
    fmt.Println(cases.Title(language.English, cases.Compact).String(title))
	if titleTemplate != nil {
		var tpl bytes.Buffer
		err := titleTemplate.Execute(&tpl, i)
		if err == nil {
			title = tpl.String()
		}
	}
	var message string
	match := reg.FindStringSubmatch(i.Summary)
	if match == nil {
		message = removeCharacters(i.Summary, "[]")
	} else {
		message = removeCharacters(match[2], "[]")
	}
	image := trayhost.Image{}
	if i.Urgency == "high" {
		image.Kind = "png"
		image.Bytes = getIcon("warning.png")
	}
	appNotify(title, message, i.HTMLURL, &image, 0)
}