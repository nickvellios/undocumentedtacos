package main

import (
	"log"
	"net/http"
	"time"

	"tacos/db"

	"github.com/julienschmidt/httprouter"
)

type glob struct {
	udb *db.UrlDB
}

const (
	readTimeout  = time.Duration(60 * time.Second)
	writeTimeout = time.Duration(60 * time.Second)
	basePath     = "/root/go"
)

var templateDir = basePath + "/src/tacos/templates/"

func main() {
	g := &glob{}
	g.udb = &db.UrlDB{}
	g.udb.Open()
	defer g.udb.Db.Close()

	router80 := httprouter.New()
	router80.GET("/", g.Index80)
	router := httprouter.New()
	router.GET("/", g.Index)
	router.GET("/trucks/:lat/:lng/:rad", g.Trucks)
	router.POST("/sighting/:id/:rating", g.Sighting)
	router.POST("/rate/:id/:vote", g.Rate)
	router.POST("/flag/:id", g.Flag)
	router.POST("/addtruck", g.AddTruck)
	router.ServeFiles("/assets/*filepath", http.Dir(basePath+"/bin/assets"))

	srv := http.Server{
		Addr:         ":80",
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		Handler:      router80,
	}

	srv_tls := http.Server{
		Addr:         ":443",
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		Handler:      router,
	}

	go func() {
		log.Fatal(srv.ListenAndServe())
	}()

	log.Fatal(srv_tls.ListenAndServeTLS("/path/to/fullchain.pem", "/path/to/privkey.pem"))
}
