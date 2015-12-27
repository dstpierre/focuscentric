package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"os"
	"regexp"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
)

var db *sql.DB

// Episode represents a single episode / youtube video
type Episode struct {
	ID           int
	ProductionID int
	Title        string
	Description  string
	ReleasedOn   time.Time
	Duration     string
	Slug         string
	YoutubeURL   string
	Minutes      int
}

// EpisodeOverview is used to display quick episode list, like latest episodes
type EpisodeOverview struct {
	ID             int
	Title          string
	Slug           string
	ProductionSlug string
	ReleasedOn     time.Time
	Price          float32
	ProductionID   int
}

// Production represents a video series, containing multiple episodes
type Production struct {
	ID               int           `json:"id"`
	Slug             string        `json:"slug"`
	Title            string        `json:"title"`
	Description      string        `json:"desc"`
	DescriptionHTML  template.HTML `json:"descHtml"`
	PresentationText string        `json:"presentationText"`
	PresentationHTML template.HTML `json:"presentationHtml"`
	Price            float32       `json:"price"`
	SalesPrice       float32       `json:"salesPrice"`
	Status           string        `json:"status"`
	ProductionType   string        `json:"productionType"`
	Author           string        `json:"author"`
	ReleasedOn       time.Time     `json:"releasedO"`
	YoutubePreview   string        `json:"youtubePreview"`
	IsFeatured       bool          `json:"isFeatured"`
	DownloadLink     *string       `json:"downloadLink"`
	Category         string        `json:"category"`
	Tags             string        `json:"tags"`
	Episodes         []*Episode
	EpisodeCount     int
	SingleEpisode    bool
}

// Post represents a blog post
type Post struct {
	ID         int
	Slug       string
	Keywords   string
	Title      string
	Author     string
	Body       string
	BodyHTML   template.HTML
	BodyExcerp string
	Tag        string
	TagLink    string
	TagName    string
	Published  time.Time
	FirstImage string
}

func openConnection() error {
	d, err := sql.Open("mssql", os.Getenv("DB"))
	if err != nil {
		return err
	}

	err = d.Ping()
	if err != nil {
		return err
	}

	db = d

	return nil
}

func closeConnection() {
	if db != nil {
		db.Close()
	}
}

func readEpisode(rows *sql.Rows) (*Episode, error) {
	e := Episode{}
	err := rows.Scan(
		&e.ID,
		&e.ProductionID,
		&e.Title,
		&e.Description,
		&e.ReleasedOn,
		&e.Duration,
		&e.Slug,
		&e.YoutubeURL,
		&e.Minutes,
	)
	return &e, err
}

func readEpisodeOverview(rows *sql.Rows) (*EpisodeOverview, error) {
	e := EpisodeOverview{}
	err := rows.Scan(
		&e.ID,
		&e.Title,
		&e.Slug,
		&e.ProductionSlug,
		&e.ReleasedOn,
		&e.Price,
		&e.ProductionID,
	)
	return &e, err
}

func readProduction(rows *sql.Rows) (*Production, error) {
	prod := Production{}
	err := rows.Scan(
		&prod.ID,
		&prod.Slug,
		&prod.Title,
		&prod.Description,
		&prod.Price,
		&prod.Status,
		&prod.ProductionType,
		&prod.Author,
		&prod.ReleasedOn,
		&prod.YoutubePreview,
		&prod.DownloadLink,
		&prod.SalesPrice,
		&prod.IsFeatured,
		&prod.PresentationText,
		&prod.Category,
		&prod.Tags)

	prod.DescriptionHTML = template.HTML(prod.Description)
	prod.PresentationHTML = template.HTML(prod.PresentationText)

	return &prod, err
}

func readPost(rows *sql.Rows) (*Post, error) {
	post := Post{}
	err := rows.Scan(
		&post.ID,
		&post.Slug,
		&post.Keywords,
		&post.Title,
		&post.Author,
		&post.Body,
		&post.Tag,
		&post.Published)
	return &post, err
}

// GetFeatured return the currently featured production
func GetFeatured() (*Production, error) {
	sql, err := db.Prepare("SELECT * FROM Productions WHERE IsFeatured = 1")
	if err != nil {
		return nil, err
	}
	defer sql.Close()

	rows, err := sql.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prod = &Production{}

	for rows.Next() {
		p, err := readProduction(rows)
		if err != nil {
			return nil, err
		}
		prod = p
	}
	return prod, nil
}

// GetCollection returns the matching production for a specific category
func GetCollection(category string) ([]*Production, error) {
	sql, err := db.Prepare("SELECT * FROM Productions WHERE Category = ?")
	if err != nil {
		return nil, err
	}
	defer sql.Close()

	rows, err := sql.Query(category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var productions []*Production
	for rows.Next() {
		p, err := readProduction(rows)
		if err != nil {
			return nil, err
		}

		productions = append(productions, p)
	}
	return productions, nil
}

// GetProduction returns a production based on a slug with all its episodes
func GetProduction(id int, slug string) (*Production, error) {
	var wc string
	if id > 0 {
		wc = "ID"
	} else {
		wc = "Slug"
	}
	qry := strings.Replace("SELECT * FROM Productions WHERE _ = ?", "_", wc, -1)

	sql, err := db.Prepare(qry)
	if err != nil {
		return nil, err
	}
	defer sql.Close()

	var p interface{}
	if id > 0 {
		p = id
	} else {
		p = slug
	}
	rows, err := sql.Query(p)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		var production *Production
		production, err := readProduction(rows)
		if err != nil {
			return nil, err
		}

		subSQL, err := db.Prepare("SELECT * FROM Episodes WHERE ProductionId = ? ORDER BY ReleasedOn ASC")
		if err != nil {
			return nil, err
		}
		defer subSQL.Close()

		subRows, err := subSQL.Query(production.ID)
		if err != nil {
			return nil, err
		}
		defer subRows.Close()

		var episodes []*Episode
		for subRows.Next() {
			e, err := readEpisode(subRows)
			if err != nil {
				return nil, err
			}

			episodes = append(episodes, e)
		}

		production.Episodes = episodes
		production.EpisodeCount = len(episodes)
		production.SingleEpisode = production.EpisodeCount == 1

		return production, nil
	}
	return nil, fmt.Errorf("production not found: %s", slug)
}

