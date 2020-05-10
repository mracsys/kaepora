// package ootrapi is an API client for the ootrandomizer.com HTTP API v2
package ootrapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"time"

	"golang.org/x/time/rate"
)

type API struct {
	http    http.Client
	key     string
	limiter *rate.Limiter
}

func New(key string) *API {
	return &API{
		// We're allowed 20 requests per 10 second
		limiter: rate.NewLimiter(2, 10),
		key:     key,
		http: http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type SeedStatus int

const (
	// the API zero value being a valid value, we need another invalid value.
	SeedStatusInvalid SeedStatus = -1

	SeedStatusGenerating   SeedStatus = 0 // in progress
	SeedStatusDone         SeedStatus = 1 // seed available to download
	SeedStatusDoneWithLink SeedStatus = 2 // "not possible from API"
	SeedStatusFailed       SeedStatus = 3 // generation failed, this won't fail gracefully for now
)

func (s SeedStatus) IsValid() bool {
	return s >= 0 && s <= 3
}

var errTODO = errors.New("not implemented")

func (api *API) getURL(subPath string, q url.Values) string {
	q.Set("key", api.key)

	u := url.URL{
		Scheme:   "https",
		Host:     "ootrandomizer.com",
		Path:     path.Join("/api/v2", subPath),
		RawQuery: q.Encode(),
	}
	return u.String()
}

func (api *API) CreateSeed(version string, settings map[string]interface{}) (string, error) {
	log.Print("debug: creating API seed")
	if err := api.limiter.Wait(context.TODO()); err != nil {
		return "", err
	}

	body, err := json.Marshal(settings)
	if err != nil {
		return "", err
	}

	url := api.getURL("/seed/create", url.Values{
		"version": {version},
		"locked":  {"1"},
	})
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")

	var response struct {
		ID       string `json:"id"`
		Version  string `json:"version"`
		Spoilers bool   `json:"spoilers"`
	}
	if err := api.do(request, &response); err != nil {
		return "", err
	}

	if response.Spoilers {
		log.Printf("warning: API ignored the locked parameter")
	}
	if response.Version != version {
		log.Printf("warning: API version mismatch, expected '%s' got '%s'", version, response.Version)
	}

	log.Printf("debug: API got seed ID %s", response.ID)

	return response.ID, nil
}

func (api *API) GetSeedStatus(id string) (SeedStatus, error) {
	log.Printf("debug: fetching API seed status for ID  %s", id)

	if err := api.limiter.Wait(context.TODO()); err != nil {
		return SeedStatusInvalid, err
	}

	url := api.getURL("/seed/status", url.Values{"id": {id}})
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return SeedStatusInvalid, err
	}

	var res struct {
		Status   SeedStatus `json:"status"`
		Progress int        `json:"progress"` // 0-100
		// ignored: version, positionQueue, maxWaitTime, isMultiWorld
	}

	if err := api.do(request, &res); err != nil {
		return SeedStatusInvalid, err
	}
	log.Printf("debug: API seed %s status: %d (%d%%)", id, res.Status, res.Progress)

	if !res.Status.IsValid() {
		log.Printf("error: API returned invalid seed status: %d", res.Status)
	}

	return res.Status, nil
}

func (api *API) GetSeedSpoilerLog(id string) ([]byte, error) {
	log.Printf("debug: fetching API seed spoiler log for ID  %s", id)

	if err := api.limiter.Wait(context.TODO()); err != nil {
		return nil, err
	}

	url := api.getURL("/seed/details", url.Values{"id": {id}})
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var res struct {
		SpoilerLog json.RawMessage `json:"spoilerLog"`
	}
	if err := api.do(request, &res); err != nil {
		return nil, err
	}

	return res.SpoilerLog, nil
}

func (api *API) GetSeedPatch(id string) ([]byte, error) {
	log.Printf("debug: fetching API seed patch for ID  %s", id)

	if err := api.limiter.Wait(context.TODO()); err != nil {
		return nil, err
	}

	url := api.getURL("/seed/patch", url.Values{"id": {id}})
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := api.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got status code %d", res.StatusCode)
	}

	return ioutil.ReadAll(res.Body)
}

func (api *API) UnlockSeedSpoilerLog(id string) error {
	if err := api.limiter.Wait(context.TODO()); err != nil {
		return err
	}

	return errTODO
}

func (api *API) do(request *http.Request, response interface{}) error {
	res, err := api.http.Do(request)
	if err != nil {
		return fmt.Errorf("unable to perform HTTP request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("got status code %d", res.StatusCode)
	}

	dec := json.NewDecoder(res.Body)
	if err := dec.Decode(&response); err != nil {
		return fmt.Errorf("unable to parse response: %s", err)
	}

	return nil
}
