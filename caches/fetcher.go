package caches

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ermes-thesis/ermes-common/utils/httpClient"
)

type Fetcher[T any] interface {
	Fetch(functionName string, functionVersion string) (T, error)
	FetchLatest(functionName string, cachedVersion string) (T, error)
}

type RemoteBundleFetcher[T any] struct {
	BaseURL     string
	ErmesSecret string
}

func NewRemoteBundleFetcher[T any](baseURL string, ermesSecret string) *RemoteBundleFetcher[T] {
	return &RemoteBundleFetcher[T]{BaseURL: baseURL, ErmesSecret: ermesSecret}
}

func (f *RemoteBundleFetcher[T]) Fetch(functionName string, functionVersion string) (*T, error) {
	url := fmt.Sprintf("%s/%s/%s", f.BaseURL, functionName, functionVersion)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-ERMES-SECRET", f.ErmesSecret)

	resp, err := httpClient.CustomClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch function: %s", string(body))
	}

	var fnBundle T
	err = json.Unmarshal(body, &fnBundle)
	return &fnBundle, err
}

func (f *RemoteBundleFetcher[T]) FetchLatest(functionName string, cachedVersion string) (*T, error) {
	url := fmt.Sprintf("%s/%s", f.BaseURL, functionName)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("If-None-Match", cachedVersion)

	req.Header.Set("X-ERMES-SECRET", f.ErmesSecret)

	resp, err := httpClient.CustomClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotModified {
		return nil, nil
	} else if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch function: %s", string(body))
	}

	var fnBundle T
	err = json.Unmarshal(body, &fnBundle)
	return &fnBundle, err
}
