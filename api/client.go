package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

var linkItemRegexp = regexp.MustCompile(`<(.+?)>; rel="(.+?)"`)

type HTTPCodeError struct {
	Code     int
	Response string
}

func (H HTTPCodeError) Error() string {
	return fmt.Sprintf("%d: %s", H.Code, H.Response)
}

type PaginatedResponseController struct {
	current     string
	prev        string
	next        string
	first       string
	last        string
	makeRequest func(url string) error
	close       func()
}

func (c *PaginatedResponseController) HasNext() bool {
	return c.last != c.current && c.next != ""
}

func (c *PaginatedResponseController) HasPrev() bool {
	return c.first != c.current && c.prev != ""
}

func (c *PaginatedResponseController) Next() error {
	return c.makeRequest(c.next)
}

func (c *PaginatedResponseController) Prev() error {
	return c.makeRequest(c.prev)
}

func (c *PaginatedResponseController) Close() {
	c.close()
}

type CanvasAPIClient struct {
	Host        string
	HttpClient  *http.Client
	BearerToken string
	ItemPerPage int
}

func (c *CanvasAPIClient) withAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.BearerToken)
}

func (c *CanvasAPIClient) makeJSONRequest(req *http.Request, data interface{}) (pagination *PaginatedResponseController, err error) {
	c.withAuth(req)
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	responseData, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, HTTPCodeError{
			Code:     resp.StatusCode,
			Response: string(responseData),
		}
	}
	if linkStr := resp.Header.Get("Link"); linkStr != "" {
		pagination = new(PaginatedResponseController)
		links := strings.Split(linkStr, ",")
		for _, linkItem := range links {
			match := linkItemRegexp.FindStringSubmatch(linkItem)
			switch match[2] {
			case "current":
				pagination.current = match[1]
			case "last":
				pagination.last = match[1]
			case "first":
				pagination.first = match[1]
			case "prev":
				pagination.prev = match[1]
			case "next":
				pagination.next = match[1]
			}
		}
	}
	err = json.Unmarshal(responseData, data)
	return
}