// GetLatestPosts returns the latest 5 blog post
func GetLatestPosts() ([]*Post, error) {
	sql, err := db.Prepare("SELECT * FROM BlogPosts ORDER BY Published DESC")
	if err != nil {
		return nil, err
	}

	var posts []*Post
	rows, err := sql.Query()
	for rows.Next() {
		p, err := readPost(rows)
		if err != nil {
			return nil, err
		}

		p.BodyHTML = template.HTML(p.Body)

		p.BodyExcerp = stripHTML(p.Body)[:300]
    
    t := strings.Split(p.Tag, "|")
    if len(t) == 2 {
      p.TagLink = t[0]
      p.TagName = t[1]
    }

		re, err := regexp.Compile("<img.*?src=\"(.*?)\"[^>]*>")
		if err != nil {
			p.FirstImage = err.Error()
		} else {
			imgs := re.FindAllStringSubmatch(p.Body, -1)
			if len(imgs) >= 1 && len(imgs[0]) >= 2 {
				p.FirstImage = imgs[0][1]
			}
		}

		posts = append(posts, p)
	}
	return posts, nil
}

func getLatestEpisodes() ([]*EpisodeOverview, error) {
	sql, err := db.Prepare("SELECT TOP 9 e.ID, e.Title, e.Slug, p.Slug, e.ReleasedOn, CASE WHEN p.SalesPrice > 0 THEN p.SalesPrice ELSE p.Price END as [Price], e.ProductionID  FROM Episodes e INNER JOIN Productions p ON e.ProductionID = p.ID ORDER BY ReleasedOn DESC")
	if err != nil {
		return nil, err
	}
	defer sql.Close()

	rows, err := sql.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []*EpisodeOverview
	for rows.Next() {
		e, err := readEpisodeOverview(rows)
		if err != nil {
			return nil, err
		}

		episodes = append(episodes, e)
	}

	return episodes, nil
}

func insertProduction(prod *Production) (int64, error) {
	sql, err := db.Prepare("INSERT INTO Productions VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer sql.Close()

	r, err := sql.Exec(prod.Slug,
		prod.Title,
		prod.Description,
		prod.Price,
		prod.Status,
		prod.ProductionType,
		prod.Author,
		time.Now(),
		prod.YoutubePreview,
		prod.DownloadLink,
		prod.SalesPrice,
		prod.IsFeatured,
		prod.PresentationText,
		prod.Category,
		prod.Tags,
	)

	if err != nil {
		return 0, err
	}

	return r.LastInsertId()
}

func updateProduction(prod *Production) error {
	sql, err := db.Prepare(`UPDATE Productions SET
    Slud = ?,
    Title = ?,
    Description = ?,
    Price = ?,
    Status = ?,
    ProductionType = ?,
    Author = ?,
    ReleasedOn = ?,
    YoutubePreview = ?,
    DownloadLink = ?,
    SalesPrice = ?,
    IsFeatured = ?,
    PresentationText = ?,
    Category = ?,
    Tags = ?
  WHERE ID = ?
  `)
	if err != nil {
		return err
	}
	defer sql.Close()

	_, err = sql.Exec(prod.Slug,
		prod.Title,
		prod.Description,
		prod.Price,
		prod.Status,
		prod.ProductionType,
		prod.Author,
		prod.ReleasedOn,
		prod.YoutubePreview,
		prod.DownloadLink,
		prod.SalesPrice,
		prod.IsFeatured,
		prod.PresentationText,
		prod.Category,
		prod.Tags,
		prod.ID,
	)

	return err
}

func insertEpisode(e *Episode) (int64, error) {
	sql, err := db.Prepare("INSERT INTO Episodes VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return -1, err
	}
	defer sql.Close()

	r, err := sql.Exec(e.ProductionID,
		e.Title,
		e.Description,
		e.ReleasedOn,
		e.Duration,
		e.Slug,
		e.YoutubeURL,
		e.Minutes,
	)
	if err != nil {
		return -1, err
	}

	return r.LastInsertId()
}

func updateEpisode(e *Episode) error {
	sql, err := db.Prepare(`UPDATE Episodes SET 
    Title = ?,
    Description = ?,
    ReleasedOn = ?,
    Duration = ?,
    Slug = ?,
    YoutubeURL = ?,
    Minutes = ?
  WHERE ID = ?
  `)
	if err != nil {
		return err
	}
	defer sql.Close()

	_, err = sql.Exec(e.ProductionID,
		e.Title,
		e.Description,
		e.ReleasedOn,
		e.Duration,
		e.Slug,
		e.YoutubeURL,
		e.Minutes,
		e.ID,
	)

	return err
}
