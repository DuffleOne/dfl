package rpc

import (
	"net/http"
	"path"
	"strings"
	"text/template"
)

func QuickTemplate(w http.ResponseWriter, data interface{}, templates []string) error {
	_, firstName := path.Split(templates[0])

	tpl, err := template.New(firstName).Delims("[[", "]]").ParseFiles(templates...)
	if err != nil {
		return err
	}

	if len(templates) == 1 {
		return tpl.ExecuteTemplate(w, firstName, data)
	}

	lastItem := templates[len(templates)-1]
	_, file := path.Split(lastItem)
	ext := path.Ext(file)
	file = strings.TrimSuffix(file, ext)

	return tpl.ExecuteTemplate(w, file, data)
}
