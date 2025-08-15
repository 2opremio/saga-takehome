package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/quic-go/quic-go/http3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc"

	"github.com/2opremio/sagatakehome/2/client"
	"github.com/2opremio/sagatakehome/2/config"
	"github.com/2opremio/sagatakehome/2/proto"
	"github.com/2opremio/sagatakehome/2/server"
)

func getArtifactDirPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return path.Join(path.Dir(filename), "artifacts")
}

func TestHTTP(t *testing.T) {
	s := httptest.NewUnstartedServer(server.NewHTTPStringHandler())
	s.EnableHTTP2 = true
	s.Start()
	defer s.Close()
	c, err := client.New(client.Config{
		Endpoint:   s.URL,
		Approach:   config.HTTPApproach,
		HTTPClient: nil,
	})
	require.NoError(t, err)
	localVal := uint64(0)
	for i := 0; i < 10; i++ {
		v, err := c.BumpCounter(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, localVal+1, v)
		localVal = v
	}
}

func TestFastHTTP(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	s := &fasthttp.Server{
		Handler: server.NewHTTPStringHandler().HandleFastHTTP,
	}
	ln, err := net.Listen("tcp4", "localhost:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		defer wg.Done()
		err := s.Serve(ln)
		require.NoError(t, err)
	}()
	defer func() {
		err := s.Shutdown()
		require.NoError(t, err)
	}()
	c, err := client.New(client.Config{
		Endpoint:   fmt.Sprintf("localhost:%d", port),
		Approach:   config.FastHTTPApproach,
		HTTPClient: nil,
	})
	require.NoError(t, err)
	localVal := uint64(0)
	for i := 0; i < 10; i++ {
		v, err := c.BumpCounter(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, localVal+1, v)
		localVal = v
	}
	wg.Wait()
}

func TestQuic(t *testing.T) {
	s := http3.Server{
		Addr:    "127.0.0.1:4040",
		Port:    4040,
		Handler: server.NewHTTPStringHandler(),
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.ListenAndServeTLS(
			path.Join(getArtifactDirPath(), "cert.pem"),
			path.Join(getArtifactDirPath(), "priv.key"))
		if err != http.ErrServerClosed {
			require.NoError(t, err)
		}
	}()
	defer func() {
		err := s.Close()
		require.NoError(t, err)
	}()
	roundTripper := &http3.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	defer roundTripper.Close()
	qClient := &http.Client{
		Transport: roundTripper,
	}
	c, err := client.New(client.Config{
		Endpoint:   "https://localhost:4040",
		Approach:   config.HTTPApproach,
		HTTPClient: qClient,
	})
	require.NoError(t, err)
	localVal := uint64(0)
	for i := 0; i < 10; i++ {
		v, err := c.BumpCounter(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, localVal+1, v)
		localVal = v
	}
	wg.Wait()
}

func TestGRPC(t *testing.T) {
	ln, err := net.Listen("tcp4", "localhost:0")
	require.NoError(t, err)
	s := grpc.NewServer()
	require.NoError(t, err)
	proto.RegisterCounterServer(s, server.NewGRPCServer())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.Serve(ln)
		require.NoError(t, err)
	}()
	defer func() {
		s.Stop()
		wg.Wait()
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	c, err := client.New(client.Config{
		Endpoint: fmt.Sprintf("localhost:%d", port),
		Approach: config.GRPCApproach,
	})
	require.NoError(t, err)
	localVal := uint64(0)
	for i := 0; i < 10; i++ {
		v, err := c.BumpCounter(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, localVal+1, v)
		localVal = v
	}
}

func runBench(b *testing.B, f func()) {
	b.ReportAllocs()
	totalOps := 0
	start := time.Now()
	for b.Loop() {
		f()
		totalOps++
	}
	elapsed := time.Since(start)
	b.ReportMetric(0, "ns/op")
	b.ReportMetric(float64(totalOps)/elapsed.Minutes(), "req/min")
	b.ReportMetric(float64(totalOps), "req")
}

