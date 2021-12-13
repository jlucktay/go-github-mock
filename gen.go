package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	"strings"

	"github.com/buger/jsonparser"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

const GITHUB_OPENAPI_DEFINITION_LOCATION = "https://github.com/github/rest-api-description/blob/main/descriptions/api.github.com/api.github.com.json?raw=true"

const OUTPUT_FILE_HEADER = `package mock

// Code generated by gen.go; DO NOT EDIT.

`
const OUTPUT_FILEPATH = "src/mock/endpointpattern.go"

type ScrapeResult struct {
	HTTPMethod      string
	EndpointPattern string
}

var debug bool

func init() {
	flag.BoolVar(&debug, "debug", false, "output debug information")
}

func fetchAPIDefinition(l log.Logger) []byte {
	resp, err := http.Get(GITHUB_OPENAPI_DEFINITION_LOCATION)

	if err != nil {
		level.Error(l).Log(
			"msg", "error fetching github's api definition",
			"err", err.Error(),
		)

		os.Exit(1)
	}

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)

	if err != nil {
		level.Error(l).Log(
			"msg", "error fetching github's api definition",
			"err", err.Error(),
		)

		os.Exit(1)
	}

	return bodyBytes
}

// formatToGolangVarName generated the proper golang variable name
// given a endpoint format from the API
func formatToGolangVarName(l log.Logger, sr ScrapeResult) string {
	result := strings.Title(strings.ToLower(sr.HTTPMethod))

	if sr.EndpointPattern == "/" {
		return result + "Slash"
	}

	// handles urls with dashes in them
	pattern := strings.ReplaceAll(sr.EndpointPattern, "-", "/")

	epSplit := strings.Split(
		pattern,
		"/",
	)

	// handle the first part of the variable name
	for _, part := range epSplit {
		if len(part) < 1 || string(part[0]) == "{" {
			continue
		}

		splitPart := strings.Split(part, "_")

		for _, p := range splitPart {
			result = result + strings.Title(p)
		}
	}

	//handle the "By`X`" part of the variable name
	for _, part := range epSplit {
		if len(part) < 1 {
			continue
		}

		if string(part[0]) == "{" {
			part = strings.ReplaceAll(part, "{", "")
			part = strings.ReplaceAll(part, "}", "")

			result += "By"

			for _, splitPart := range strings.Split(part, "_") {
				result += strings.Title(splitPart)
			}
		}
	}

	return result
}

func formatToGolangVarNameAndValue(l log.Logger, lsr ScrapeResult) string {
	return fmt.Sprintf(
		`var %s EndpointPattern = EndpointPattern{
	Pattern: "%s",
	Method:  "%s",
}
`,
		formatToGolangVarName(l, lsr),
		lsr.EndpointPattern,
		strings.ToUpper(lsr.HTTPMethod),
	) + "\n"
}

func parseOpenApiDefinition(apiDefinition []byte) <-chan ScrapeResult {
	outputChan := make(chan ScrapeResult)

	go func() {
		jsonparser.ObjectEach(
			apiDefinition,
			func(objectKey, endpointDefinition []byte, _ jsonparser.ValueType, _ int) error {
				endpointPattern := string(objectKey)

				jsonparser.ObjectEach(
					endpointDefinition,
					func(method, _ []byte, _ jsonparser.ValueType, _ int) error {
						httpMethod := string(method)

						outputChan <- ScrapeResult{
							HTTPMethod:      httpMethod,
							EndpointPattern: endpointPattern,
						}

						return nil
					},
				)

				return nil
			},
			"paths",
		)

		close(outputChan)
	}()

	return outputChan
}

func main() {
	flag.Parse()

	var l log.Logger

	l = log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))

	l = log.With(l, "caller", log.DefaultCaller)

	if debug {
		l = level.NewFilter(l, level.AllowDebug())
		level.Debug(l).Log("msg", "running in debug mode")
	} else {
		l = level.NewFilter(l, level.AllowInfo())
	}

	apiDefinition := fetchAPIDefinition(l)

	buf := bytes.NewBuffer([]byte(OUTPUT_FILE_HEADER))

	scrapeResultChan := parseOpenApiDefinition(apiDefinition)

	for sr := range scrapeResultChan {
		level.Debug(l).Log(
			"msg", fmt.Sprintf("Writing %s", sr.EndpointPattern),
		)

		code := formatToGolangVarNameAndValue(
			l,
			sr,
		)

		buf.WriteString(code)
	}

	ioutil.WriteFile(
		OUTPUT_FILEPATH,
		buf.Bytes(),
		0755,
	)

	errorsFound := false

	// to catch possible format errors
	if err := exec.Command("gofmt", "-w", "src/mock/endpointpattern.go").Run(); err != nil {
		level.Error(l).Log("msg", fmt.Sprintf("error executing gofmt: %s", err.Error()))
		errorsFound = true
	}

	// to catch everything else (hopefully)
	if err := exec.Command("go", "vet", "./...").Run(); err != nil {
		level.Error(l).Log("msg", fmt.Sprintf("error executing go vet: %s", err.Error()))
		errorsFound = true
	}

	if errorsFound {
		os.Exit(1)
	}
}
