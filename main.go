package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	templates      map[string]*template.Template
	latestEpisodes []*EpisodeOverview
	latestPosts    []*Post
	tags           map[string]string
)

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
	err := openConnection()
	if err != nil {
		log.Fatal(err)
	}
	defer closeConnection()

	le, err := getLatestEpisodes()
	if err != nil {
		log.Println("Cannot get latest episodes: " + err.Error())
	} else {
		latestEpisodes = le
	}

	lb, err := GetLatestPosts()
	if err != nil {
		log.Println("Cannot get latest blog posts")
	} else {
		latestPosts = lb

		tags = make(map[string]string)

		for _, p := range lb {
			t := strings.Split(p.Tag, "|")
			if _, ok := tags[t[0]]; !ok {
				tags[t[0]] = t[1]
			}
		}
	}

	loadTemplates()
	http.HandleFunc("/content/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, r.URL.Path[1:])
	})
	http.HandleFunc("/collections/", collectionsHandler)
	http.HandleFunc("/production/", productionHandler)
	http.HandleFunc("/episode/", episodeHandler)

	http.HandleFunc("/recent", recentHandler)

	http.HandleFunc("/blog/show/", blogEntryHandler)
	http.HandleFunc("/blog/tag/", blogHandler)
	http.HandleFunc("/blog", blogHandler)

	http.HandleFunc("/contact", contactHandler)

	http.HandleFunc("/docs/privacy", func(w http.ResponseWriter, r *http.Request) {
		d := &pageData{Title: "Condition de vie privée", LatestEpisodes: latestEpisodes[0:3]}
		if err := render(w, "privacy.html", d); err != nil {
			log.Println(err.Error())
		}
	})

	http.HandleFunc("/buy", buyHandler)
	http.HandleFunc("/download/", downloadHandler)

	http.Handle("/api/episodes", auth(http.HandlerFunc(episodesHandler)))
	http.Handle("/api/episodes/", auth(http.HandlerFunc(episodesHandler)))

	http.Handle("/api/productions", auth(http.HandlerFunc(productionsHandler)))
	http.Handle("/api/productions/", auth(http.HandlerFunc(productionsHandler)))

	http.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		d := &pageData{Title: "Une erreur est survenue"}
		if err := render(w, "error.html", d); err != nil {
			log.Println(err.Error())
		}
	})

	http.HandleFunc("/", homeHandler)

	port := os.Getenv("HTTP_PLATFORM_PORT")
	if len(port) == 0 {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
