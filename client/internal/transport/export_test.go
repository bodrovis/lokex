package transport

import (
	"context"
	"io"
	"net/http"
)

func ExportDo(
	r *Requester,
	ctx context.Context,
	method, path string,
	body io.Reader,
	v any,
	headers http.Header,
) error {
	return r.do(ctx, method, path, body, v, headers)
}

func ExportNewRequest(
	r *Requester,
	ctx context.Context,
	method, path string,
	body io.Reader,
	headers http.Header,
) (*http.Request, error) {
	return r.newRequest(ctx, method, path, body, headers)
}

func ExportMergeHeaders(dst, src http.Header) {
	mergeHeaders(dst, src)
}

func ExportHandleResponse(resp *http.Response, v any) error {
	return handleResponse(resp, v)
}

func ExportDecodeJSONResponse(resp *http.Response, v any) error {
	return decodeJSONResponse(resp, v)
}