func runParallelBench(b *testing.B, cfg client.Config, f func(c *client.Client)) {
	b.ReportAllocs()
	var totalOps atomic.Uint64
	start := time.Now()
	b.RunParallel(func(pb *testing.PB) {
		c, err := client.New(cfg)
		require.NoError(b, err)
		for pb.Next() {
			f(c)
			totalOps.Add(1)
		}
	})
	elapsed := time.Since(start)
	b.ReportMetric(0, "ns/op")
	b.ReportMetric(float64(totalOps.Load())/elapsed.Minutes(), "req/min")
	b.ReportMetric(float64(totalOps.Load()), "req")
}

func BenchmarkHTTP_NoKeepAlive(b *testing.B) {
	s := httptest.NewUnstartedServer(server.NewHTTPStringHandler())
	s.EnableHTTP2 = true
	s.Start()
	defer s.Close()
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DisableKeepAlives = true
	httpC := &http.Client{Transport: t}
	cfg := client.Config{
		Endpoint:   s.URL,
		Approach:   config.HTTPApproach,
		HTTPClient: httpC,
	}
	c, err := client.New(cfg)
	require.NoError(b, err)
	b.Run("Sequential", func(b *testing.B) {
		runBench(b, func() {
			_, err := c.BumpCounter(context.Background(), 1)
			if err != nil {
				b.Fatal(err)
			}
		})
	})
	b.Run("Parallel", func(b *testing.B) {
		runParallelBench(b, cfg, func(c *client.Client) {
			_, err := c.BumpCounter(context.Background(), 1)
			if err != nil {
				b.Fatal(err)
			}
		})
	})
}

// The Go client uses a connection pool by default so this counts as a client pool usecase
func BenchmarkHTTP(b *testing.B) {
	s := httptest.NewUnstartedServer(server.NewHTTPStringHandler())
	s.EnableHTTP2 = true
	s.Start()
	defer s.Close()
	cfg := client.Config{
		Endpoint: s.URL,
		Approach: config.HTTPApproach,
	}
	c, err := client.New(cfg)
	require.NoError(b, err)
	b.Run("Sequential", func(b *testing.B) {
		runBench(b, func() {
			_, err := c.BumpCounter(context.Background(), 1)
			if err != nil {
				b.Fatal(err)
			}
		})
	})
	b.Run("Parallel", func(b *testing.B) {
		runParallelBench(b, cfg, func(c *client.Client) {
			_, err := c.BumpCounter(context.Background(), 1)
			if err != nil {
				b.Fatal(err)
			}
		})
	})
}

func BenchmarkHTTP_TLS_NoKeepAlive(b *testing.B) {
	s := httptest.NewUnstartedServer(server.NewHTTPStringHandler())
	s.EnableHTTP2 = true
	s.StartTLS()
	defer s.Close()
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	t.DisableKeepAlives = true
	httpC := &http.Client{Transport: t}
	c, err := client.New(client.Config{
		Endpoint:   s.URL,
		Approach:   config.HTTPApproach,
		HTTPClient: httpC,
	})
	require.NoError(b, err)
	runBench(b, func() {
		_, err := c.BumpCounter(context.Background(), 1)
		if err != nil {
			b.Fatal(err)
		}
	})
}

func BenchmarkHTTP_TLS(b *testing.B) {
	s := httptest.NewUnstartedServer(server.NewHTTPStringHandler())
	s.EnableHTTP2 = true
	s.StartTLS()
	defer s.Close()
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	httpC := &http.Client{Transport: t}
	c, err := client.New(client.Config{
		Endpoint:   s.URL,
		Approach:   config.HTTPApproach,
		HTTPClient: httpC,
	})
	require.NoError(b, err)
	runBench(b, func() {
		_, err := c.BumpCounter(context.Background(), 1)
		if err != nil {
			b.Fatal(err)
		}
	})
}

