package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/corpix/uarand"
	"github.com/gocolly/colly/v2"
	"github.com/rix4uni/urlflux/banner"
	"github.com/spf13/pflag"
)

// Thread safe map
var sm sync.Map

type paramSet struct {
	mu     sync.Mutex
	params map[string]struct{}
}

var paramFinderMap sync.Map

func main() {
	concurrent := pflag.IntP("concurrency", "c", 10, "Number of concurrent goroutines.")
	parallelism := pflag.IntP("parallelism", "p", 10, "Number of concurrent inputs to process.")
	depth := pflag.Int("depth", 3, "Depth to crawl.")
	insecure := pflag.Bool("insecure", false, "Disable TLS verification.")
	subsInScope := pflag.Bool("include-subdomains", false, "Include subdomains for crawling.")
	proxy := pflag.String("proxy", "", "Proxy URL. E.g. --proxy http://127.0.0.1:8080")
	maxtime := pflag.Int("maxtime", -1, "Maximum time to crawl each URL from stdin, in seconds.")
	disableRedirects := pflag.Bool("disable-redirects", false, "Disable following redirects")
	timeout := pflag.Int("timeout", 30, "HTTP request timeout duration (in seconds)")
	silent := pflag.Bool("silent", false, "silent mode.")
	version := pflag.Bool("version", false, "Print the version of the tool and exit.")
	verbose := pflag.Bool("verbose", false, "enable verbose mode")

	pflag.Parse()

	if *version {
		banner.PrintBanner()
		banner.PrintVersion()
		os.Exit(0)
	}

	if !*silent {
		banner.PrintBanner()
	}

	var proxyURL *url.URL
	if *proxy != "" {
		os.Setenv("PROXY", *proxy)
	}
	if envProxy := os.Getenv("PROXY"); envProxy != "" {
		proxyURL, _ = url.Parse(envProxy)
	}

	// Check for stdin input
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		fmt.Fprintln(os.Stderr, "No urls detected. Hint: cat urls.txt | urlflux")
		os.Exit(1)
	}

	results := make(chan string, *concurrent)
	jobs := make(chan string)

	var wg sync.WaitGroup
	for i := 0; i < *parallelism; i++ {
		wg.Add(1)
		go worker(jobs, results, proxyURL, concurrent, depth, subsInScope, disableRedirects, insecure, timeout, maxtime, silent, verbose, &wg)
	}

	go func() {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			jobs <- s.Text()
		}
		if err := s.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	for res := range results {
		if isUnique(res) {
			fmt.Fprintln(w, res)
		}
	}
}

