package api

import (
	"net/http"
	"net/url"
	"strings"
)

type ModuleItem struct {
	Content struct {
		Typename    string `json:"__typename"`
		ID          string `json:"id"`
		IDLegacy    string `json:"_id"`
		DisplayName string `json:"displayName,omitempty"`
		Title       string `json:"title,omitempty"`

		Modules []struct {
			Name string `json:"name"`
		} `json:"modules,omitempty"`
	} `json:"content,omitempty"`
}

type GraphQLModuleResponse struct {
	Course struct {
		ModulesConnection struct {
			Nodes []struct {
				Name        string       `json:"name"`
				ID          string       `json:"id"`
				ModuleItems []ModuleItem `json:"moduleItems"`
			} `json:"nodes"`
			PageInfo ModulePageInfo `json:"pageInfo"`
		} `json:"modulesConnection"`
	} `json:"course"`
}

type ModulePageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

func (c *CanvasAPIClient) ListModuleItemsAll(courseId string) ([]ModuleItem, error) {
	var moduleItems []ModuleItem
	var hasNextPage = true
	var endCursor = ""
	for hasNextPage {
		moduleItemsPage, pageInfo, err := c.ListModuleItems(courseId, endCursor)
		if err != nil {
			return nil, err
		}
		moduleItems = append(moduleItems, moduleItemsPage...)
		hasNextPage = pageInfo.HasNextPage
		endCursor = pageInfo.EndCursor
	}
	return moduleItems, nil
}

func (c *CanvasAPIClient) ListModuleItems(courseId string, endCursor string) ([]ModuleItem, ModulePageInfo, error) {
	const query = `
	query ModulesQuery($courseId: ID!, $pageCursor: String) {
		course(id: $courseId) {
		  modulesConnection(after: $pageCursor) {
			nodes {
			  name
			  id
			  moduleItems {
				content {
				  ... on File {
					__typename
					id
					displayName
					_id
					modules {
						name
					}
				  }
				  ... on SubHeader {
					__typename
					title
					modules {
						name
					}
				  }
				}
			  }
			}
			pageInfo {
			  hasNextPage
			  endCursor
			}
		  }
		}
	  }
	  
	`
	requestParams := url.Values{
		"query":                 {query},
		"variables[courseId]":   {courseId},
		"variables[pageCursor]": {endCursor},
	}
	req, err := http.NewRequest("POST", c.BuildURL("/api/graphql"), strings.NewReader(requestParams.Encode()))
	if err != nil {
		return nil, ModulePageInfo{}, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	var resp struct {
		Data GraphQLModuleResponse `json:"data"`
	}
	if _, err := c.makeJSONRequest(req, &resp); err != nil {
		return nil, ModulePageInfo{}, err
	}
	var moduleItems []ModuleItem
	for _, module := range resp.Data.Course.ModulesConnection.Nodes {
		moduleItems = append(moduleItems, module.ModuleItems...)
	}
	return moduleItems, resp.Data.Course.ModulesConnection.PageInfo, nil
}
