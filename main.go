package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type probeArgs struct {
	httpPorts  []string
	httpsPorts []string
}

func (p *probeArgs) Set(val string) error {
	// Adding port templates
	xxlarge := []string{"8401", "5080", "593", "7443", "4443", "5357", "81", "300", "591", "593", "832", "981", "1010", "1311", "2082", "2087", "2095", "2096", "2480", "3000", "3128", "3333", "4243", "4567", "4711", "4712", "4993", "5000", "5104", "5108", "5800", "6543", "7000", "7396", "7474", "8000", "8001", "8008", "8014", "8042", "8069", "8080", "8081", "8088", "8090", "8091", "8118", "8123", "8172", "8222", "8243", "8280", "8281", "8333", "8443", "8500", "8834", "8880", "8888", "8983", "9000", "9043", "9060", "9080", "9090", "9091", "9200", "9443", "9800", "9981", "12443", "16080", "18091", "18092", "20720", "28017", "280", "591", "777", "900", "2301", "2381", "2688", "2693", "2851", "3106", "3128", "5490", "5554", "6842", "8001", "8002", "8007", "8008", "8010", "8081", "8765", "311", "598", "620", "765", "901", "1085", "1188", "1559", "1739", "1772", "1963", "2314", "2339", "2403", "2534", "2784", "2837", "2907", "2929", "2930", "2972", "3011", "3012", "3334", "3342", "4801", "5987", "6588", "7001", "7002", "7015", "8208", "8383", "8893", "9090", "9280", "10000", "21845", "22555", "25867", "8083"}
	xlarge := []string{"81", "300", "591", "593", "832", "981", "1010", "1311", "2082", "2087", "2095", "2096", "2480", "3000", "3128", "3333", "4243", "4567", "4711", "4712", "4993", "5000", "5104", "5108", "5800", "6543", "7000", "7396", "7474", "8000", "8001", "8008", "8014", "8042", "8069", "8080", "8081", "8088", "8090", "8091", "8118", "8123", "8172", "8222", "8243", "8280", "8281", "8333", "8443", "8500", "8834", "8880", "8888", "8983", "9000", "9043", "9060", "9080", "9090", "9091", "9200", "9443", "9800", "9981", "12443", "16080", "18091", "18092", "20720", "28017"}
	large := []string{"81", "591", "2082", "2087", "2095", "2096", "3000", "8000", "8001", "8008", "8080", "8083", "8443", "8834", "8888"}
	small := []string{"7000", "7001", "8000", "8001", "8008", "8080", "8083", "8443", "8834", "8888", "10000"}

	pair := strings.SplitN(val, ":", 2)
	if len(pair) == 1 {
		// using `-p p1,p2...pn` format
		fields := strings.Split(pair[0], ",")
		for _, f := range fields {
			if f == "small" {
				p.httpPorts = append(p.httpPorts, small...)
				p.httpsPorts = append(p.httpsPorts, small...)
			} else if f == "large" {
				p.httpPorts = append(p.httpPorts, large...)
				p.httpsPorts = append(p.httpsPorts, large...)
			} else if f == "xlarge" {
				p.httpPorts = append(p.httpPorts, xlarge...)
				p.httpsPorts = append(p.httpsPorts, xlarge...)
			} else if f == "xxlarge" {
				p.httpPorts = append(p.httpPorts, xxlarge...)
				p.httpsPorts = append(p.httpsPorts, xxlarge...)
			} else {
				p.httpPorts = append(p.httpPorts, f)
				p.httpsPorts = append(p.httpsPorts, f)
			}
		}
	} else if len(pair) == 2 {
		// using `-p proto:p1,p2...pn` format
		proto := pair[0]
		fields := strings.Split(pair[1], ",")
		for _, f := range fields {
			if proto == "https" {
				if f == "small" {
					p.httpsPorts = append(p.httpsPorts, small...)
				} else if f == "large" {
					p.httpsPorts = append(p.httpsPorts, large...)
				} else if f == "xlarge" {
					p.httpsPorts = append(p.httpsPorts, xlarge...)
				} else if f == "xxlarge" {
					p.httpsPorts = append(p.httpsPorts, xxlarge...)
				} else {
					p.httpsPorts = append(p.httpsPorts, f)
				}
			} else {
				if f == "small" {
					p.httpPorts = append(p.httpPorts, small...)
				} else if f == "large" {
					p.httpPorts = append(p.httpPorts, large...)
				} else if f == "xlarge" {
					p.httpPorts = append(p.httpPorts, xlarge...)
				} else if f == "xxlarge" {
					p.httpPorts = append(p.httpPorts, xxlarge...)
				} else {
					p.httpPorts = append(p.httpPorts, f)
				}
			}
		}
	}

	return nil
}

func (p probeArgs) String() string {
	return strings.Join(append(p.httpPorts, p.httpsPorts...), ",")
}

