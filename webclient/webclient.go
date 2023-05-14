// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package webclient

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
)

func ParseJsonResponse(resp *http.Response, v any) error {
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("query returned error code %d", resp.StatusCode)
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
