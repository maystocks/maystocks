// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package properties

import "strconv"

func SetPositiveValue(n *int, value string) {
	valInt, err := strconv.Atoi(value)
	if err == nil && valInt > 0 {
		*n = valInt
	}
}
