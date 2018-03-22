// Copyright © 2017 Syndesis Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apps

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
)

const appsAcceptHeader = "application/vnd.github.machine-man-preview+json"

var apiBaseURL = strings.TrimSuffix(github.NewClient(http.DefaultClient).BaseURL.String(), "/")

// Transport provides a http.RoundTripper by wrapping an existing
// http.RoundTripper and provides GitHub App authentication as an
// installation.
//
// See https://developer.github.com/apps/building-integrations/setting-up-and-registering-github-apps/about-authentication-options-for-github-apps/#authenticating-as-an-installation
type Transport struct {
	BaseURL        string            // baseURL is the scheme and host for GitHub API, defaults to https://api.github.com
	tr             http.RoundTripper // tr is the underlying roundtripper being wrapped
	key            *rsa.PrivateKey   // key is the GitHub Apps's private key
	appID          int64               // appID is the GitHub App's ID
	installationID int64               // installationID is the GitHub Apps's Installation ID

	mu    *sync.Mutex  // mu protects token
	token *accessToken // token is the installation's access token
}

// accessToken is an installation access token response from GitHub
type accessToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

var _ http.RoundTripper = &Transport{}

// NewTransportFromKeyFile returns an Transport using a private key from file.
func NewTransportFromKeyFile(tr http.RoundTripper, appID, installationID int64, privateKeyFile string) (*Transport, error) {
	privateKey, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		return nil, errors.Wrap(err, "could not read private key")
	}
	return NewTransport(tr, appID, installationID, privateKey)
}

// NewTransport returns an Transport using private key. The key is parsed
// and if any errors occur the transport is nil and error is non-nil.
//
// The provided tr http.RoundTripper should be shared between multiple
// installations to ensure reuse of underlying TCP connections.
//
// The returned Transport is safe to be used concurrently.
func NewTransport(tr http.RoundTripper, appID, installationID int64, privateKey []byte) (*Transport, error) {
	t := &Transport{
		tr:             tr,
		appID:          appID,
		installationID: installationID,
		BaseURL:        apiBaseURL,
		mu:             &sync.Mutex{},
	}
	var err error
	t.key, err = jwt.ParseRSAPrivateKeyFromPEM(privateKey)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse private key")
	}
	return t, nil
}

// RoundTrip implements http.RoundTripper interface.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	if t.token == nil || t.token.ExpiresAt.Add(-time.Minute).Before(time.Now()) {
		// Token is not set or expired/nearly expired, so refresh
		if err := t.refreshToken(); err != nil {
			t.mu.Unlock()
			return nil, errors.Wrapf(err, "could not refresh installation id %d's token", t.installationID)
		}
	}
	t.mu.Unlock()

	req.Header.Set("Authorization", "token "+t.token.Token)
	resp, err := t.tr.RoundTrip(req)
	return resp, err
}

func (t *Transport) refreshToken() error {
	// TODO these claims could probably be reused between installations before expiry
	claims := &jwt.StandardClaims{
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
		Issuer:    strconv.FormatInt(t.appID,10),
	}
	bearer := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	ss, err := bearer.SignedString(t.key)
	if err != nil {
		return errors.Wrap(err, "could not sign jwt")
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/installations/%d/access_tokens", t.BaseURL, t.installationID), nil)
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", ss))
	req.Header.Set("Accept", appsAcceptHeader)

	client := &http.Client{Transport: t.tr}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "could not get access_tokens from GitHub API for installation ID %d", t.installationID)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		body, _ := ioutil.ReadAll(resp.Body)
		return errors.Errorf("received non 2xx response status %d (%q) when fetching %v. Response body: %s", resp.StatusCode, resp.Status, req.URL, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(&t.token); err != nil {
		return err
	}

	return nil
}
