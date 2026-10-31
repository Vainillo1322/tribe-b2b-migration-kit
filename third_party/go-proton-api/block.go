package proton

import (
	"context"
	"io"

	"github.com/go-resty/resty/v2"
)

func (c *Client) GetBlock(ctx context.Context, bareURL, token string) (io.ReadCloser, error) {
	res, err := c.doRes(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetHeader("pm-storage-token", token).SetDoNotParseResponse(true).Get(bareURL)
	})
	if err != nil {
		return nil, err
	}

	return res.RawBody(), nil
}

// VerifyRevision fetches the verification data needed to derive per-block
// upload tokens before POST /drive/blocks (new upload protocol). The API
// returns the fields at the top level of the response body.
func (c *Client) VerifyRevision(ctx context.Context, shareID, linkID, revisionID string) (*RevisionVerification, error) {
	var res RevisionVerification

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).
			Get("/drive/shares/" + shareID + "/links/" + linkID + "/revisions/" + revisionID + "/verification")
	}); err != nil {
		return nil, err
	}

	return &res, nil
}

func (c *Client) RequestBlockUpload(ctx context.Context, req BlockUploadReq) ([]BlockUploadLink, error) {
	var res struct {
		UploadLinks []BlockUploadLink
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).SetBody(req).Post("/drive/blocks")
	}); err != nil {
		return nil, err
	}

	return res.UploadLinks, nil
}

func (c *Client) UploadBlock(ctx context.Context, bareURL, token string, block io.Reader) error {
	return c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.
			SetHeader("pm-storage-token", token).
			// Only the storage token identifies us here: no API auth
			// headers. The multipart part is field "Block", filename
			// "blob", without an explicit Content-Type (empty string
			// makes resty omit the per-part header).
			SetMultipartField("Block", "blob", "", block).
			Post(bareURL)
	})
}
