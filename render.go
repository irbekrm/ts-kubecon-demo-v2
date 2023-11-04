package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"tailscale.com/util/must"
	"tailscale.com/words"
)

//go:embed picker.html
var embeddedTemplate string

const wkPattern = "https://en.wikipedia.org/w/api.php?action=query&prop=pageimages&format=json&piprop=original&pilimit=1&titles=%s"

type WikiImg struct {
	Source string `json:"source"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
type WikiEntry struct {
	Title    string  `json:"title"`
	Original WikiImg `json:"original"`
}

type WikiResp struct {
	Query struct {
		Pages map[string]WikiEntry
	} `json:"query"`
}

func getImg(ctx context.Context, words []string) Img {
	start := time.Now()
	var imgSrc, title string
	tries := 0
	found := false
	for !found && tries < 5 {
		tries++

		idx := rand.Intn(len(words))
		word := words[idx]

		req := must.Get(http.NewRequestWithContext(ctx, "GET", fmt.Sprintf(wkPattern, word), nil))
		req.Header.Add("Accept-Encoding", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			continue // try a different one
		}
		b, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			log.Fatalf("getImg: response read failed %d: %v", res.StatusCode, err)
		}
		if res.StatusCode != 200 {
			continue // try a different one
		}
		var wiki WikiResp
		err = json.Unmarshal(b, &wiki)
		if err != nil {
			log.Printf("getImg: unmarshal failed: %v", err)
			continue // try a different one
		}

		if wiki.Query.Pages == nil {
			log.Printf("getImg: nil query page: %v", wiki)
			continue // try a different one
		}

		for _, v := range wiki.Query.Pages {
			if (v.Original == WikiImg{}) {
				continue // try a different one
			}
			imgSrc = v.Original.Source
			title = v.Title

			if v.Original.Width < 1000 {
				found = true // if the image is too large, try to find a smaller one
			}
		}
	}
	end := time.Now()
	log.Printf("image retrieval took %v ms for %v tries", end.UnixMilli()-start.UnixMilli(), tries)
	return Img{
		Src:  imgSrc,
		Name: title,
	}
}

type Img struct {
	Src  string
	Name string
}

func render(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("ts").Parse(embeddedTemplate))

	data := struct {
		Tail  Img
		Scale Img
	}{
		Tail:  getImg(r.Context(), words.Tails()),
		Scale: getImg(r.Context(), words.Scales()),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, data)
}
