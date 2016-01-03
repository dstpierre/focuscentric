package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
)

type pageData struct {
	Title             string
	SubTitle          string
	CurrentEpisode    *Episode
	CurrentProduction *Production
	Productions       []*Production
	LatestEpisodes    []*EpisodeOverview
	Posts             []*Post
	Entry             *Post
	Tags              map[string]string
}

var purchaseTmpl *template.Template

func init() {
	t, err := template.ParseFiles("emails/purchase.html")
	if err != nil {
		log.Println(err.Error())
	} else {
		purchaseTmpl = t
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	prod, err := GetFeatured()
	if err != nil {
		log.Printf("error on db: %s", err)
		http.Redirect(w, r, "/error", http.StatusExpectationFailed)
		return
	}
	d := &pageData{Title: "Focus Centric - Formations video techniques", CurrentProduction: prod, LatestEpisodes: latestEpisodes[0:3]}
	if err := render(w, "index.html", d); err != nil {
		log.Println(err)
	}
}

func collectionsHandler(w http.ResponseWriter, r *http.Request) {
	id := getID(r.URL.Path, "/collections/")
	productions, err := GetCollection(slugToCategory(id))
	if err != nil {
		log.Printf("error on collectionHandler: %s", err)
		http.Redirect(w, r, "/error", http.StatusExpectationFailed)
		return
	}
	d := &pageData{Title: "Formations: " + id, SubTitle: id, Productions: productions, LatestEpisodes: latestEpisodes[0:3]}
	if err := render(w, "collections.html", d); err != nil {
		log.Println(err)
	}
}

func productionHandler(w http.ResponseWriter, r *http.Request) {
	id := getID(r.URL.Path, "/production/")
	production, err := GetProduction(-1, id)
	if err != nil {
		log.Printf("error on productionHandler: %s", err)
		http.Redirect(w, r, "/error", http.StatusExpectationFailed)
		return
	}
	d := &pageData{
		Title:             production.Title,
		SubTitle:          categoryToSlug(production.Category),
		CurrentProduction: production,
		LatestEpisodes:    latestEpisodes[0:3],
	}
	if err := render(w, "production.html", d); err != nil {
		log.Println(err)
	}
}

func episodeHandler(w http.ResponseWriter, r *http.Request) {
	slug := getID(r.URL.Path, "/episode/")
	productionID, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		log.Print("no production id supplied in query string")
		http.Redirect(w, r, "/error", http.StatusExpectationFailed)
		return
	}
	production, err := GetProduction(productionID, "")
	if err != nil {
		log.Printf("error on episodeHandler: %s", err)
		http.Redirect(w, r, "/error", http.StatusExpectationFailed)
		return
	}
	var current *Episode
	for _, e := range production.Episodes {
		if e.Slug == slug {
			current = e
			break
		}
	}
	if current == nil {
		log.Print("unable to find the current epidose")
		http.Redirect(w, r, "/error", http.StatusNotFound)
		return
	}
	d := &pageData{
		Title:             current.Title,
		CurrentEpisode:    current,
		CurrentProduction: production,
		LatestEpisodes:    latestEpisodes[0:3],
	}
	if err := render(w, "episode.html", d); err != nil {
		log.Println(err)
	}
}

func recentHandler(w http.ResponseWriter, r *http.Request) {
	d := &pageData{Title: "Récemment publiés", LatestEpisodes: latestEpisodes}
	if err := render(w, "recent.html", d); err != nil {
		log.Println(err)
	}
}

func blogHandler(w http.ResponseWriter, r *http.Request) {
	title := "Blogue"
	posts := latestPosts
	tag := getID(r.URL.Path, "/blog/tag/")
	if len(tag) > 0 {
		title = "Blogue: " + tag
		posts = nil

		if t, ok := tags[tag]; ok {
			for _, p := range latestPosts {
				if strings.ToUpper(p.Tag) == strings.ToUpper(tag+"|"+t) {
					posts = append(posts, p)
				}
			}
		} else {
			log.Println("Tag not found: " + tag)
		}
	}

	d := &pageData{Title: title, LatestEpisodes: latestEpisodes[0:6], Posts: posts, Tags: tags}
	if err := render(w, "blog.html", d); err != nil {
		log.Println(err)
	}
}

func blogEntryHandler(w http.ResponseWriter, r *http.Request) {
	slug := getID(r.URL.Path, "/blog/show/")
	if len(slug) == 0 {
		http.Redirect(w, r, "/blog", http.StatusMovedPermanently)
		return
	}

	var entry *Post
	for _, b := range latestPosts {
		if strings.ToUpper(slug) == strings.ToUpper(b.Slug) {
			entry = b
			break
		}
	}

	if entry == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	d := &pageData{Title: entry.Title, LatestEpisodes: latestEpisodes, Entry: entry, Tags: tags}
	if err := render(w, "post.html", d); err != nil {
		log.Println(err)
	}
}

