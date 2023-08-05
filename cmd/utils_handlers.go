package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
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

// Performs SSL challenge and response to everything else
func registerSuccess(w http.ResponseWriter, r *http.Request) {
	Logger.Println("succesfully registered, subscribing to webhook")
	data := url.Values{}
	data.Add("client_id", rootAppID)
	data.Add("client_secret", rootAppSecret)
	data.Add("callback_url", "https://"+rootDomain+"/webhook")
	data.Add("verify_token", rootAppVerifyToken)

	resp, err := http.PostForm(StravaWebhookSubscribeURL, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Unexpected status code from Strava API: %d\n", resp.StatusCode), http.StatusBadGateway)
		return
	}
	fmt.Fprint(w, "Subscribing to webhook")
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

// webhook handles all incoming webhooks from Strava
func webhook(w http.ResponseWriter, r *http.Request) {
	// This endpoint needs to handle both POST (actual webhook) and GET (subscription confirmation)
	// methods
	Logger.Printf("incoming webhook: %s\n", r.Method)
	switch r.Method {
	case "POST":
		Logger.Println("unhandled POST")
	case "GET":
		// Callback validation
		queryParams := r.URL.Query()
		hubMode := queryParams.Get("hub.mode")
		if hubMode != "subscribe" {
			http.Error(w, "Invalid mode", http.StatusBadRequest)
			return
		}
		hubVerifyToken := queryParams.Get("hub.verify_token")
		if hubVerifyToken != rootAppVerifyToken {
			http.Error(w, "Invalid verify token", http.StatusForbidden)
			return
		}
		hubChallenge := queryParams.Get("hub.challenge")
		payload := struct {
			Challenge string `json:"hub.challenge"`
		}{
			Challenge: hubChallenge,
		}
		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		req, err := http.NewRequest("POST", r.RequestURI, bytes.NewBuffer(jsonPayload))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		Logger.Println("responded to webhook validation request")
		return
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
