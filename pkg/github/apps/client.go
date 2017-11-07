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
	"net/http"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
)

// Shared transport to reuse TCP connections.
var tr = &http.Transport{}

func Client(appID, installationID int, privateKey []byte) (*github.Client, error) {
	itr, err := NewTransport(tr, appID, installationID, privateKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create transport from private key file")
	}

	return github.NewClient(&http.Client{Transport: itr}), nil
}
