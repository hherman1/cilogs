package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var auth = flag.String("a", "", "sets the API token to use with circle")
var out = flag.String("d", ".", "sets the directory to write all artifacts to")
var printOnly = flag.Bool("p", false, "if set, only prints artifact URLs and does not download them")

func main() {
	if err := run(); err != nil {
		fmt.Println("error:", err)
		flag.Usage()
		os.Exit(1)
	}
}

func init() {
	flag.Usage = func() {
		fmt.Println("Usage of cilogs <jobUrl>:")
		flag.PrintDefaults()
	}
}

func run() error {
	flag.Parse()
	if *auth == "" {
		return errors.New("-a is a required argument")
	}
	if len(flag.Args()) != 1 {
		return fmt.Errorf("expected exactly 1 URL argument, got %v", len(flag.Args()))
	}
	u := flag.Args()[0]

	// compute project slug
	pidx := strings.Index(u, "/pipelines")
	if pidx == -1 {
		return fmt.Errorf("could not parse url: could not find substring '/pipelines': %v", u)
	}
	widx := strings.Index(u, "/workflows")
	if widx == -1 {
		return fmt.Errorf("could not parse url: could not find substring '/workflows': %v", u)
	}
	endpslug := strings.LastIndex(u[:widx], "/")
	if endpslug == -1 {
		return fmt.Errorf("could not parse url: could not find last '/' in substring: %v", u[:widx])
	}
	pslug := u[pidx+len("/pipelines/") : endpslug]

	// compute job number
	jidx := strings.Index(u, "/jobs/")
	if jidx == -1 {
		return fmt.Errorf("could not parse url: could not find substring '/jobs/': %v", u)
	}
	jobN, _, _ := strings.Cut(u[jidx+len("/jobs/"):], "/")
	if _, err := strconv.Atoi(jobN); err != nil {
		return fmt.Errorf("could not parse url: job id %v was not a number: %w", jobN, err)
	}

	// Create first artifact URI
	u = strings.Replace(u, "app.", "", 1) // remove possible leading app.
	uurl, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("could not parse url: %w", err)
	}
	uurl.Path = fmt.Sprintf("/api/v2/project/%v/%v/artifacts", pslug, jobN)
	uurl.RawQuery = ""
	r, err := http.NewRequest("GET", uurl.String(), nil)
	if err != nil {
		return fmt.Errorf("create artifacts http request: %w", err)
	}
	r.Header.Add("Circle-Token", *auth)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return fmt.Errorf("load artifact list: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bs, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("bad response status code: %v", resp.StatusCode)
		}
		return fmt.Errorf("bad response %v: %v", resp.StatusCode, string(bs))
	}
	var artifacts struct {
		Items []struct {
			Path string `json:"path"`
			URL  string `json:"url"`
		} `json:"items"`
	}
	err = json.NewDecoder(resp.Body).Decode(&artifacts)
	if err != nil {
		return fmt.Errorf("parsing artifacts JSON: %w", err)
	}
	resp.Body.Close()

	if *printOnly {
		for _, a := range artifacts.Items {
			fmt.Println(a.URL)
		}
		return nil
	}

	// download all the artifacts
	const workers = 16
	tickets := make(chan struct{}, workers)
	for i := 0; i < workers; i++ {
		tickets <- struct{}{}
	}
	results := make(chan error)
	for _, a := range artifacts.Items {
		a := a
		go func() {
			_ = <-tickets
			defer func() {
				tickets <- struct{}{}
			}()
			results <- downloadArtifact(a.Path, a.URL)
		}()
	}
	for _ = range artifacts.Items {
		err = <-results
		if err != nil {
			return err
		}
	}

	return nil
}

// execution        40.89 real         1.04 user         0.90 sys
func downloadArtifact(rpath, aurl string) error {

	err := func() error {
		r, err := http.NewRequest("GET", aurl, nil)
		if err != nil {
			return fmt.Errorf("create http request: %w", rpath, err)
		}
		r.Header.Add("Circle-Token", *auth)
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			return fmt.Errorf("load artifact: %w", rpath, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			bs, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("bad response status code: %v", rpath, resp.StatusCode)
			}
			return fmt.Errorf("bad response %v: %v", rpath, resp.StatusCode, string(bs))
		}

		// copy body to destination
		dest := filepath.Join(*out, strings.Replace(rpath, "~/", "", 1))
		err = os.MkdirAll(filepath.Dir(dest), 0777)
		if err != nil {
			return fmt.Errorf("make output dir: %w", rpath, err)
		}
		f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
		if err != nil {
			return fmt.Errorf("open output file: %w", rpath, err)
		}
		defer f.Close()
		_, err = io.Copy(f, resp.Body)
		if err != nil {
			return fmt.Errorf("copy body to output file: %w", err)
		}
		return nil
	}()
	if err != nil {
		return fmt.Errorf("artifact '%v': %w", rpath, err)
	}
	fmt.Println("downloaded", aurl)
	return nil
}
