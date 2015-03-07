package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

var templates map[string]*template.Template

func loadTemplates() {
	if templates == nil {
		templates = make(map[string]*template.Template)
	}

	layouts, err := filepath.Glob("views/layouts/*.html")
	if err != nil {
		log.Fatal(err)
	}

	pages, err := filepath.Glob("views/pages/*.html")
	if err != nil {
		log.Fatal(err)
	}

	for _, page := range pages {
		for _, layout := range layouts {
			templates[filepath.Base(page)] = template.Must(template.New(page).ParseFiles(layout, page))
		}
	}
}

func render(w http.ResponseWriter, name string, data *pageData) (err error) {
	template, ok := templates[name]
	if !ok {
		err = fmt.Errorf("The template %s does not exists", name)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	template.ExecuteTemplate(w, "base", data)

	return nil
}

func main() {
	loadTemplates()
	http.HandleFunc("/content/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, r.URL.Path[1:])
	})
	http.HandleFunc("/collections/", collectionsHandler)
	http.HandleFunc("/production/", productionHandler)
	http.HandleFunc("/episode/", episodeHandler)
	http.HandleFunc("/", homeHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
