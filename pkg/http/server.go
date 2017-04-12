// Copyright © 2017 Red Hat iPaaS Authors
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

package http

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/redhat-ipaas/pure-bot/pkg/config"
)

func New(cfg config.HTTPConfig, h http.HandlerFunc) *Server {
	srv := &Server{
		cfg: cfg,
		http: &http.Server{
			Addr:    net.JoinHostPort(cfg.Address, strconv.Itoa(cfg.Port)),
			Handler: h,
		},
	}
	return srv
}

type Server struct {
	cfg  config.HTTPConfig
	http *http.Server
}

func (s *Server) Start() error {
	var err error
	if s.cfg.TLSCert != "" && s.cfg.TLSKey != "" {
		err = s.http.ListenAndServeTLS(s.cfg.TLSCert, s.cfg.TLSKey)
	} else {
		err = s.http.ListenAndServe()
	}
	if err != nil {
		return errors.Wrap(err, "web server failed")
	}
	return nil
}

func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.http.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "failed to cleanly shutdown web server")
	}
	return nil
}
