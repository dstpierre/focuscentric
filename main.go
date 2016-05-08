package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
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

func weblog(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(w, r)

		duration := time.Since(start).Nanoseconds()
		var durationUnits string
		switch {
		case duration > 2000000:
			durationUnits = "ms"
			duration /= 1000000
		case duration > 1000:
			durationUnits = "μs"
			duration /= 1000
		default:
			durationUnits = "ns"
		}

		for k, v := range w.Header() {
			log.Printf("%s = %s", k, v)
		}

		log.Printf("[%d %s] %s '%s'\n", duration, durationUnits, w.Header().Get("Status-Line"), r.URL.Path)
	})
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
	http.Handle("/collections/", weblog(http.HandlerFunc(collectionsHandler)))
	http.Handle("/production/", weblog(http.HandlerFunc(productionHandler)))
	http.Handle("/episode/", weblog(http.HandlerFunc(episodeHandler)))

	http.Handle("/recent", weblog(http.HandlerFunc(recentHandler)))

	http.Handle("/blog/show/", weblog(http.HandlerFunc(blogEntryHandler)))
	http.Handle("/blog/tag/", weblog(http.HandlerFunc(blogHandler)))
	http.Handle("/blog", weblog(http.HandlerFunc(blogHandler)))

	http.Handle("/contact", weblog(http.HandlerFunc(contactHandler)))

	http.Handle("/docs/privacy", weblog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d := &pageData{Title: "Condition de vie privée", LatestEpisodes: latestEpisodes[0:3]}
		if err := render(w, "privacy.html", d); err != nil {
			log.Println(err.Error())
		}
	})))

	http.Handle("/buy", weblog(http.HandlerFunc(buyHandler)))
	http.Handle("/download/", weblog(http.HandlerFunc(downloadHandler)))

	http.Handle("/api/episodes", weblog(auth(http.HandlerFunc(episodesHandler))))
	http.Handle("/api/episodes/", weblog(auth(http.HandlerFunc(episodesHandler))))

	http.Handle("/api/productions", weblog(auth(http.HandlerFunc(productionsHandler))))
	http.Handle("/api/productions/", weblog(auth(http.HandlerFunc(productionsHandler))))

	http.Handle("/error", weblog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d := &pageData{Title: "Une erreur est survenue"}
		if err := render(w, "error.html", d); err != nil {
			log.Println(err.Error())
		}
	})))

	http.Handle("/", weblog(http.HandlerFunc(homeHandler)))

	port := os.Getenv("HTTP_PLATFORM_PORT")
	if len(port) == 0 {
		port = "8081"
	}
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
