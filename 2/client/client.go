package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/2opremio/sagatakehome/2/config"
	"github.com/2opremio/sagatakehome/2/proto"
)

type Client struct {
	c          Config
	grpcClient proto.CounterClient
}

type Config struct {
	Endpoint                 string
	Approach                 config.Approach
	HTTPClient               *http.Client
	FastHTTPClient           *fasthttp.Client
	DisableFastHTTPKeepAlive bool
}

func New(c Config) (*Client, error) {
	result := &Client{
		c: c,
	}
	if c.Approach == config.GRPCApproach {
		dialOption := grpc.WithTransportCredentials(insecure.NewCredentials())
		conn, err := grpc.NewClient(c.Endpoint, dialOption)
		if err != nil {
			return nil, err
		}
		result.grpcClient = proto.NewCounterClient(conn)
	}
	return result, nil
}

func (c *Client) BumpCounter(ctx context.Context, by uint64) (uint64, error) {
	switch c.c.Approach {
	case config.HTTPApproach:
		return c.bumpCounterHTTP(ctx, by)
	case config.FastHTTPApproach:
		return c.bumpCounterFastHTTP(ctx, by)
	case config.GRPCApproach:
		req := &proto.BumpRequest{By: by}
		reply, err := c.grpcClient.Bump(ctx, req)
		if err != nil {
			return 0, fmt.Errorf("failed to send request: %w", err)
		}
		return reply.Counter, nil
	default:
		return 0, fmt.Errorf("unsupported approach: %d", c.c.Approach)
	}
}

func (c *Client) bumpCounterHTTP(ctx context.Context, by uint64) (uint64, error) {
	byReader := strings.NewReader(strconv.FormatUint(by, 10))
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, c.c.Endpoint+"/counter", byReader)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	httpC := http.DefaultClient
	if c.c.HTTPClient != nil {
		httpC = c.c.HTTPClient
	}
	resp, err := httpC.Do(r)
	if err != nil {
		return 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response body: %w", err)
	}
	ret, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse response body: %w", err)
	}
	return ret, nil
}

func (c *Client) bumpCounterFastHTTP(_ context.Context, by uint64) (uint64, error) {
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(c.c.Endpoint + "/counter")
	req.Header.SetMethod(fasthttp.MethodPost)
	if c.c.DisableFastHTTPKeepAlive {
		req.Header.Set("Connection", "close")
	}
	req.SetBodyRaw([]byte(strconv.FormatUint(by, 10)))

	resp := fasthttp.AcquireResponse()
	client := &fasthttp.Client{}
	if c.c.FastHTTPClient != nil {
		client = c.c.FastHTTPClient
	}
	err := client.Do(req, resp)
	fasthttp.ReleaseRequest(req)
	if err != nil {
		return 0, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	ret, err := strconv.ParseUint(strings.TrimSpace(string(resp.Body())), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse response body: %w", err)
	}
	return ret, nil
}