func contactHandler(w http.ResponseWriter, r *http.Request) {
	d := &pageData{Title: "Récemment publiés", LatestEpisodes: latestEpisodes[0:3]}
	if err := render(w, "contact.html", d); err != nil {
		log.Println(err)
	}
}

func buyHandler(w http.ResponseWriter, r *http.Request) {
	handleError := func(w http.ResponseWriter, r *http.Request, msg string) {
		log.Println(msg)
		http.Redirect(w, r, "/error", http.StatusBadRequest)
	}

	token := r.FormValue("stripeToken")
	email := r.FormValue("stripeEmail")
	productionID, err := strconv.ParseInt(r.FormValue("id"), 10, 32)
	if err != nil {
		handleError(w, r, "Invalid production id: "+r.FormValue("id"))
		return
	}

	p, err := GetProduction(int(productionID), "")
	if err != nil {
		handleError(w, r, "Production not found")
		return
	}

	stripe.Key = os.Getenv("STRIPE")
	params := &stripe.ChargeParams{}
	params.Amount = uint64(p.CurrentPrice)
	params.Currency = "cad"
	params.Desc = "Achat de " + p.Title
	params.SetSource(token)

	ch, err := charge.New(params)
	if err != nil {
		handleError(w, r, err.Error())
		return
	}

	purchase := Purchase{}
	purchase.ProductionID = int(productionID)
	purchase.Amount = p.CurrentPrice
	purchase.ChargeID = ch.ID
	purchase.Email = email
	err = insertPurchase(purchase)
	if err != nil {
		handleError(w, r, err.Error())
	}

	var emailData = new(struct {
		Name  string
		Title string
		Token string
	})

	emailData.Name = email
	emailData.Title = p.Title

	key := fmt.Sprintf("%s|%d|%s", email, productionID, ch.ID)
	emailData.Token = base64.URLEncoding.EncodeToString([]byte(key))

	var b bytes.Buffer
	purchaseTmpl.Execute(&b, emailData)

	sendMail(email, "Confirmation d'achat", b.String())

	d := &pageData{Title: "Confirmation d'achat", LatestEpisodes: latestEpisodes[0:3]}
	if err := render(w, "confirm.html", d); err != nil {
		log.Println(err)
	}
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	key := getID(r.URL.Path, "/download/")
	if len(key) == 0 {
		http.Redirect(w, r, "/error", http.StatusNotFound)
		return
	}

	b, err := base64.URLEncoding.DecodeString(key)
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusNotFound)
		return
	}

	parts := strings.Split(string(b), "|")
	if len(parts) != 3 {
		http.Redirect(w, r, "/error", http.StatusNotFound)
		return
	}

	prodID, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusNotFound)
		return
	}

	if err = increaseDownload(parts[0], int(prodID), parts[2]); err != nil {
		http.Redirect(w, r, "/error", http.StatusNotFound)
		return
	}

	data, err := ioutil.ReadFile(fmt.Sprintf("prods/%d.zip", prodID))
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%d.zip", prodID))
	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Expires", "0")
	http.ServeContent(w, r, fmt.Sprintf("download/%d.zip", prodID), time.Now(), bytes.NewReader(data))
}

func getID(url string, controller string) string {
	if len(url) < len(controller) || strings.ToUpper(url) == strings.ToUpper(controller) {
		return ""
	}

	return url[len(controller):]
}

func slugToCategory(slug string) string {
	category := ""
	switch strings.ToUpper(slug) {
	case "JAVASCRIPT-NODEJS":
		category = "JavaScript / NodeJS"
	case "NET":
		category = ".NET"
	case "MOBILE":
		category = "iOS, Android, Windows Phone"
	case "PYTHON":
		category = "Python"
	case "GO":
		category = "Go / Golang"
	case "AUTRES":
		category = "Autres / Labs"
	}
	return category
}

func categoryToSlug(category string) string {
	slug := ""
	switch category {
	case "JavaScript / NodeJS":
		slug = "JAVASCRIPT-NODEJS"
	case ".NET":
		slug = "NET"
	case "iOS, Android, Windows Phone":
		slug = "MOBILE"
	case "Python":
		slug = "PYTHON"
	case "GO":
		slug = "GO"
	case "AUTRES":
		slug = "Autres"
	}
	return strings.ToLower(slug)
}
