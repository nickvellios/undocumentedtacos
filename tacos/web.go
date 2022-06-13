package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/disintegration/imaging"
	"github.com/julienschmidt/httprouter"
)

var templates = template.Must(template.ParseFiles(
	templateDir + "index.html",
))

type Page struct {
	Title   string
	Content interface{}
}

type Location struct {
	ID          int     `json:"id"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	Ext         string  `json:"ext"`
	Name        string  `json:"name"`
	LastUpdated string  `json:"last_updated"`
}

type Response struct {
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

const (
	MAX_FILE = 20000000 // Max size of file, 20mb
)

func renderTemplate(w http.ResponseWriter, tmpl string, data *Page) {
	err := templates.ExecuteTemplate(w, tmpl, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Send a JSON result back to the client with given status code
func writeResponse(w http.ResponseWriter, code int, r interface{}, error string) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	res := &Response{r, error}
	err := json.NewEncoder(w).Encode(res)

	return err
}

// Index page handler
func (g *glob) Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	p := &Page{
		Title: "Home",
		Content: struct {
			Value1 interface{}
			Value2 interface{}
		}{
			template.HTML(""),
			template.HTML(""),
		},
	}
	renderTemplate(w, "index", p)
}

// Index port 80 page handler
func (g *glob) Index80(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	http.Redirect(w, r, "https://undocumentedtacos.com", 302)
}

// Truck list page handler
func (g *glob) Trucks(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	lat := ps.ByName("lat")
	lng := ps.ByName("lng")
	rad := ps.ByName("rad")

	err := writeResponse(w, 200, g.loadTrucks(lat, lng, rad), "")
	if err != nil {
		fmt.Println(err.Error())
	}
}

// Flag truck
func (g *glob) Flag(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	msg, err := g.flag(id)
	if err != nil {
		writeResponse(w, 500, "", "Something broke...")
	}
	err = writeResponse(w, 200, msg, "")
	if err != nil {
		fmt.Println(err.Error())
	}
}

// Truck sighting
func (g *glob) Sighting(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	rating := ps.ByName("rating")
	msg, err := g.sighting(id, rating)
	if err != nil {
		writeResponse(w, 500, "", "Something broke...")
	}
	err = writeResponse(w, 200, msg, "")
	if err != nil {
		fmt.Println(err.Error())
	}
}

// Truck rating
func (g *glob) Rate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	vote := ps.ByName("vote")
	msg, err := g.rate(id, vote)
	if err != nil {
		writeResponse(w, 500, "", "Something broke...")
	}
	err = writeResponse(w, 200, msg, "")
	if err != nil {
		fmt.Println(err.Error())
	}
}

func (g *glob) AddTruck(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Make sure we aren't flooded with excess data from some asshole
	if r.ContentLength > MAX_FILE {
		writeResponse(w, 413, "", "File exceeds maximum size")
		return
	}

	// Check again to be sure we aren't flooded with excess data
	r.Body = http.MaxBytesReader(w, r.Body, MAX_FILE)

	r.ParseMultipartForm(MAX_FILE)
	file, handler, err := r.FormFile("afile")
	if err != nil {
		writeResponse(w, 413, "", "File exceeds maximum size")
		fmt.Println(err)
		return
	}
	defer file.Close()

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("Internal Server Error")) // User IP is not IP:port.  Something is fishy, kill it just in case.
		return
	}

	ext := path.Ext(handler.Filename)
	lat := r.FormValue("lat")
	lng := r.FormValue("lng")
	name := r.FormValue("name")

	id, err := g.add(lat, lng, name, ip, ext)
	if err != nil {
		err = writeResponse(w, 500, "", "Something isn't right...")
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	// It's a good idea to ensure the upload directory below has permissions set to not allow executution
	tempFilename := basePath + "/bin/assets/img/ul/prerendered_" + strconv.Itoa(id) + ext
	f, err := os.OpenFile(tempFilename, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	io.Copy(f, file)

	img, err := imaging.Open(tempFilename)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("Internal Server Error"))
	}
	thumb := imaging.Fit(img, 1024, 768, imaging.Lanczos)
	err = imaging.Save(thumb, basePath+"/bin/assets/img/ul/"+strconv.Itoa(id)+ext)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("Internal Server Error"))
	}
	err = os.Remove(tempFilename)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("Internal Server Error"))
	}

	err = writeResponse(w, 200, "", "")
	if err != nil {
		fmt.Println(err.Error())
	}
}

func (g *glob) add(latitude, longitude, name, ip, ext string) (int, error) {
	var insertID int
	latF, _ := strconv.ParseFloat(latitude, 64)
	lngF, _ := strconv.ParseFloat(longitude, 64)
	if g.exists(latitude, longitude) {
		_, err := g.udb.Db.Query("DELETE FROM locations WHERE ST_X(location::geometry) = $1 AND ST_Y(location::geometry) = $2", lngF, latF)
		if err != nil {
			fmt.Println(err.Error())
			return 0, err
		}
	}
	query := fmt.Sprintf("INSERT INTO locations(location, ip, name, photo_ext, last_updated, flagged, rating, sighting) VALUES(ST_GeogFromText('POINT(%f %f)'), '%s', $1, $2, NOW(), 0, 0, 0) returning id;", lngF, latF, ip)
	err := g.udb.Db.QueryRow(query, name, ext).Scan(&insertID)
	if err != nil {
		fmt.Println(err.Error())
		return -1, err
	}

	return insertID, nil
}

func (g *glob) exists(latitude, longitude string) bool {
	latF, _ := strconv.ParseFloat(latitude, 64)
	lngF, _ := strconv.ParseFloat(longitude, 64)
	rows, err := g.udb.Db.Query("SELECT COUNT(*) as count FROM locations WHERE ST_X(location::geometry) = $1 AND ST_Y(location::geometry) = $2", lngF, latF)
	if err != nil {
		fmt.Println(err.Error())
	}
	var count int
	for rows.Next() {
		err := rows.Scan(&count)
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	return count > 0
}

func (g *glob) flag(id string) (string, error) {
	_, err := g.udb.Db.Query("UPDATE locations SET flagged = flagged + 1, last_updated = NOW() WHERE id = $1", id)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}

	return "", nil
}

func (g *glob) sighting(id, there string) (string, error) {
	var query string
	if there == "1" {
		query = "UPDATE locations SET sighting = sighting + 1, last_updated = NOW() WHERE id = $1"
	} else {
		query = "UPDATE locations SET sighting = sighting - 1, last_updated = NOW() WHERE id = $1"
	}
	_, err := g.udb.Db.Query(query, id)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}

	return "", nil
}

func (g *glob) rate(id, rating string) (string, error) {
	var query string
	if rating == "1" {
		query = "UPDATE locations SET rating = rating + 1, last_updated = NOW() WHERE id = $1"
	} else {
		query = "UPDATE locations SET rating = rating - 1, last_updated = NOW() WHERE id = $1"
	}
	_, err := g.udb.Db.Query(query, id)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}

	return "", nil
}

func (g *glob) loadTrucks(latitude, longitude, radius string) interface{} {
	latF, _ := strconv.ParseFloat(latitude, 64)
	lngF, _ := strconv.ParseFloat(longitude, 64)
	radF, _ := strconv.ParseFloat(radius, 64)

	query := fmt.Sprintf(`SELECT id, ST_X(location::geometry) as lng, ST_Y(location::geometry) as lat, photo_ext, name, last_updated::DATE as lu
								 FROM 	locations
								 WHERE 	ST_Distance_Sphere(
									 			ST_GeomFromText(
													'POINT(%f %f)',4326
												),
												ST_GeomFromText(
													'POINT(' || ST_X(location::geometry) || ' ' || ST_Y(location::geometry) || ')', 4326
												)
										) <= %f * 1609.34`,
		lngF, latF, math.Min(radF, 200), // Maximum 2000mi radius for now, as we grow, this will shrink.
	)

	rows, err := g.udb.Db.Query(query)

	if err != nil {
		fmt.Println(err.Error())
	}
	defer rows.Close()

	var locs []Location

	for rows.Next() {
		var id int
		var lat float64
		var lng float64
		var ext string
		var name string
		var t time.Time
		err := rows.Scan(&id, &lng, &lat, &ext, &name, &t)
		const layout = "Jan 2, 2006"
		date := t.Format(layout)
		locs = append(locs, Location{
			ID:          id,
			Lat:         lat,
			Lng:         lng,
			Ext:         ext,
			Name:        name,
			LastUpdated: date,
		})
		if err != nil {
			fmt.Println(err.Error())
		}
	}
	return locs
}
