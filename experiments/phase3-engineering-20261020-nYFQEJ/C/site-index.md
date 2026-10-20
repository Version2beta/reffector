# Relevant analyzer sites

Source: writeheader-callers.json; all use net/http.(ResponseWriter).WriteHeader with interface_dispatch uncertainty.

- `site:5ac367205c3144399d887d69a4d5b0734a30860852136313886dc43a73300703` — `(*Server).ServeHTTP` at `/Users/rob/Development/reffector/experiments/caddy/modules/caddyhttp/server.go:823`
- `site:6421ff2efa29eb1e359d225ef680e568e52bdd6ec3d097542669619e947787a3` — `(*Server).ServeHTTP` at `/Users/rob/Development/reffector/experiments/caddy/modules/caddyhttp/server.go:825`
- `site:91e32c4b0408404835375db8029e3e64fcf5312e53cda55157fb5e353827fa82` — `(*Server).ServeHTTP` at `/Users/rob/Development/reffector/experiments/caddy/modules/caddyhttp/server.go:842`
- `site:afda971af6b315c6470be2e69021b7d506d9cab2b784476fc4ca42e3f131a50a` — `(*Handler).finalizeResponse` at `/Users/rob/Development/reffector/experiments/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:1452`
- `site:3c867c8fb3c996837d209296ad570c2337bfa4bc3818a30cfd2f7cb1895b30bd` — `(*Handler).handleUpgradeResponse` at `/Users/rob/Development/reffector/experiments/caddy/modules/caddyhttp/reverseproxy/streaming.go:119`
- `site:dcf640c7bd2d3d15026a165f7f9a2597766f7a5fb1e34ff89666563fc04d7eb3` — `(*Handler).handleUpgradeResponse` at `/Users/rob/Development/reffector/experiments/caddy/modules/caddyhttp/reverseproxy/streaming.go:149`
- `site:3b613437fc37adebd378c4832cc31b02c7cd65f55f00d7f9e2477b20b1b4f9f3` — `(*Handler).proxyLoopIteration` at `/Users/rob/Development/reffector/experiments/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:852`
- `site:63070b79b14c01fb5390068602aec185d5262b0e8155a598eca63b237e972aae` — `(*Handler).reverseProxy$2` at `/Users/rob/Development/reffector/experiments/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:1215`
- `site:d78e9c7941411d64be89f01d63efe276f19ced962d8bfc5aa1b6e78e455080e9` — `(*Server).Upgrade` at `/Users/rob/go/pkg/mod/github.com/quic-go/webtransport-go@v0.13.0/server.go:467`
