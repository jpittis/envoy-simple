People often talk about Envoy in the context of Service Mesh, Kubernetes, and
Cloud Native consultants trying to sell you half working products with cute
mascots. I'll let you in on a secret: Envoy is just a multi-purpose L4/L7 proxy
like NGINX or HAProxy, and it certainly has nothing to do with Severless. Don't
take my word for it. Pull up your terminal, and I'll show you with the help of
a little bit of Go, some YAML, and docker-compose.

Let's create a new project, and write some Go.

```
$ mkdir envoy-simple
$ cd envoy-simple
$ touch main.go
```

We'll fill it with pretty well the simplest Go HTTP server money can't buy:

```
package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Success!\n")
	})
	log.Fatal(http.ListenAndServe("127.0.0.1:5555", nil))
}
```

Try it out with cURL!

```
$ go run main.go
```

```
$ curl 127.0.0.1:5555
Success!
```

The plan is to spin-up an Envoy process and configure it to proxy connections
to our HTTP server. The Envoy maintainers kindly provide an Envoy docker
container, so let's get docker-compose to set it up for us.

```
version: "3"
services:
  envoy:
    image: envoyproxy/envoy-alpine
    network_mode: host
    volumes:
      - ./envoy.yaml:/etc/envoy/envoy.yaml
```

We're using host networking to keep things simple. The Envoy process in the
container is pre-configured to read a YAML config file found at
`/etc/envoy/envoy.yaml`, so let's slap a new `envoy.yaml` file into our project
and get docker-compose to mount it into the container.

```
$ touch envoy.yaml
```

At this point if we try turning on docker-compose we'll see the Envoy process
try to read `/etc/envoy/envoy.yaml` but exit because the file is empty.

```
$ docker-compose up
...
envoy_1  | [2020-10-24 03:30:32.596][1][info][main] [source/server/server.cc:512] exiting
envoy_1  | Unable to convert YAML as JSON:
```

So let's fill it with the MVP of Envoy configs: an admin debug endpoint.

```
admin:
  access_log_path: /dev/null
  address:
    socket_address:
      address: 127.0.0.1
      port_value: 9901
```

Now when we turn on docker-compose, Envoy doesn't crash.

```
$ docker-compose up
...
envoy_1  | [2020-10-24 03:35:11.253][1][info][main] [source/server/server.cc:468] starting main dispatch loop
```

We can confirm the admin debug endpoint is working by visiting it in our
browser at `127.0.0.1:9901`. If you really want to have some fun, click the
"quitquitquit" button, I dare you!

Now that we've got ourselves a working Envoy, time to configure it as a TCP
proxy!

We're going to define some `static_resources` which are unchanging Envoy config
resources specified in a file and read once on boot (as opposed to dynamic
resources which Envoy can receive without needing to restart).

The first resource we'll specify is a `cluster`. Think of a cluster as a set of
endpoints which Envoy will load-balance across.

```
static_resources:
  clusters:
    - name: banana
      connect_timeout: 1s
      load_assignment:
        cluster_name: banana
        endpoints:
          lb_endpoints:
            - endpoint:
                address:
                  socket_address:
                    address: 127.0.0.1
                    port_value: 5555
```

We're calling this cluster `banana` and we only specify one endpoint: our Go
HTTP server. This cluster doesn't do anything on it's own, but we'll now tell
Envoy how to proxy traffic to it.

Let's define a second resource called a `listener` which listens (you guessed
it!) on the specified port and proxies traffic to our previously defined
cluster.

```
  listeners:
    - address:
        socket_address:
          address: 127.0.0.1
          port_value: 4444
      filter_chains:
        - filters:
          - name: envoy.tcp_proxy
            config:
              stat_prefix: banana
              cluster: banana
```

You can see we're listening on `127.0.0.1:4444`. `filters` and `filter_chains`
are how we tell Envoy what to do with connections to this listener. We're
telling Envoy to behave as a simple TCP proxy to `cluster: banana`.

Time to try out cURL again. First spin-up the Go HTTP server without
docker-compose. Obviously there's nothing listening on port 4444.

```
$ curl localhost:4444
curl: (7) Failed to connect to localhost port 4444: Connection refused
```

And now let's spin-up docker-compose and try a second time.

```
$ curl localhost:4444
Success!
```

We're now proxying traffic through Envoy!

Now for the last trick up my sleeve, let's enable access logging to stdout so
we can see the connections flowing through Envoy. You need to enable access
logging on the TCP proxy filter.

```
...
        - filters:
          - name: envoy.tcp_proxy
            config:
              stat_prefix: banana
              cluster: banana
              access_log:
                - name: envoy.file_access_log
                  config:
                    path: /dev/stdout
```

Now if we restart docker-compose, we'll see access logs being printed to
stdout. The format is a little cryptic. The last field is the upstream host,
and the `78 125` tuple are the bytes received and bytes sent.

```
envoy_1  | [2020-10-24T03:59:41.810Z] "- - -" 0 - 78 125 0 - "-" "-" "-" "-" "127.0.0.1:555
```

Hopefully you've dissociated Envoy from the usual hip buzzwords. Coolest of
all, you've now got a little playground to toy with Envoy! Edit that YAML file
to your heart's desire! We are YAML engineers after all.
