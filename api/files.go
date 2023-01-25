package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type RESTFolderResponse struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	FullName       string `json:"full_name"`
	ContextID      int64  `json:"context_id"`
	ContextType    string `json:"context_type"`
	ParentFolderID int64  `json:"parent_folder_id"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	LockAt         string `json:"lock_at"`
	UnlockAt       string `json:"unlock_at"`
	Position       int64  `json:"position"`
	Locked         bool   `json:"locked"`
	FoldersURL     string `json:"folders_url"`
	FilesURL       string `json:"files_url"`
	FilesCount     int64  `json:"files_count"`
	FoldersCount   int64  `json:"folders_count"`
	Hidden         *bool  `json:"hidden"`
	LockedForUser  bool   `json:"locked_for_user"`
	HiddenForUser  bool   `json:"hidden_for_user"`
	ForSubmissions bool   `json:"for_submissions"`
	CanUpload      bool   `json:"can_upload"`
}

type RESTFileResponse struct {
	ID            int64       `json:"id"`
	UUID          string      `json:"uuid"`
	FolderID      int64       `json:"folder_id"`
	DisplayName   string      `json:"display_name"`
	Filename      string      `json:"filename"`
	UploadStatus  string      `json:"upload_status"`
	ContentType   string      `json:"content-type"`
	URL           string      `json:"url"`
	Size          int64       `json:"size"`
	CreatedAt     string      `json:"created_at"`
	UpdatedAt     string      `json:"updated_at"`
	UnlockAt      string      `json:"unlock_at"`
	Locked        bool        `json:"locked"`
	Hidden        bool        `json:"hidden"`
	LockAt        string      `json:"lock_at"`
	HiddenForUser bool        `json:"hidden_for_user"`
	ThumbnailURL  string      `json:"thumbnail_url"`
	ModifiedAt    string      `json:"modified_at"`
	MimeClass     string      `json:"mime_class"`
	MediaEntryID  interface{} `json:"media_entry_id"`
	LockedForUser bool        `json:"locked_for_user"`
}

func (c *CanvasAPIClient) BuildURL(uri string) string {
	query := ""
	if c.ItemPerPage != 0 {
		query = "?per_page=" + strconv.Itoa(c.ItemPerPage)
	}
	if strings.HasSuffix(c.Host, "/") {
		return c.Host + uri[1:] + query
	} else {
		return c.Host + uri + query
	}
}

func (c *CanvasAPIClient) GetFileByID(fid int64) (RESTFileResponse, error) {
	url := c.BuildURL(fmt.Sprintf("/api/v1/files/%d", fid))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return RESTFileResponse{}, err
	}
	resp := RESTFileResponse{}
	_, err = c.makeJSONRequest(req, &resp)
	return resp, err
}

func (c *CanvasAPIClient) ListAllFilesByCourse(cid int64) (<-chan []RESTFileResponse, *PaginatedResponseController, error) {
	url := c.BuildURL(fmt.Sprintf("/api/v1/courses/%d/files", cid))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	resp := make([]RESTFileResponse, 0)
	pagination, err := c.makeJSONRequest(req, &resp)
	if err != nil {
		return nil, nil, err
	}
	respChan := make(chan []RESTFileResponse, 5)
	respChan <- resp
	if pagination == nil {
		close(respChan)
		return respChan, nil, nil
	}
	pagination.close = func() {
		close(respChan)
	}
	paginationRequestFunc := func(url string) error {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		resp := make([]RESTFileResponse, 0)
		newPagination, err := c.makeJSONRequest(req, &resp)
		if err != nil {
			return err
		}
		newPagination.makeRequest = pagination.makeRequest
		newPagination.close = pagination.close
		*pagination = *newPagination
		respChan <- resp
		return nil
	}
	pagination.makeRequest = paginationRequestFunc
	return respChan, pagination, nil
}

func (c *CanvasAPIClient) GetAllFiles(cid int64) (map[int64]RESTFileResponse, error) {
	res := make(map[int64]RESTFileResponse, 0)
	respChan, pagination, err := c.ListAllFilesByCourse(cid)
	if err != nil {
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		for {
			newResponse, ok := <-respChan
			if !ok {
				done <- struct{}{}
				return
			}
			for _, file := range newResponse {
				res[file.ID] = file
			}
		}
	}()
	if pagination != nil {
		for pagination.HasNext() {
			err := pagination.Next()
			if err != nil {
				return res, err
			}
		}
	}
	pagination.Close()
	<-done
	return res, nil
}

func (c *CanvasAPIClient) ListAllFoldersByCourse(cid int64) (<-chan []RESTFolderResponse, *PaginatedResponseController, error) {
	url := c.BuildURL(fmt.Sprintf("/api/v1/courses/%d/folders", cid))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	resp := make([]RESTFolderResponse, 0)
	pagination, err := c.makeJSONRequest(req, &resp)
	if err != nil {
		return nil, nil, err
	}
	respChan := make(chan []RESTFolderResponse, 5)
	respChan <- resp
	if pagination == nil {
		return respChan, nil, nil
	}
	pagination.close = func() {
		close(respChan)
	}
	paginationRequestFunc := func(url string) error {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		resp := make([]RESTFolderResponse, 0)
		newPagination, err := c.makeJSONRequest(req, &resp)
		if err != nil {
			return err
		}
		newPagination.makeRequest = pagination.makeRequest
		newPagination.close = pagination.close
		*pagination = *newPagination
		respChan <- resp
		return nil
	}
	pagination.makeRequest = paginationRequestFunc
	return respChan, pagination, nil
}

func (c *CanvasAPIClient) GetAllFolders(cid int64) (map[int64]RESTFolderResponse, error) {
	res := make(map[int64]RESTFolderResponse, 0)
	respChan, pagination, err := c.ListAllFoldersByCourse(cid)
	if err != nil {
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		for {
			newResponse, ok := <-respChan
			if !ok {
				done <- struct{}{}
				return
			}
			for _, folder := range newResponse {
				res[folder.ID] = folder
			}
		}
	}()
	if pagination != nil {
		for pagination.HasNext() {
			err := pagination.Next()
			if err != nil {
				return res, err
			}
		}
	}
	pagination.Close()
	<-done
	return res, nil
}
