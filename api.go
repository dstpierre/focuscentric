package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
)

func auth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Api-Key")
		log.Printf("Inside middleware with key %s", key)
		if len(key) == 0 || key != "1234" {
			respond(w, r, http.StatusUnauthorized, nil)
		} else {
			h.ServeHTTP(w, r)
		}
	})
}

func parseBody(body io.ReadCloser, v interface{}) error {
	defer body.Close()

	decoder := json.NewDecoder(body)
	return decoder.Decode(v)
}

func respond(w http.ResponseWriter, r *http.Request, status int, data interface{}) error {
	if err, ok := data.(error); ok {
		data = struct {
			Err string `json:"error"`
		}{err.Error()}
	}
	js, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)
	return nil
}

func episodesHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s\t%s", r.Method, r.URL.Path)

	if r.Method == "GET" {
		id := getID(r.URL.Path, "/api/episodes/")
		if len(id) > 0 {
			log.Println("Get id: " + id)
		} else {
			log.Println("Get all episodes")
		}
	} else if r.Method == "POST" || r.Method == "PUT" {
		var data *Episode
		err := parseBody(r.Body, &data)
		if err != nil {
			respond(w, r, http.StatusBadRequest, nil)
			return
		}

		log.Printf("%v\n", data)
		// selon the Method, faire un insert ou un update
		respond(w, r, http.StatusOK, data)
	} else if r.Method == "DELETE" {
		log.Println("delete")
	}
}

func productionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		id := getID(r.URL.Path, "/api/productions/")
		if len(id) > 0 {
			prodID, err := strconv.ParseInt(id, 10, 32)
			if err != nil {
				respond(w, r, http.StatusBadRequest, err)
				return
			}

			p, err := GetProduction(int(prodID), "")
			if err != nil {
				respond(w, r, http.StatusInternalServerError, err)
			} else {
				respond(w, r, http.StatusOK, p)
			}
		} else {
			log.Println("todo list all productions...")
		}
	} else if r.Method == "POST" || r.Method == "PUT" {
		var data *Production
		err := parseBody(r.Body, &data)
		if err != nil {
			respond(w, r, http.StatusBadRequest, nil)
			return
		}

		if data.ID > 0 {
			err = updateProduction(data)
			if err != nil {
				respond(w, r, http.StatusInternalServerError, err)
			} else {
				respond(w, r, http.StatusOK, true)
			}
		} else {
			id, err := insertProduction(data)
			if err != nil {
				respond(w, r, http.StatusInternalServerError, err)
			} else {
				respond(w, r, http.StatusCreated, id)
			}
		}
	} else if r.Method == "DELETE" {
		log.Println("todo delete production")
	}
}
