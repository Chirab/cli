package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/Scalingo/cli/config"
	"github.com/Scalingo/cli/debug"
	"github.com/Scalingo/cli/httpclient"
	"github.com/Scalingo/cli/io"
	"github.com/Scalingo/cli/users"
	"gopkg.in/errgo.v1"
)

type AppNotFound struct {
	App string
}

func (err *AppNotFound) Error() string {
	return fmt.Sprintf("Application '%s' has not been found, have you done a typo?", err.App)
}

type APIRequest struct {
	NoAuth   bool
	URL      string
	Method   string
	Endpoint string
	Token    string
	Expected Statuses
	Params   interface{}
}

func (req *APIRequest) FillDefaultValues() error {
	if req.URL == "" {
		host := config.C.ApiUrl
		req.URL = fmt.Sprintf("%s%s", host, Prefix)
	}
	if req.Method == "" {
		req.Method = "GET"
	}
	if req.Expected == nil || len(req.Expected) == 0 {
		req.Expected = Statuses{200}
	}
	if req.Params == nil {
		req.Params = make(map[string]interface{})
	}
	if req.Token == "" && !req.NoAuth {
		user, err := AuthFromConfig()
		if err != nil {
			return errgo.Mask(err, errgo.Any)
		}
		if user == nil {
			fmt.Println("You need to be authenticated to use Scalingo client.\nNo account ? → https://scalingo.com")
			user, err = Auth()
			if err != nil {
				return errgo.Mask(err, errgo.Any)
			}
		}
		CurrentUser = user
		req.Token = CurrentUser.AuthToken
	}
	return nil
}

var CurrentUser *users.User
var Prefix = config.C.ApiPrefix

type Statuses []int

func (statuses Statuses) Contains(status int) bool {
	for _, s := range statuses {
		if s == status {
			return true
		}
	}
	return false
}

// Execute an API request and return its response/error
func (req *APIRequest) Do() (*http.Response, error) {
	err := req.FillDefaultValues()
	if err != nil {
		return nil, errgo.Mask(err, errgo.Any)
	}

	var httpReq *http.Request

	if req.isAppRelated() {
		ok, err := req.checkAppExist()
		if err != nil {
			return nil, errgo.Mask(err)
		}
		if !ok {
			return nil, &AppNotFound{req.getApp()}
		}
	}

	// Execute the HTTP request according to the HTTP method
	switch req.Method {
	case "PATCH":
		fallthrough
	case "POST":
		fallthrough
	case "WITH_BODY":
		buffer, err := json.Marshal(req.Params)
		if err != nil {
			return nil, errgo.Mask(err, errgo.Any)
		}
		reader := bytes.NewReader(buffer)
		httpReq, err = http.NewRequest(req.Method, req.URL+"/"+req.Endpoint, reader)
		if err != nil {
			return nil, errgo.Mask(err, errgo.Any)
		}
	case "GET", "DELETE":
		values := url.Values{}
		for key, value := range req.Params.(map[string]interface{}) {
			values.Add(key, fmt.Sprintf("%v", value))
		}
		req.Endpoint = fmt.Sprintf("%s?%s", req.Endpoint, values.Encode())
		httpReq, err = http.NewRequest(req.Method, req.URL+"/"+req.Endpoint, nil)
		if err != nil {
			return nil, errgo.Mask(err, errgo.Any)
		}
	}

	debug.Printf("[API] %v %v\n", httpReq.Method, httpReq.URL)
	debug.Printf(io.Indent(fmt.Sprintf("Headers: %v", httpReq.Header), 6))
	debug.Printf(io.Indent("Params : %v", 6), req.Params)

	httpReq.SetBasicAuth("", req.Token)
	res, err := httpclient.Do(httpReq)
	if err != nil {
		fmt.Printf("Fail to query %s: %v\n", httpReq.Host, err)
		os.Exit(1)
	}

	if req.Expected.Contains(res.StatusCode) {
		return res, nil
	}

	defer res.Body.Close()

	if res.StatusCode == 422 {
		var unprocessableError *UnprocessableEntity
		err = ParseJSON(res, &unprocessableError)
		if err != nil {
			return nil, errgo.Mask(err, errgo.Any)
		}
		return nil, errgo.Mask(unprocessableError, errgo.Any)
	}

	if res.StatusCode == 500 {
		return nil, NewRequestFailedError(res.StatusCode, "server internal error - our team has been notified", httpReq)
	}

	if req.Token != "" && res.StatusCode == 401 {
		return nil, NewRequestFailedError(res.StatusCode, "unauthorized - you are not authorized to do this operation", httpReq)
	}

	return nil, NewRequestFailedError(res.StatusCode, fmt.Sprintf("invalid status from server: %v", res.Status), httpReq)
}

func ParseJSON(res *http.Response, data interface{}) error {
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errgo.Mask(err, errgo.Any)
	}

	debug.Println(string(body))

	return json.Unmarshal(body, data)
}

func (req *APIRequest) isAppRelated() bool {
	return strings.HasPrefix(req.Endpoint, "/apps/")
}

func (req *APIRequest) getApp() string {
	if req.isAppRelated() {
		return strings.Split(req.Endpoint, "/")[2]
	}
	return ""
}

func (req *APIRequest) checkAppExist() (bool, error) {
	r, err := http.NewRequest("HEAD", req.URL+"/apps/"+req.getApp(), nil)
	if err != nil {
		return false, errgo.Mask(err, errgo.Any)
	}
	r.SetBasicAuth("", req.Token)
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return false, errgo.Mask(err, errgo.Any)
	}
	return res.StatusCode == 200, nil
}
