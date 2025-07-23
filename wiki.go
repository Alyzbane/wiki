package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Page represents a wiki page with a title and content body
type Page struct {
	Title string
	Body  []byte
}

// IndexPage contains data for rendering the index page with all available pages
type IndexPage struct {
	Pages []string
}

const (
	savePath     = "data"      // Directory where wiki pages are stored
	templatePath = "templates" // Directory containing HTML templates
)

// Pre-compiled templates with custom function for processing wiki links
var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"processLinks": processLinks,
}).ParseFiles(
	filepath.Join(templatePath, "edit.html"),
	filepath.Join(templatePath, "view.html"),
	filepath.Join(templatePath, "index.html"),
))

// Regular expression to validate and extract page names from URLs
var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

// =============================================================================
// DATA PERSISTENCE FUNCTIONS
// =============================================================================

// save writes the page content to a text file in the data directory
func (p *Page) save() error {
	filename := p.Title + ".txt"
	filePath := filepath.Join(savePath, filename)

	return os.WriteFile(filePath, p.Body, 0600)
}

// loadPage retrieves a wiki page from the filesystem by reading its corresponding text file
func loadPage(title string) (*Page, error) {
	filename := title + ".txt"
	filePath := filepath.Join(savePath, filename)

	body, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return &Page{Title: title, Body: body}, nil
}

// getAllPages scans the data directory and returns a list of all available wiki page names
func getAllPages() ([]string, error) {
	files, err := os.ReadDir(savePath)
	if err != nil {
		return nil, err
	}

	var pages []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".txt") {
			pageName := strings.TrimSuffix(file.Name(), ".txt")
			pages = append(pages, pageName)
		}
	}
	return pages, nil
}

// =============================================================================
// TEMPLATE RENDERING FUNCTIONS
// =============================================================================

// processLinks converts wiki-style links [PageName] into HTML anchor tags
func processLinks(body []byte) template.HTML {
	s := string(body)
	re := regexp.MustCompile(`\[([a-zA-Z0-9]+)\]`)
	processed := re.ReplaceAllStringFunc(s, func(match string) string {
		pageName := match[1 : len(match)-1]
		return `<a href="/view/` + pageName + `">` + pageName + `</a>`
	})
	return template.HTML(processed)
}

// renderTemplate executes an HTML template with page data and handles any rendering errors
func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// renderIndexTemplate executes the index template with a list of all available pages
func renderIndexTemplate(w http.ResponseWriter, tmpl string, indexData *IndexPage) {
	err := templates.ExecuteTemplate(w, tmpl+".html", indexData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// =============================================================================
// HTTP HANDLER FUNCTIONS
// =============================================================================

// indexHandler displays the main index page showing all available wiki pages
func indexHandler(w http.ResponseWriter, r *http.Request) {
	pages, err := getAllPages()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	indexData := &IndexPage{Pages: pages}
	renderIndexTemplate(w, "index", indexData)
}

// viewHandler displays a wiki page in read-only mode, redirecting to edit if page doesn't exist
func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

// editHandler displays the edit form for a wiki page, creating a new page if it doesn't exist
func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)

	// If the page does not exist, create a new one with an empty body.
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

// saveHandler processes form submissions to save wiki page content and redirects to view mode
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

// rootHandler handles requests to the root path, redirecting to the index page
func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		indexHandler(w, r)
		return
	}
	http.NotFound(w, r)
}

// =============================================================================
// MIDDLEWARE AND UTILITY FUNCTIONS
// =============================================================================

// makeHandler creates a wrapper that validates URL paths and extracts page titles before calling the actual handler
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

// =============================================================================
// APPLICATION ENTRY POINT
// =============================================================================

// main initializes the wiki application, sets up HTTP routes, and starts the web server
func main() {
	err := os.MkdirAll(savePath, 0755) // Ensure the savePath directory exists.
	if err != nil {
		log.Fatal(err)
	}

	// Serve static files (CSS)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/index", indexHandler)
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))

	// Wrap the default ServeMux with a logging middleware
	loggedMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/static/") && r.URL.Path != "/favicon.ico" {
			log.Printf("Request: %s %s", r.Method, r.URL.Path)
		}
		http.DefaultServeMux.ServeHTTP(w, r)
	})

	// Log server start and listen on port 8080
	log.Println("Server started on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", loggedMux))
}
