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

// Production represents a video series, containing multiple episodes
type Production struct {
	ID               int
	Slug             string
	Title            string
	Description      string
	DescriptionHTML  template.HTML
	PresentationText string
	PresentationHTML template.HTML
	Price            float32
	SalesPrice       float32
	Status           string
	ProductionType   string
	Author           string
	ReleasedOn       time.Time
	YoutubePreview   string
	IsFeatured       bool
	DownloadLink     *string
	Category         string
	Tags             string
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
	Tag        string
	Published  time.Time
	FirstImage string
}

func openConnection() (*sql.DB, error) {
	return sql.Open("mssql", os.Getenv("FOCUS_SQL"))
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
	conn, err := openConnection()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	sql, err := conn.Prepare("SELECT * FROM Productions WHERE IsFeatured = 1")
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
	conn, err := openConnection()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	sql, err := conn.Prepare("SELECT * FROM Productions WHERE Category = ?")
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
	conn, err := openConnection()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	var wc string
	if id > 0 {
		wc = "ID"
	} else {
		wc = "Slug"
	}
	qry := strings.Replace("SELECT * FROM Productions WHERE _ = ?", "_", wc, -1)

	sql, err := conn.Prepare(qry)
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

		subSQL, err := conn.Prepare("SELECT * FROM Episodes WHERE ProductionId = ? ORDER BY ReleasedOn ASC")
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

// GetHomePosts returns the latest 5 blog post
func GetHomePosts() ([]*Post, error) {
	conn, err := openConnection()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	sql, err := conn.Prepare("SELECT TOP 5 * FROM BlogPosts ORDER BY Published DESC")
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

		re, err := regexp.Compile("<img.*?src=\"(.*?)\"[^>]*>")
		if err != nil {
			p.FirstImage = ""
		} else {
			imgs := re.FindAllStringSubmatch(p.Body, -1)
			if len(imgs) > 0 {
				p.FirstImage = imgs[0][0]
			}
		}

		posts = append(posts, p)
	}
	return posts, nil
}
