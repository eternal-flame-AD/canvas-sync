package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	badger "github.com/dgraph-io/badger"

	"github.com/jinzhu/configor"

	"github.com/cavaliergopher/grab/v3"

	"github.com/eternal-flame-AD/canvas-sync/api"
)

func sanitizeFn(s string) string {
	for oldChar, newChar := range map[string]string{
		"%":  "％",
		"<":  "＞",
		">":  "＜",
		"\"": "-",
		"/":  "_",
		"\\": "",
		":":  "：",
		"*":  "",
		"?":  "？",
	} {
		s = strings.ReplaceAll(s, oldChar, newChar)
	}
	return strings.TrimSpace(s)
}

type Config struct {
	CourseID    int64
	WorkerCount int
	Token       string
	Host        string
	UseModules  bool
}

var (
	config = new(Config)
	db     *badger.DB
)

type File struct {
	FolderPath []string
	FileName   string
	Size       int64
	CreatedAt  string
	UpdatedAt  string
	ModifiedAt string
	URL        string
}

func Rev[T comparable](s []T) {
	for i := 0; i < len(s)/2; i++ {
		s[i], s[len(s)-1-i] = s[len(s)-1-i], s[i]
	}
}

func DownloadFiles(list []File) {
	client := grab.NewClient()
	var batch []*grab.Request
	for _, f := range list {
		pathElements := make([]string, len(f.FolderPath)+1)
		for i := range f.FolderPath {
			pathElements[i] = sanitizeFn(f.FolderPath[i])
		}
		pathElements[len(f.FolderPath)] = sanitizeFn(f.FileName)
		fullPath := strings.Join(pathElements, string(os.PathSeparator))
		os.MkdirAll(path.Dir(fullPath), 0644)
		if db != nil && compareLocalFile(db, fullPath, f) {
			log.Printf("%s - Local file up to date.", fullPath)
			continue
		}
		req, err := grab.NewRequest(fullPath, f.URL)
		if err != nil {
			log.Printf("Error building download request: %v", err)
			continue
		}
		req.Tag = f
		batch = append(batch, req)
	}
	respChan := client.DoBatch(config.WorkerCount, batch...)

	currentTasks := make(map[string]*grab.Response)

	timer := time.NewTicker(500 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case resp, ok := <-respChan:
			if !ok {
				return
			}
			currentTasks[resp.Filename] = resp
			curResp := resp
			go func() {
				curResp.Wait()
				if err := curResp.Err(); err != nil {
					log.Printf("Error downloading %s: %v", curResp.Filename, err)
				}
				updateLocalFileDB(db, resp.Filename, resp.Request.Tag.(File))
				delete(currentTasks, resp.Filename)
			}()
		case <-timer.C:
			for fn, response := range currentTasks {
				fmt.Printf(
					"%s %d/%d bytes (%.2f%%) [ETA: %ds]\n",
					fn,
					response.BytesComplete(),
					response.BytesComplete(),
					response.Progress()*100.,
					response.ETA().Second(),
				)
			}
		}
	}
}

func init() {
	configFile := "canvas-sync.yml"
	if len(os.Args) == 2 {
		configFile = os.Args[1]
	}
	os.Chdir(path.Dir(configFile))
	configor.Load(&config, configFile)

	dbopt := badger.DefaultOptions(".canvas-sync")
	dbopt = dbopt.WithLevelOneSize(1 << 20)

	db, _ = badger.Open(dbopt)
}

func main() {
	defer db.Close()

	client := &api.CanvasAPIClient{
		HttpClient:  new(http.Client),
		Host:        config.Host,
		BearerToken: config.Token,
	}

	var files []File
	var err error
	if config.UseModules {
		files, err = getFileListFromModulesAPI(client)
	} else {
		files, err = getFileListFromFilesAPI(client)
	}
	if err != nil {
		log.Fatalf("Error while getting file list from files api: %v", err)
	}

	log.Printf("%d files loaded.", len(files))
	DownloadFiles(files)
}

func getFileListFromFilesAPI(client *api.CanvasAPIClient) ([]File, error) {
	folders, err := client.GetAllFolders(config.CourseID)
	if err != nil {
		return nil, err
	}
	files, err := client.GetAllFiles(config.CourseID)
	if err != nil {
		return nil, err
	}
	res := make([]File, 0)
	for _, file := range files {
		path := make([]string, 0)
		curFolder := file.FolderID
		for curFolder != 0 {
			path = append(path, folders[curFolder].Name)
			curFolder = folders[curFolder].ParentFolderID
		}
		Rev(path)
		res = append(res, File{
			FolderPath: path,
			FileName:   file.DisplayName,
			Size:       file.Size,
			CreatedAt:  file.CreatedAt,
			UpdatedAt:  file.UpdatedAt,
			ModifiedAt: file.ModifiedAt,
			URL:        file.URL,
		})
	}
	return res, nil
}

func getFileListFromModulesAPI(client *api.CanvasAPIClient) ([]File, error) {
	modulesItems, err := client.ListModuleItemsAll(strconv.FormatInt(config.CourseID, 10))
	if err != nil {
		return nil, err
	}
	log.Printf("%d module items found.", len(modulesItems))
	var res []File
	curModule := ""
	curSubHeader := ""
	for _, item := range modulesItems {

		if len(item.Content.Modules) > 0 {
			newModule := item.Content.Modules[0].Name
			if newModule != curModule {
				curModule = newModule
				curSubHeader = ""
			}
		}

		switch item.Content.Typename {
		case "File":
			idInt, err := strconv.ParseInt(item.Content.IDLegacy, 10, 64)
			if err != nil {
				log.Printf("Error while parsing file id %s: %v", item.Content.IDLegacy, err)
				continue
			}
			fileInfo, err := client.GetFileByID(idInt)
			if err != nil {
				log.Printf("Error while getting file info for file %s: %v", item.Content.IDLegacy, err)
				continue
			}
			f := File{
				FolderPath: []string{"Modules", curModule},
				FileName:   fileInfo.DisplayName,
				Size:       fileInfo.Size,
				CreatedAt:  fileInfo.CreatedAt,
				UpdatedAt:  fileInfo.UpdatedAt,
				URL:        fileInfo.URL,
			}
			if curSubHeader != "" {
				f.FolderPath = append(f.FolderPath, curSubHeader)
			}
			res = append(res, f)
		case "SubHeader":
			curSubHeader = item.Content.Title
		}
	}
	return res, nil
}
