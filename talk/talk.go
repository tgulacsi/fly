// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package talk

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tgulacsi/go/iohlp"
)

type HTTPClient struct{ http.Client }

func (c HTTPClient) Get(ctx context.Context, URL string) (*io.SectionReader, error) {
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	sr, err := iohlp.MakeSectionReader(resp.Body, 1<<20)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		var buf strings.Builder
		io.Copy(&buf, sr)
		return nil, fmt.Errorf("%s: %s", resp.Status, buf.String())
	}
	return sr, err
}