func BenchmarkFastHTTP_NoKeepAlive(b *testing.B) {
	var wg sync.WaitGroup
	wg.Add(1)
	s := &fasthttp.Server{
		Handler: server.NewHTTPStringHandler().HandleFastHTTP,
	}
	ln, err := net.Listen("tcp4", "localhost:4040")
	require.NoError(b, err)
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		defer wg.Done()
		err := s.Serve(ln)
		require.NoError(b, err)
	}()
	defer func() {
		err := s.Shutdown()
		require.NoError(b, err)
		wg.Wait()
	}()
	cfg := client.Config{
		Endpoint:                 fmt.Sprintf("http://localhost:%d", port),
		Approach:                 config.FastHTTPApproach,
		DisableFastHTTPKeepAlive: true,
		FastHTTPClient: &fasthttp.Client{
			MaxConnsPerHost: 50,
		},
	}
	c, err := client.New(cfg)
	require.NoError(b, err)

	b.Run("Sequential", func(b *testing.B) {
		runBench(b, func() {
			_, err := c.BumpCounter(context.Background(), 1)
			if err != nil {
				b.Fatal(err)
			}
		})
	})
	b.Run("Parallel", func(b *testing.B) {
		runParallelBench(b, cfg, func(c *client.Client) {
			_, err := c.BumpCounter(context.Background(), 1)
			if err != nil {
				b.Fatal(err)
			}
		})
	})
}

func BenchmarkFastHTTP(b *testing.B) {
	var wg sync.WaitGroup
	wg.Add(1)
	s := &fasthttp.Server{
		Handler:       server.NewHTTPStringHandler().HandleFastHTTP,
		MaxConnsPerIP: 50,
	}
	ln, err := net.Listen("tcp4", "localhost:0")
	require.NoError(b, err)
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		defer wg.Done()
		err := s.Serve(ln)
		require.NoError(b, err)
	}()
	defer func() {
		err := s.Shutdown()
		require.NoError(b, err)
		wg.Wait()
	}()
	// Give enough time for the server to start
	cfg := client.Config{
		Endpoint: fmt.Sprintf("http://localhost:%d", port),
		Approach: config.FastHTTPApproach,
		FastHTTPClient: &fasthttp.Client{
			MaxConnsPerHost: 50,
		},
	}
	c, err := client.New(cfg)
	require.NoError(b, err)

	b.Run("Sequential", func(b *testing.B) {
		runBench(b, func() {
			_, err := c.BumpCounter(context.Background(), 1)
			if err != nil {
				b.Fatal(err)
			}
		})
	})
	b.Run("Parallel", func(b *testing.B) {
		runParallelBench(b, cfg, func(c *client.Client) {
			_, err := c.BumpCounter(context.Background(), 1)
			if err != nil {
				b.Fatal(err)
			}
		})
	})
}

func BenchmarkFastHTTP_TLS_NoKeepAlive(b *testing.B) {
	var wg sync.WaitGroup
	wg.Add(1)
	s := &fasthttp.Server{
		Handler: server.NewHTTPStringHandler().HandleFastHTTP,
	}
	ln, err := net.Listen("tcp4", "localhost:4040")
	require.NoError(b, err)
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		defer wg.Done()
		err := s.ServeTLS(
			ln,
			path.Join(getArtifactDirPath(), "cert.pem"),
			path.Join(getArtifactDirPath(), "priv.key"),
		)
		require.NoError(b, err)
	}()
	defer func() {
		err := s.Shutdown()
		require.NoError(b, err)
		wg.Wait()
	}()
	c, err := client.New(client.Config{
		Endpoint: fmt.Sprintf("https://localhost:%d", port),
		Approach: config.FastHTTPApproach,
		FastHTTPClient: &fasthttp.Client{
			TLSConfig:       &tls.Config{InsecureSkipVerify: true},
			MaxConnsPerHost: 50,
		},
		DisableFastHTTPKeepAlive: true,
	})
	require.NoError(b, err)

	runBench(b, func() {
		_, err := c.BumpCounter(context.Background(), 1)
		if err != nil {
			b.Fatal(err)
		}
	})
}

