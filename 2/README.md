# Coding and Optimization Challenge

Instead of coding an independent client/server `main()` function, I created benchmarks for different implementation alternatives.

The core part of the server itself is a simple in-memory counter using Golang's `atomic` library.
This allows for maximum concurrency without any lock contention.

On top of that I created benchmarks (see `main_test.go` with different transports and serialization formats)
trying to simulate different situations. The reason for this is that the challenge statement is too
simplistic and doesn't indicate where the server would be used 
(Internally, in a backend system without encryption? With many requests from a single client?)

Here are the benchmarks I created:

1. HTTP 2 using the standard Golang library.
   * With and without TCP Keepalive (removing keepalive _simulates_ separate clients
     by closing the connection after every request).
   * With and without TLS.
   * Using sequential and parallel (using one client per core) benchmarks
   * Using a textual base-10 string representation for requests and responses.
2. HTTP using https://github.com/valyala/fasthttp
    * With and without TCP Keepalive (removing keepalive _simulates_ separate clients
      by closing the connection after every request).
    * With and without TLS.
    * Using sequential and parallel (using one client per core) benchmarks.
    * Using a textual base-10 string representation for requests and responses.
3. QUIC using https://github.com/quic-go/quic-go . The idea here is to optimize the connection
   establishment. QUIC uses UDP as a transport and is known to result in much faster connections.
   My hypothesis is that, with such a simple endpoint, the connection establishment would dominate
   the time spent in the remote call (compared to the serving time).
    * Always with TLS (which is mandatory in QUIC and is what prompted me to use TLS in the other
      benchmarks for comparison)
    * Using sequential and parallel (using one client per core) benchmarks.
    * Using a textual base-10 string representation for requests and responses.
4. GRPC + Protobuf representation using `protoc` and  https://github.com/grpc/grpc-go.
   I wasn't sure if this would really help, since the amount of data sent is small and simple
   and TCP is used under the hood.
   (i.e. the marhsaling and unmarshalling of textual data may not )
    * Without TLS (I just didn't have time for more)


The benchmarks were ran with both the client and server running in the same machine (through the loopback interface)
of my Mac Studio Ultra M1 using MacOS 10.15.6 .

If had more time, I would had tested clients and servers in separate machines since the loopback interface is known to
be optimized (the OS can make use of faster paths not available when interacting between different machines) and doesn't
suffer from the same connection establishment latency problems. 

Here are the benchmark results:

```
goos: darwin
goarch: arm64
pkg: github.com/2opremio/sagatakehome/2
cpu: Apple M1 Ultra
BenchmarkHTTP_NoKeepAlive
BenchmarkHTTP_NoKeepAlive/Sequential
BenchmarkHTTP_NoKeepAlive/Sequential-20 	    9426	      9426 req	    472458 req/min	   20176 B/op	     138 allocs/op
BenchmarkHTTP_NoKeepAlive/Parallel
BenchmarkHTTP_NoKeepAlive/Parallel-20   	   10000	     10000 req	    518970 req/min	   41613 B/op	     147 allocs/op
BenchmarkHTTP
BenchmarkHTTP/Sequential
BenchmarkHTTP/Sequential-20             	   24808	     24808 req	   1269313 req/min	    7478 B/op	      77 allocs/op
BenchmarkHTTP/Parallel
BenchmarkHTTP/Parallel-20               	   15408	     15408 req	    793509 req/min	   27955 B/op	     101 allocs/op
BenchmarkHTTP_TLS_NoKeepAlive
BenchmarkHTTP_TLS_NoKeepAlive-20        	     664	       664.0 req	     33393 req/min	  149274 B/op	    1186 allocs/op
BenchmarkHTTP_TLS
BenchmarkHTTP_TLS-20                    	   16598	     16598 req	    835427 req/min	    9940 B/op	      84 allocs/op
BenchmarkFastHTTP_NoKeepAlive
BenchmarkFastHTTP_NoKeepAlive/Sequential
BenchmarkFastHTTP_NoKeepAlive/Sequential-20         	   12247	     12247 req	    612997 req/min	    2761 B/op	      43 allocs/op
BenchmarkFastHTTP_NoKeepAlive/Parallel
BenchmarkFastHTTP_NoKeepAlive/Parallel-20         	   21837	     21837 req	    972925 req/min	    5738 B/op	      49 allocs/op
BenchmarkFastHTTP
BenchmarkFastHTTP/Sequential
BenchmarkFastHTTP/Sequential-20                     	   48274	     48274 req	   2493138 req/min	     958 B/op	      13 allocs/op
BenchmarkFastHTTP/Parallel
BenchmarkFastHTTP/Parallel-20                       	   44168	     44168 req	   2200873 req/min	     971 B/op	      13 allocs/op
BenchmarkFastHTTP_TLS_NoKeepAlive
BenchmarkFastHTTP_TLS_NoKeepAlive-20                	     738	       738.0 req	     35064 req/min	  111294 B/op	     900 allocs/op
BenchmarkFastHTTP_TLS
BenchmarkFastHTTP_TLS-20                            	   44335	     44335 req	   2349103 req/min	    1011 B/op	      15 allocs/op
BenchmarkQUIC
BenchmarkQUIC/Sequential
BenchmarkQUIC/Sequential-20                         	   11720	     11720 req	    589169 req/min	   24558 B/op	     258 allocs/op
BenchmarkQUIC/Parallel
BenchmarkQUIC/Parallel-20                           	   20024	     20024 req	    998333 req/min	   26568 B/op	     230 allocs/op
BenchmarkGRPC
BenchmarkGRPC/Sequential
BenchmarkGRPC/Sequential-20                         	   16844	     16844 req	    849226 req/min	    8880 B/op	     168 allocs/op
BenchmarkGRPC/Parallel
BenchmarkGRPC/Parallel-20                           	   26606	     26606 req	   1399944 req/min	    9002 B/op	     168 allocs/op
FAIL
PASS
```

The clear winner is `fasthttp` with 2.5 million req/min, it's surprising to see that:

1. It beat GRPC, even the parallel version, at 1.4 million req/min. This is probably
   due to the lower GC pressure (`allocs/op` in the result). Even with that, it is
   still surprising that a framework using code-generated encoders/decoders didn't
   perform better.
2. The parallel `fasthttp` benchmark has lower performance. I suspected this was
   due to `fasthttp` using parallelism by default (and a quick look at the code
   seems to confirm it)
3. QUIC didn't perform better, my guess is that it doesn't use Keepalive by default,
   although I didn't have time to dig deeper into it.
  

It's worth noting that `fasthttp` achieves those 2.5 million req/min when using Keepalive (i.e. persistent connections).
When not using keepalive, the connection is closed after each request (which serves to simulate disjoint clients) bringing 
its the values much closer to the other contenders.

Note: If I had more time I would had cleaned up the `main_test.go` file, which has a looot of replicated code at this point
(it grew out of hand when adding more and more transports).


