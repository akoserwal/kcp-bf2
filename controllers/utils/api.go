package utils

import "time"

const (
	DefaultAuthURL  = "https://sso.redhat.com/auth/realms/redhat-external"
	DefaultAPIURL   = "https://api.openshift.com"
	DefaultClientID = "cloud-services"
	// gap between API polls
	WatchInterval = 5 * time.Second
)