func BenchmarkFastHTTP_TLS(b *testing.B) {
	var wg sync.WaitGroup
	wg.Add(1)
	s := &fasthttp.Server{
		Handler: server.NewHTTPStringHandler().HandleFastHTTP,
	}
	ln, err := net.Listen("tcp4", "localhost:0")
	require.NoError(b, err)
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		defer wg.Done()
		err := s.ServeTLS(
			ln,
			path.Join(getArtifactDirPath(), "cert.pem"),
			path.Join(getArtifactDirPath(), "priv.key"),
		)
		require.NoError(b, err)
	}()
	defer func() {
		err := s.Shutdown()
		require.NoError(b, err)
		wg.Wait()
	}()
	c, err := client.New(client.Config{
		Endpoint: fmt.Sprintf("https://localhost:%d", port),
		Approach: config.FastHTTPApproach,
		FastHTTPClient: &fasthttp.Client{
			TLSConfig:       &tls.Config{InsecureSkipVerify: true},
			MaxConnsPerHost: 50,
		},
	})
	require.NoError(b, err)

	runBench(b, func() {
		_, err := c.BumpCounter(context.Background(), 1)
		if err != nil {
			b.Fatal(err)
		}
	})
}

func BenchmarkQUIC(b *testing.B) {
	s := http3.Server{
		Addr:    "127.0.0.1:4040",
		Port:    4040,
		Handler: server.NewHTTPStringHandler(),
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.ListenAndServeTLS(
			path.Join(getArtifactDirPath(), "cert.pem"),
			path.Join(getArtifactDirPath(), "priv.key"))
		if err != http.ErrServerClosed {
			require.NoError(b, err)
		}
	}()
	defer func() {
		err := s.Close()
		require.NoError(b, err)
		wg.Wait()
	}()
	roundTripper := &http3.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	defer roundTripper.Close()
	qClient := &http.Client{
		Transport: roundTripper,
	}
	cfg := client.Config{
		Endpoint:   "https://localhost:4040",
		Approach:   config.HTTPApproach,
		HTTPClient: qClient,
	}
	c, err := client.New(cfg)
	require.NoError(b, err)
	b.Run("Sequential", func(b *testing.B) {
		runBench(b, func() {
			_, err := c.BumpCounter(context.Background(), 1)
			if err != nil {
				b.Fatal(err)
			}
		})
	})
	b.Run("Parallel", func(b *testing.B) {
		runParallelBench(b, cfg, func(c *client.Client) {
			_, err := c.BumpCounter(context.Background(), 1)
			if err != nil {
				b.Fatal(err)
			}
		})
	})
}

func BenchmarkGRPC(b *testing.B) {
	ln, err := net.Listen("tcp4", "localhost:0")
	require.NoError(b, err)
	s := grpc.NewServer()
	require.NoError(b, err)
	proto.RegisterCounterServer(s, server.NewGRPCServer())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.Serve(ln)
		require.NoError(b, err)
	}()
	defer func() {
		s.Stop()
		wg.Wait()
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	cfg := client.Config{
		Endpoint: fmt.Sprintf("localhost:%d", port),
		Approach: config.GRPCApproach,
	}
	c, err := client.New(cfg)
	require.NoError(b, err)
	b.Run("Sequential", func(b *testing.B) {
		runBench(b, func() {
			_, err := c.BumpCounter(context.Background(), 1)
			if err != nil {
				b.Fatal(err)
			}
		})
	})
	b.Run("Parallel", func(b *testing.B) {
		runParallelBench(b, cfg, func(c *client.Client) {
			_, err := c.BumpCounter(context.Background(), 1)
			if err != nil {
				b.Fatal(err)
			}
		})
	})
}
