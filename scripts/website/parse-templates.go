//go:generate go run parse-templates.go
package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

func main() {

	_, scriptPath, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Could not get current working directory")
	}
	tpgDir := scriptPath
	for !strings.HasPrefix(filepath.Base(tpgDir), "terraform-provider-") && tpgDir != "/" {
		tpgDir = filepath.Clean(tpgDir + "/..")
	}
	if tpgDir == "/" {
		log.Fatal("Script was run outside of fastly provider directory")
	}

	tmpl := template.Must(template.ParseFiles(
							tpgDir + "/website/docs/r/service_v1.html.markdown.tmpl",
							tpgDir + "/website/docs/r/service_v2.html.markdown.tmpl",
							tpgDir + "/website/docs/r/templates/_biqquery_logging.markdown.tmpl",
							tpgDir + "/website/docs/r/templates/blobstorage_logging.markdown.tmpl",
							tpgDir + "/website/docs/r/templates/gcs_logging.markdown.tmpl",
							tpgDir + "/website/docs/r/templates/https_logging.markdown.tmpl",
							tpgDir + "/website/docs/r/templates/splunk_logging.markdown.tmpl",
							tpgDir + "/website/docs/r/templates/syslog_logging.markdown.tmpl",
	))

	pages := []map[string]string{{"name": "service_v1", "path": "r/service_v1.html.markdown"},
								 {"name": "service_v2", "path": "r/service_v2.html.markdown"}}

	for _, page := range pages {
		mdFile, mdFileErr := os.Create(tpgDir + "/website/docs/" + page["path"])
		if mdFileErr != nil {
			panic(mdFileErr)
		}
		defer mdFile.Close()

		mdFileExecuteErr := tmpl.ExecuteTemplate(mdFile, page["name"], nil)
		if mdFileExecuteErr != nil {
			panic(mdFileExecuteErr)
		}
	}


}