func worker(jobs <-chan string, results chan<- string, proxyURL *url.URL, concurrent, depth *int, subsInScope, disableRedirects, insecure *bool, timeout, maxtime *int, silent, verbose *bool, wg *sync.WaitGroup) {
	defer wg.Done()
	parseURL := url.Parse

	for input := range jobs {
		var urlsToTry []string
		if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
			urlsToTry = []string{input}
		} else {
			urlsToTry = []string{"https://" + input, "http://" + input, "https://www." + input, "http://www." + input}
		}

		success := false
	urlsLoop:
		for _, tryURL := range urlsToTry {
			hostname, err := extractHostname(tryURL)
			if err != nil {
				continue
			}

			// Instantiate default collector
			allowedHosts := []string{hostname}
			if strings.HasPrefix(hostname, "www.") {
				allowedHosts = append(allowedHosts, strings.TrimPrefix(hostname, "www."))
			} else {
				allowedHosts = append(allowedHosts, "www."+hostname)
			}
			c := colly.NewCollector(
				// limit crawling to the domain of the specified URL
				colly.AllowedDomains(allowedHosts...),
				// set MaxDepth to the specified depth
				colly.MaxDepth(*depth),
				// specify Async for threading
				colly.Async(true),
			)

			c.UserAgent = uarand.GetRandom()

			// if --subdomains is present, use regex to filter out subdomains in scope.
			if *subsInScope {
				c.AllowedDomains = nil
				c.URLFilters = []*regexp.Regexp{regexp.MustCompile(".*(\\.|\\/\\/)" + strings.ReplaceAll(hostname, ".", "\\.") + "((#|\\/|\\?).*)?")}
			}

			// If `-dr` flag provided, do not follow HTTP redirects.
			if *disableRedirects {
				c.SetRedirectHandler(func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				})
			}
			// Set parallelism
			c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: *concurrent})

			// Print every href found, and visit it
			c.OnHTML("a[href]", func(e *colly.HTMLElement) {
				link := e.Attr("href")
				result := e.Request.AbsoluteURL(link)
				if result != "" {
					defer func() {
						if err := recover(); err != nil {
							return
						}
					}()
					results <- result
				}
				e.Request.Visit(link)
			})

			// find and print all the JavaScript files
			c.OnHTML("script[src]", func(e *colly.HTMLElement) {
				result := e.Request.AbsoluteURL(e.Attr("src"))
				if result != "" {
					defer func() {
						if err := recover(); err != nil {
							return
						}
					}()
					results <- result
				}
			})

			// find and print all the form action URLs
			c.OnHTML("form[action]", func(e *colly.HTMLElement) {
				result := e.Request.AbsoluteURL(e.Attr("action"))
				if result != "" {
					defer func() {
						if err := recover(); err != nil {
							return
						}
					}()
					results <- result
				}
			})

			if proxyURL != nil && proxyURL.Host != "" {
				// Skip TLS verification for proxy, if -insecure specified
				c.WithTransport(&http.Transport{
					Proxy:           http.ProxyURL(proxyURL),
					TLSClientConfig: &tls.Config{InsecureSkipVerify: *insecure},
				})
			} else {
				// Skip TLS verification if -insecure flag is present
				c.WithTransport(&http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: *insecure},
				})
			}

			c.SetRequestTimeout(time.Duration(*timeout) * time.Second)

			// Extract input/textarea parameters for paramfinder-style output
			c.OnHTML("input[name], textarea[name]", func(e *colly.HTMLElement) {
				name := e.Attr("name")
				if name == "" {
					return
				}
				urlStr := e.Request.URL.String()
				val, _ := paramFinderMap.LoadOrStore(urlStr, &paramSet{params: make(map[string]struct{})})
				ps := val.(*paramSet)
				ps.mu.Lock()
				ps.params[name] = struct{}{}
				ps.mu.Unlock()
			})

			// Output paramfinder-style URLs after page is fully scraped
			c.OnScraped(func(r *colly.Response) {
				urlStr := r.Request.URL.String()
				val, ok := paramFinderMap.LoadAndDelete(urlStr)
				if !ok {
					return
				}
				ps := val.(*paramSet)
				ps.mu.Lock()
				params := make([]string, 0, len(ps.params))
				for name := range ps.params {
					params = append(params, name+"=rix4uni")
				}
				ps.mu.Unlock()

				if len(params) == 0 {
					return
				}

				parsedURL, err := parseURL(urlStr)
				if err != nil {
					return
				}

				parsedURL.RawQuery = strings.Join(params, "&")
				result := parsedURL.String()

				if result == urlStr {
					return
				}

				// If maxtime occurs before goroutines are finished, recover from panic
				defer func() {
					if err := recover(); err != nil {
						return
					}
				}()
				results <- result
			})

			gotResponse := false
			c.OnResponse(func(r *colly.Response) {
				gotResponse = true
			})

			if *verbose {
				c.OnError(func(r *colly.Response, err error) {
					if r != nil && r.Request != nil {
						log.Println("Error on", r.Request.URL, ":", err)
					} else {
						log.Println("Error:", err)
					}
				})
			}

			if *maxtime == -1 {
				// Start scraping
				c.Visit(tryURL)
				// Wait until threads are finished
				c.Wait()
			} else {
				finished := make(chan int, 1)

				go func() {
					// Start scraping
					c.Visit(tryURL)
					// Wait until threads are finished
					c.Wait()
					finished <- 0
				}()

				select {
				case _ = <-finished: // the crawling finished before the maxtime
					close(finished)
				case <-time.After(time.Duration(*maxtime) * time.Second): // maxtime reached
					if *verbose {
						log.Println("[maxtime] " + tryURL)
					}
					success = true
					break urlsLoop
				}
			}

			if gotResponse {
				success = true
				break
			}
		}

		if !success && *verbose {
			log.Println("Failed to crawl:", input)
		}
	}
}

// extractHostname() extracts the hostname from a URL or bare domain and returns it
func extractHostname(urlString string) (string, error) {
	if !strings.HasPrefix(urlString, "http://") && !strings.HasPrefix(urlString, "https://") {
		urlString = "https://" + urlString
	}
	u, err := url.Parse(urlString)
	if err != nil || u.Hostname() == "" {
		return "", errors.New("Input must be a valid URL or domain")
	}

	return u.Hostname(), nil
}

// returns whether the supplied url is unique or not
func isUnique(url string) bool {
	_, present := sm.Load(url)
	if present {
		return false
	}
	sm.Store(url, true)
	return true
}
