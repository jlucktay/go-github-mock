package mock

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

// WithRequestMatchHandler implements a request callback
// for the given `pattern`.
//
// For custom implementations, this handler usage is encouraged.
//
// Example:
//
//	WithRequestMatchHandler(
//		GetOrgsProjectsByOrg,
//		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
//			w.Write(MustMarshal([]github.Project{
//				{
//					Name: github.String("mocked-proj-1"),
//				},
//				{
//					Name: github.String("mocked-proj-2"),
//				},
//			}))
//		}),
//	)
func WithRequestMatchHandler(
	ep EndpointPattern,
	handler http.Handler,
) MockBackendOption {
	return func(router *mux.Router) {
		router.Handle(ep.Pattern, handler).Methods(ep.Method)
	}
}

// WithRequestMatch implements a simple FIFO for requests
// of the given `pattern`.
//
// Once all responses have been used, it shall panic()!
//
// Example:
//
//	WithRequestMatch(
//		GetUsersByUsername,
//		github.User{
//			Name: github.String("foobar"),
//		},
//	)
func WithRequestMatch(
	ep EndpointPattern,
	responsesFIFO ...interface{},
) MockBackendOption {
	responses := [][]byte{}

	for _, r := range responsesFIFO {
		switch v := r.(type) {
		case []byte:
			responses = append(responses, v)
		default:
			responses = append(responses, MustMarshal(r))
		}
	}

	return WithRequestMatchHandler(ep, &FIFOReponseHandler{
		Responses: responses,
	})
}

// WithRequestMatchEnterprise Same as `WithRequestMatch` but for Github Enterprise
func WithRequestMatchEnterprise(
	ep EndpointPattern,
	responsesFIFO ...interface{},
) MockBackendOption {
	// prepend `/api/v3` like go-github: https://github.com/google/go-github/blob/8c7625e6a26563e0e031916cc44231912fc52e49/github/github.go#L375
	ep.Pattern = fmt.Sprintf("/api/v3%s", ep.Pattern)

	return WithRequestMatch(ep, responsesFIFO...)
}

// WithRequestMatchPages honors pagination directives.
//
// Pages can be requested in any order and each page can be called multiple times.
//
// E.g.
//
//	mockedHTTPClient := NewMockedHTTPClient(
//		WithRequestMatchPages(
//			GetOrgsReposByOrg,
//			[]github.Repository{
//				{
//					Name: github.String("repo-A-on-first-page"),
//				},
//				{
//					Name: github.String("repo-B-on-first-page"),
//				},
//			},
//			[]github.Repository{
//				{
//					Name: github.String("repo-C-on-second-page"),
//				},
//				{
//					Name: github.String("repo-D-on-second-page"),
//				},
//			},
//		),
//	)
func WithRequestMatchPages(
	ep EndpointPattern,
	pages ...interface{},
) MockBackendOption {
	p := [][]byte{}

	for _, r := range pages {
		p = append(p, MustMarshal(r))
	}

	return WithRequestMatchHandler(ep, &PaginatedReponseHandler{
		ResponsePages: p,
	})
}

// WithRequestMatchPagesEnterprise Same as `WithRequestMatchPages` but for Github Enterprise
func WithRequestMatchPagesEnterprise(
	ep EndpointPattern,
	pages ...interface{},
) MockBackendOption {
	// prepend `/api/v3` like go-github: https://github.com/google/go-github/blob/8c7625e6a26563e0e031916cc44231912fc52e49/github/github.go#L375
	ep.Pattern = fmt.Sprintf("/api/v3%s", ep.Pattern)

	return WithRequestMatchPages(ep, pages...)
}
