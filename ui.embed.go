package gojourney

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed ui/dist
var journeyUI embed.FS

func embeddedJourneyUI() http.FileSystem {
	ui, err := fs.Sub(journeyUI, "ui/dist")
	if err != nil {
		return http.FS(journeyUI)
	}
	return http.FS(ui)
}

func registerRESTUI(router gin.IRouter, uiPath string, fileSystem http.FileSystem) {
	uiPath = cleanRESTUIPath(uiPath)
	if fileSystem == nil {
		fileSystem = embeddedJourneyUI()
	}
	serve := func(context *gin.Context) {
		name := strings.TrimPrefix(context.Param("filepath"), "/")
		if name == "" {
			serveRESTUIIndex(context, fileSystem, uiPath)
			return
		}
		if file, err := fileSystem.Open(name); err == nil {
			stat, statErr := file.Stat()
			_ = file.Close()
			if statErr == nil && !stat.IsDir() {
				context.FileFromFS(name, fileSystem)
				return
			}
		}
		serveRESTUIIndex(context, fileSystem, uiPath)
	}
	router.GET(uiPath, serve)
	router.GET(path.Join(uiPath, "/*filepath"), serve)
}

func serveRESTUIIndex(context *gin.Context, fileSystem http.FileSystem, uiPath string) {
	file, err := fileSystem.Open("index.html")
	if err != nil {
		context.Status(http.StatusNotFound)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		context.Status(http.StatusInternalServerError)
		return
	}
	data = injectRESTUIBase(data, uiPath)
	context.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

func injectRESTUIBase(data []byte, uiPath string) []byte {
	base := []byte(`<base href="` + cleanRESTUIPath(uiPath) + `/">`)
	if bytes.Contains(data, []byte("<base ")) {
		return data
	}
	if idx := bytes.Index(data, []byte("<head>")); idx >= 0 {
		idx += len("<head>")
		result := make([]byte, 0, len(data)+len(base))
		result = append(result, data[:idx]...)
		result = append(result, base...)
		result = append(result, data[idx:]...)
		return result
	}
	return data
}

func cleanRESTUIPath(uiPath string) string {
	uiPath = "/" + strings.Trim(strings.TrimSpace(uiPath), "/")
	if uiPath == "/" {
		return "/journey-ui"
	}
	return uiPath
}