func main() {

	// concurrency flag
	var concurrency int
	flag.IntVar(&concurrency, "c", 20, "set the concurrency level (split equally between HTTPS and HTTP requests)")

	// probe flags
	var probes probeArgs
	flag.Var(&probes, "p", "add additional probe (e.g. -p proto:port or -p <small|large|xlarge>)")

	// skip default probes flag
	var skipDefault bool
	flag.BoolVar(&skipDefault, "s", false, "skip the default probes (http:80 and https:443)")

	// timeout flag
	var to int
	flag.IntVar(&to, "t", 10000, "timeout (milliseconds)")

	// prefer https
	var preferHTTPS bool
	flag.BoolVar(&preferHTTPS, "prefer-https", false, "only try plain HTTP if HTTPS fails")

	// filter out cloudflare error pages
	var filterCloudflareErrors bool
	flag.BoolVar(&filterCloudflareErrors, "filter-cf-errors", false, "Filter out Cloudflare error pages")

	// HTTP method to use
	var method string
	flag.StringVar(&method, "method", "GET", "HTTP method to use")

	// HTTP User-Agent to use
	var userAgent string
	flag.StringVar(&userAgent, "A", "httprobe", "HTTP User-Agent to use")

	// http/socks proxy
	var proxyURI string
	flag.StringVar(&proxyURI, "x", "", "HTTP/SOCKS proxy to use")

	// follow redirects
	var followRedirects bool
	flag.BoolVar(&followRedirects, "follow-redirect", false, "follow redirects")

	flag.Parse()

	// make an actual time.Duration out of the timeout
	timeout := time.Duration(to * 1000000)

	var filterStrings []string

	// Add Cloudflare signatures to filterStrings if filterCloudflareErrors
	if filterCloudflareErrors {
		filterStrings = append(filterStrings, "<center>cloudflare</center>", "cf_styles-css")
	}

	var tr = &http.Transport{
		MaxIdleConns:      30,
		IdleConnTimeout:   time.Second,
		DisableKeepAlives: true,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: time.Second,
		}).DialContext,
	}

	if proxyURI != "" {
		PROXY_ADDR := proxyURI
		url_i := url.URL{}
		url_proxy, _ := url_i.Parse(PROXY_ADDR)
		tr.Proxy = http.ProxyURL(url_proxy)
	}

	re := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	client := &http.Client{
		Transport:     tr,
		CheckRedirect: re,
		Timeout:       timeout,
	}

	if followRedirects {
		client.CheckRedirect = nil
	}

	// domain/port pairs are initially sent on the httpsURLs channel.
	// If they are listening and the --prefer-https flag is set then
	// no HTTP check is performed; otherwise they're put onto the httpURLs
	// channel for an HTTP check.
	httpsURLs := make(chan string)
	httpURLs := make(chan string)
	output := make(chan string)

	// HTTPS workers
	var httpsWG sync.WaitGroup
	for i := 0; i < concurrency/2; i++ {
		httpsWG.Add(1)

		go func() {
			for url := range httpsURLs {

				// always try HTTPS first
				withProto := "https://" + url
				isOpen, lastUrl := isListening(client, withProto, method, userAgent, filterStrings)
				if isOpen {
					output <- lastUrl

					// skip trying HTTP if --prefer-https is set
					if preferHTTPS {
						continue
					}
				}

				httpURLs <- url
			}

			httpsWG.Done()
		}()
	}

	// HTTP workers
	var httpWG sync.WaitGroup
	for i := 0; i < concurrency/2; i++ {
		httpWG.Add(1)

		go func() {
			for url := range httpURLs {
				withProto := "http://" + url
				isOpen, lastUrl := isListening(client, withProto, method, userAgent, filterStrings)
				if isOpen {
					output <- lastUrl
					continue
				}
			}

			httpWG.Done()
		}()
	}

	// Close the httpURLs channel when the HTTPS workers are done
	go func() {
		httpsWG.Wait()
		close(httpURLs)
	}()

	// Output worker
	var outputWG sync.WaitGroup
	outputWG.Add(1)
	go func() {
		for o := range output {
			fmt.Println(o)
		}
		outputWG.Done()
	}()

	// Close the output channel when the HTTP workers are done
	go func() {
		httpWG.Wait()
		close(output)
	}()

	// accept domains on stdin
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		domain := strings.ToLower(sc.Text())

		// Skip unresolvable domains
		if _, err := net.LookupIP(domain); err != nil {
			continue
		}

		// submit standard port checks
		if !skipDefault {
			httpsURLs <- domain
		}

		// submit any additional proto:port probes
		for _, p := range probes.httpsPorts {
			// This is a little bit funny as "https" will imply an
			// http check as well unless the --prefer-https flag is
			// set. On balance I don't think that's *such* a bad thing
			// but it is maybe a little unexpected.
			httpsURLs <- fmt.Sprintf("%s:%s", domain, p)
		}
		for _, p := range probes.httpPorts {
			httpURLs <- fmt.Sprintf("%s:%s", domain, p)
		}
	}

	// once we've sent all the URLs off we can close the
	// input/httpsURLs channel. The workers will finish what they're
	// doing and then call 'Done' on the WaitGroup
	close(httpsURLs)

	// check there were no errors reading stdin (unlikely)
	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input: %s\n", err)
	}

	// Wait until the output waitgroup is done
	outputWG.Wait()
}

func isListening(client *http.Client, url, method string, userAgent string, filterStrings []string) (bool, string) {

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return false, url
	}

	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Connection", "close")
	req.Close = true

	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()

		if len(filterStrings) != 0 {
			// Read the first 512 bytes of the response and check for presence of any filter strings
			peek := make([]byte, 512)
			resp.Body.Read(peek)
			peekStr := string(peek)
			for _, filterString := range filterStrings {
				if strings.Contains(peekStr, filterString) {
					return true, url
				}
			}
		} else {
			io.Copy(io.Discard, resp.Body)
		}
	}

	if err != nil {
		return false, url
	}

	// In case any redirection was followed, returns the last url accessed
	return true, resp.Request.URL.String()
}
