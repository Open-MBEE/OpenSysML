// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package engine

import (
	"context"
	"io"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
)

// Serve reads Content-Length-delimited frames from r and writes answers to w
// until r reaches its end, speaking the JSON half of stdiorpc's protocol: the
// same framing, envelope and error codes, with a protobuf frame refused since
// this product carries no protobuf. Calls are handled sequentially and answered
// in order.
func (e *Engine) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	return jsonrpc.Serve(ctx, r, w, e.Call)
}
