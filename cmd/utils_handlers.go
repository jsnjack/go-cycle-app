package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

// Authenticates user with Strava and save tokens in database
func register(w http.ResponseWriter, r *http.Request) {
	// If access is denied, error=access_denied will be included in the query
	// string. If access is accepted, code and scope parameters will be included
	// in the query string. The code parameter contains the authorization code
	// required to complete the authentication process.

	logger, ok := r.Context().Value(HL).(*log.Logger)
	if !ok {
		logger = Logger
	}

	stravaError := r.URL.Query().Get("error")
	if stravaError != "" {
		errText := fmt.Sprintf("Error from Strava: %s\n", stravaError)
		logger.Printf(errText)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, errText)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		errText := "Code not found"
		logger.Printf(errText)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, errText)
		return
	}

	// Request token
	form := url.Values{}
	form.Add("client_id", rootAppID)
	form.Add("client_secret", rootAppSecret)
	form.Add("code", code)
	form.Add("grant_type", "authorization_code")
	formData := form.Encode()
	req, _ := http.NewRequest("POST", StravaAuthURL, strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errText := err.Error()
		logger.Printf(errText)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, errText)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errText := fmt.Sprintf("Unexpected status code from Strava API: %d\n", resp.StatusCode)
		logger.Printf(errText)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, errText)
		return
	}

	// Parse response
	bodyByte, _ := io.ReadAll(resp.Body)
	var stravaData StravaResponseAuth
	err = json.Unmarshal(bodyByte, &stravaData)
	if err != nil {
		errText := err.Error()
		logger.Printf(errText)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, errText)
		return
	}

	// Generate new authID
	authID, err := GenerateRandomID(10)
	if err != nil {
		errText := err.Error()
		logger.Printf(errText)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, errText)
		return
	}

	logger.Printf("New user %s", authID)

	err = SaveAuthData(authID, &stravaData.StravaResponseRefresh, stravaData.Athlete.ID)
	if err != nil {
		errText := err.Error()
		logger.Printf(errText)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, errText)
		return
	}

	http.Redirect(w, r, "https://"+rootDomain+"/register/success?authid="+authID, http.StatusFound)
}

// Uploads activity to Strava
func upload(w http.ResponseWriter, r *http.Request) {
	var authID string
	var file multipart.File

	logger, ok := r.Context().Value(HL).(*log.Logger)
	if !ok {
		logger = Logger
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != "POST" {
		logger.Printf("Method is not allowed: %s\n", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	err := r.ParseMultipartForm(0)
	if err != nil {
		logger.Println(err.Error())
		http.Error(w, "Failed to parse body", http.StatusUnprocessableEntity)
		return
	}
	for k, v := range r.PostForm {
		switch k {
		case "authid":
			authID = v[0]
		default:
			logger.Printf("Unexpected parameter %s: %v\n", k, v)
		}
	}

	accessToken, err := RefreshAccessToken(authID)
	if err != nil {
		errText := err.Error()
		logger.Printf(errText)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, errText)
		return
	}

	file, _, err = r.FormFile("file")
	if err != nil {
		logger.Println(err.Error())
		http.Error(w, "Failed to parse body and retrive file", http.StatusUnprocessableEntity)
		return
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "go_cycle_activity.gpx")
	if err != nil {
		logger.Println(err.Error())
		http.Error(w, "Failed to parse body", http.StatusUnprocessableEntity)
		return
	}
	_, err = io.Copy(part, file)
	if err != nil {
		logger.Println(err.Error())
		http.Error(w, "Failed to parse body", http.StatusUnprocessableEntity)
		return
	}

	writer.WriteField("activity_type", "virtualride")
	writer.WriteField("data_type", "gpx")
	writer.WriteField("name", "go-cycle activity")
	err = writer.Close()
	if err != nil {
		logger.Println(err.Error())
		http.Error(w, "Failed to parse body", http.StatusUnprocessableEntity)
		return
	}

	req, err := http.NewRequest("POST", StravaUploadURL, body)
	if err != nil {
		logger.Println(err.Error())
		http.Error(w, "Failed to parse body", http.StatusUnprocessableEntity)
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errText := err.Error()
		logger.Printf(errText)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, errText)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		bodyByte, _ := io.ReadAll(resp.Body)
		errText := fmt.Sprintf("Unexpected status code from Strava API: %d: %s\n", resp.StatusCode, string(bodyByte))
		logger.Printf(errText)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, errText)
		return
	}
}

// Performs SSL challenge and response to everything else
func registerSuccess(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Another Triumph\ngo-cycle-auth "+Version)
}

// Performs SSL challenge and response to everything else
func sslChallenge(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "go-cycle-auth "+Version)
}

// connectRequest is the entry point for a new user to register in the app
func connectRequest(w http.ResponseWriter, r *http.Request) {
	// Read the template file from the embedded filesystem
	tmplContent, err := TemplatesStorage.ReadFile("templates/connectRequest.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse the template content
	tmpl, err := template.New("template").Parse(string(tmplContent))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		AppID       string
		RedirectURL string
	}{
		AppID:       rootAppID,
		RedirectURL: "https://" + rootDomain + "/register",
	}

	// Render the template with the provided data
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
