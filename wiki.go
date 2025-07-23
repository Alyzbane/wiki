package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"text/template"
)

type Page struct {
	Title string
	Body  []byte
}

const (
	savePath     = "data"
	templatePath = "templates"
)

var templates = template.Must(template.ParseFiles(
	filepath.Join(templatePath, "edit.html"),
	filepath.Join(templatePath, "view.html"),
))

var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

// Create the filename based on the page title and save it in the specified directory.
// Save the page relative to the savePath directory.
func (p *Page) save() error {
	filename := p.Title + ".txt"
	filePath := filepath.Join(savePath, filename)

	return os.WriteFile(filePath, p.Body, 0600)
}

// loadPage loads a page from the file system based on its title.
func loadPage(title string) (*Page, error) {
	filename := title + ".txt"
	filePath := filepath.Join(savePath, filename)

	body, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return &Page{Title: title, Body: body}, nil
}

// renderTemplate renders a template with the provided page data.
// It handles errors by writing an error response to the client.
func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// viewHandler handles requests to view a page.
// If the page does not exist, it redirects to the edit page.
func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

// editHandler handles requests to edit a page.
func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)

	// If the page does not exist, create a new one with an empty body.
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

// saveHandler handles requests to save a page.
// It reads the page title from the URL and the body from the form data.
func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2]) // Call the handler with the title extracted from the URL.
	}
}

func main() {
	err := os.MkdirAll(savePath, 0755) // Ensure the savePath directory exists.
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	log.Println("Server started on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
