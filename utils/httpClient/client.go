package httpClient

import "net/http"

var CustomClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConnsPerHost: 100,
	},
}
