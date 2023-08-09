// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package webclient

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
)

func ParseJsonResponse(resp *http.Response, v any) error {
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, err := io.ReadAll(resp.Body)
		if err == nil && len(b) > 0 {
			return fmt.Errorf("%s (status %d)", b, resp.StatusCode)
		}
		return fmt.Errorf("query returned status code %d", resp.StatusCode)
	}

	m, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || m != "application/json" {
		return fmt.Errorf("invalid content type %s", resp.Header.Get("Content-Type"))
	}

	if err = json.NewDecoder(resp.Body).Decode(v); err != nil {
		return err
	}
	return nil
}
