package transport

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/bodrovis/lokex/v2/internal/apierr"
)

type Requester struct {
	BaseURL    string
	Token      string
	UserAgent  string
	HTTPClient *http.Client
}

// DoJSON performs one HTTP request expecting a JSON API response.
// If body is non-nil, Content-Type is set to application/json.
// Accept is set to application/json by do.
func (r *Requester) DoJSON(
	ctx context.Context,
	method, path string,
	body io.Reader,
	v any,
) error {
	headers := make(http.Header)
	if body != nil {
		headers.Set("Content-Type", "application/json")
	}
	return r.do(ctx, method, path, body, v, headers)
}

// Body is sent as-is. If v is non-nil, the response body is decoded as JSON.
func (r *Requester) do(
	ctx context.Context,
	method, path string,
	body io.Reader,
	v any,
	headers http.Header,
) error {
	req, err := r.newRequest(ctx, method, path, body, headers)
	if err != nil {
		return err
	}

	if r.HTTPClient == nil {
		return fmt.Errorf("send request: nil http client")
	}

	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		// after Do() net/http already handled closing the request body.
		return fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return handleResponse(resp, v)
}

func (r *Requester) newRequest(
	ctx context.Context,
	method, path string,
	body io.Reader,
	headers http.Header,
) (*http.Request, error) {
	closeBody := func() {
		if body == nil {
			return
		}
		if cl, ok := body.(io.Closer); ok {
			_ = cl.Close()
		}
	}

	fullURL, err := url.JoinPath(r.BaseURL, path)
	if err != nil {
		closeBody()
		return nil, fmt.Errorf("join url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		closeBody()
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Api-Token", r.Token)
	req.Header.Set("User-Agent", r.UserAgent)
	req.Header.Set("Accept", "application/json")

	mergeHeaders(req.Header, headers)

	return req, nil
}

func mergeHeaders(dst, src http.Header) {
	for k, vv := range src {
		if len(vv) == 0 {
			continue
		}
		dst[k] = append([]string(nil), vv...)
	}
}

func handleResponse(resp *http.Response, v any) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp)
	}

	if v == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	return decodeJSONResponse(resp, v)
}

func parseAPIError(resp *http.Response) error {
	slurp, _ := io.ReadAll(
		io.LimitReader(resp.Body, apierr.DefaultErrCap),
	)

	ae := apierr.Parse(slurp, resp.StatusCode)
	ae.Resp = resp

	return ae
}

func decodeJSONResponse(resp *http.Response, v any) error {
	cr := &countingReader{r: resp.Body}

	if err := json.UnmarshalRead(cr, v); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			if resp.ContentLength > 0 && cr.n < resp.ContentLength {
				return fmt.Errorf("read response: %w", io.ErrUnexpectedEOF)
			}

			return fmt.Errorf("decode response: unexpected end of JSON input")
		}

		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}
