package api

import (
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

var errorTemplate *template.Template

/*
Init initializes the error template for the application.
It attempts to parse the error.html template file. If parsing fails,
it creates a simple fallback template to ensure error rendering.
This function should be called once at the start of the application.
*/
func Init() {
	var err error
	errorTemplate, err = template.ParseFiles("template/error.html")
	if err != nil {
		// log.Printf("Warning: Error parsing error template: %v", err)
		// Create a simple fallback template
		errorTemplate = template.Must(template.New("error").Parse(`
            <html><body>
            <h1>Error {{.Code}}</h1>
            <p>{{.Message}}</p>
            </body></html>
        `))
		//  log.Println("Error parsing, using fallback template")
	}
}

/*
renderError handles the rendering of error pages.
It sets the HTTP status code, executes the error template with the provided status and message,
and logs any errors that occur during template execution.
Parameters:
  - w: http.ResponseWriter to write the response
  - status: HTTP status code for the error
  - message: Error message to display
*/
func renderError(w http.ResponseWriter, status int, message string) {
	Init()
	w.WriteHeader(status)
	err := errorTemplate.Execute(w, struct {
		Code    int
		Message string
	}{
		Code:    status,
		Message: message,
	})
	if err != nil {
		log.Printf("Error rendering error template: %v", err)
	}
}

/*
HomeHandler manages requests to the home page of the application.
It checks if the requested path is the root ("/") and if the HTTP method is GET.
If these conditions are not met, it renders appropriate error pages.
Otherwise, it parses and executes the home.html template.

Parameters:
  - w: http.ResponseWriter to write the response
  - r: *http.Request containing the request details
*/
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		renderError(w, http.StatusNotFound, "The Page you're trying to acess is unavailable")
		return
	}

	if r.Method != http.MethodGet {
		renderError(w, http.StatusMethodNotAllowed, "Wrong method")
		return
	}

	// Parse the homepage template
	temp, err := template.ParseFiles("template/home.html") // Ensure you have home.html in the template directory
	if err != nil {
		renderError(w, http.StatusInternalServerError, "Error loading template")
		return
	}

	// Execute the template and write to the response
	err = temp.Execute(w, nil) // No data is passed to the homepage template
	if err != nil {
		renderError(w, http.StatusInternalServerError, "Error executing template")
	}
}

/*
ArtistsHandler manages requests to the artists listing page.
It verifies the correct URL path and HTTP method, then fetches and displays
the list of artists. If any errors occur during this process, it renders
appropriate error pages.

Parameters:
  - w: http.ResponseWriter to write the response
  - r: *http.Request containing the request details
*/
func ArtistsHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/artists/" {
		renderError(w, http.StatusNotFound, "The Page you're trying to acess is unavailable")
		return
	}

	if r.Method != http.MethodGet {
		renderError(w, http.StatusMethodNotAllowed, "Wrong method")
		return
	}

	templatePath := filepath.Join("template", "artists.html")
	temp1, err := template.ParseFiles(templatePath)
	if err != nil {
		renderError(w, http.StatusInternalServerError, "Error loading template")
		return
	}

	result, err := ReadArtists("https://groupietrackers.herokuapp.com/api/artists")
	if err != nil {
		renderError(w, http.StatusInternalServerError, "Error fetching artists")
		return
	}

	err = temp1.Execute(w, result)
	if err != nil {
		renderError(w, http.StatusInternalServerError, "Error executing template")
	}
}

type ArtistData struct {
	Artist    Artist    `json:"artist"`
	Dates     DateEntry `json:"dates"`
	Locations Location  `json:"locations"`
	Relations Relation  `json:"relations"`
	Section   string    `json:"section"`
}

func ArtistHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		renderError(w, http.StatusMethodNotAllowed, "Wrong method")
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/artist/") || len(strings.Split(r.URL.Path, "/")) != 3 {
		renderError(w, http.StatusNotFound, "The Page you're trying to access is unavailable")
		return
	}

	id1 := strings.Split(r.URL.Path, "/")
	if len(id1) < 3 {
		renderError(w, http.StatusBadRequest, "Artist ID not found")
		return
	}

	id := id1[len(id1)-1]

	// Check for the section query parameter
	section := r.URL.Query().Get("section")
	if section != "" && section != "locations" && section != "dates" && section != "relations" && section != "all" {
		renderError(w, http.StatusNotFound, "The section you're trying to access is unavailable")
		return
	}

	// Fetch artist details
	baseURL := "https://groupietrackers.herokuapp.com/api/artists/"
	artistResult, err := ReadArtist(baseURL, id)
	if err != nil || artistResult.ID == 0 {
		renderError(w, http.StatusNotFound, "The Page you're trying to access is unavailable")
		return
	}

	// Fetch related data: dates, locations, relations
	datesResult, err := ReadDate("https://groupietrackers.herokuapp.com/api/dates/", id)
	if err != nil {
		renderError(w, http.StatusInternalServerError, "Error fetching dates")
		return
	}

	locationsResult, err := ReadLocation("https://groupietrackers.herokuapp.com/api/locations/", id)
	if err != nil {
		renderError(w, http.StatusInternalServerError, "Error fetching locations")
		return
	}

	relationsResult, err := ReadRelations("https://groupietrackers.herokuapp.com/api/relation/", id)
	if err != nil {
		renderError(w, http.StatusInternalServerError, "Error fetching relations")
		return
	}

	// Combine all results into a single struct to pass to the template
	artistData := ArtistData{
		Artist:    artistResult,
		Dates:     datesResult,
		Locations: locationsResult,
		Relations: relationsResult,
		Section:   section, // Add this line to pass the section to the template
	}

	// Load and execute the artist template with combined data
	temp1, err := template.ParseFiles("template/artist.html")
	if err != nil {
		renderError(w, http.StatusInternalServerError, "Error loading template")
		return
	}

	err = temp1.Execute(w, artistData)
	if err != nil {
		renderError(w, http.StatusInternalServerError, "Error executing template")
	}
}
// SearchLocationHandler handles search requests for artist locations.
func SearchLocationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		renderError(w, http.StatusMethodNotAllowed, "Wrong method")
		return
	}

	// Get the location query parameter from the URL
	location := r.URL.Query().Get("location")
	if location == "" {
		renderError(w, http.StatusBadRequest, "Location query parameter is required")
		return
	}

	// Retrieve the list of artists from the API
	artists, err := ReadArtists("https://groupietrackers.herokuapp.com/api/artists")
	if err != nil {
		renderError(w, http.StatusInternalServerError, "Error fetching artists")
		return
	}

	// Prepare a slice to hold artists who match the location
	var filteredArtists []Artistlocation // Ensure ArtistData contains artist info and location info

	// Iterate through each artist to fetch their locations
	for _, artist := range artists {
		// Convert artist.ID to string for the ReadLocation function
		artistIDStr := strconv.Itoa(artist.ID)

		// Fetch location data for each artist by ID
		locationsResult, err := ReadLocation("https://groupietrackers.herokuapp.com/api/locations/", artistIDStr)
		if err != nil {
			log.Printf("Error fetching locations for artist ID %d: %v", artist.ID, err)
			continue
		}

		// Check if any of the artist's locations match the query (case-insensitive)
		for _, loc := range locationsResult.Locations {
			if strings.EqualFold(loc, location) {
				// Add matching artist to the filtered list
				filteredArtists = append(filteredArtists, Artistlocation{
					Artist:    artist,
					Locations: locationsResult, // Store the locations for rendering later
				})
				break // No need to check other locations for this artist
			}
		}
	}

	// Render the filtered results using the search_results template
	temp, err := template.ParseFiles("template/search_results.html")
	if err != nil {
		renderError(w, http.StatusInternalServerError, "Error loading search results template")
		return
	}

	err = temp.Execute(w, filteredArtists) // Pass the filtered results to the template
	if err != nil {
		renderError(w, http.StatusInternalServerError, "Error executing search results template")
	}
}
