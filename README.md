## urlflux

A fast web crawler for gathering URLs, JavaScript files, and hidden parameters.

## Installation
```
go install github.com/rix4uni/urlflux@latest
```

## Download prebuilt binaries
```
wget https://github.com/rix4uni/urlflux/releases/download/v0.0.1/urlflux-linux-amd64-0.0.1.tgz
tar -xvzf urlflux-linux-amd64-0.0.1.tgz
rm -rf urlflux-linux-amd64-0.0.1.tgz
mv urlflux ~/go/bin/urlflux
```
Or download [binary release](https://github.com/rix4uni/urlflux/releases) for your platform.

## Compile from source
```
git clone --depth 1 https://github.com/rix4uni/urlflux.git
cd urlflux; go install
```

## Usage
```console
  -c, --concurrency int      Number of concurrent goroutines. (default 10)
      --depth int            Depth to crawl. (default 3)
      --disable-redirects    Disable following redirects
      --include-subdomains   Include subdomains for crawling.
      --insecure             Disable TLS verification.
      --maxtime int          Maximum time to crawl each URL from stdin, in seconds. (default -1)
  -p, --parallelism int      Number of concurrent inputs to process. (default 10)
      --proxy string         Proxy URL. E.g. --proxy http://127.0.0.1:8080
      --silent               silent mode.
      --timeout int          HTTP request timeout duration (in seconds) (default 30)
      --verbose              enable verbose mode
      --version              Print the version of the tool and exit.
```

**Features:**
- **Auto protocol fallback** — bare domains like `example.com` automatically try `https://` first, then fall back to `http://`
- **Random user-agent** — every request uses a random realistic user-agent string
- **Built-in parameter extraction** — automatically discovers input/textarea names and appends them as `name=rix4uni`
- **Deduplicated output** — all URLs are deduplicated automatically
- **Clean stdout** — errors and warnings only appear with `--verbose`, so output is safe to pipe

## Example usages

Single URL:
```console
echo "google.com" | urlflux
```

Multiple URLs:
```console
cat subs.txt | urlflux
```

subs.txt contains:
```console
example.com
https://testsite.com
test.example.com
```

Send all requests through a proxy:
```console
cat subs.txt | urlflux --proxy http://localhost:8080
```

Include subdomains:
```console
echo "example.com" | urlflux --include-subdomains
```

Set crawl depth:
```console
echo "example.com" | urlflux --depth 2
```

Set max crawl time per URL:
```console
cat subs.txt | urlflux --maxtime 10
```

Save results to a file:
```console
cat subs.txt | urlflux > output.txt
```

Show errors:
```console
echo "example.com" | urlflux --verbose
```
