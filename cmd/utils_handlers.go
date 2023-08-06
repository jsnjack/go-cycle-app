package cmd

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jellydator/ttlcache/v3"
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

	logger.Printf("saving data for athlete %d", stravaData.Athlete.ID)
	err = SaveAuthData(stravaData.Athlete.ID, &stravaData.StravaResponseRefresh)
	if err != nil {
		errText := err.Error()
		logger.Printf(errText)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, errText)
		return
	}

	accountID, err := GenerateRandomID(30)
	logger.Printf("generating account id for athlete %d: %s", stravaData.Athlete.ID, accountID)
	if err != nil {
		logger.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, err.Error())
		return
	}

	AccountCache.Set(accountID, stravaData.Athlete.ID, ttlcache.DefaultTTL)

	http.Redirect(w, r, "https://"+rootDomain+"/account?accountId="+accountID, http.StatusFound)
}

func accountHandler(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("accountId")
	item := AccountCache.Get(accountID)
	if item.Value() == 0 {
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}
	athleteID := item.Value()

	switch r.Method {
	case http.MethodGet:
		tmplContent, err := TemplatesStorage.ReadFile("templates/account.html")
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

		// Render the template with the provided data
		err = tmpl.Execute(w, map[string]interface{}{
			"AthleteID": athleteID,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case http.MethodPost:
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Failed to parse form data", http.StatusBadRequest)
			return
		}

		// Extract the form values
		goalStr := r.FormValue("goal")
		goalNumber, err := strconv.Atoi(goalStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = SetGoal(athleteID, float64(goalNumber))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, "https://"+rootDomain+"/success", http.StatusFound)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func successHandler(w http.ResponseWriter, r *http.Request) {
	tmplContent, err := TemplatesStorage.ReadFile("templates/success.html")
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

	// Render the template with the provided data
	err = tmpl.Execute(w, map[string]interface{}{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Subscribes app to Strava webhooks. Done only once
func subscribeToWebhook(w http.ResponseWriter, r *http.Request) {
	logger, ok := r.Context().Value(HL).(*log.Logger)
	if !ok {
		logger = Logger
	}
	data := url.Values{}
	data.Add("client_id", rootAppID)
	data.Add("client_secret", rootAppSecret)
	data.Add("callback_url", "https://"+rootDomain+"/webhook")
	data.Add("verify_token", rootAppVerifyToken)

	resp, err := http.PostForm(StravaWebhookSubscribeURL, data)
	if err != nil {
		logger.Println(err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
}

// rootHandler is the entry point for a new user to register in the app
func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
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
	logger, ok := r.Context().Value(HL).(*log.Logger)
	if !ok {
		logger = Logger
	}
	switch r.Method {
	case "POST":
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data := StravaWebhookData{}
		err = json.Unmarshal(body, &data)
		if err != nil {
			logger.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if data.AspectType != "delete" && data.ObjectType == "activity" {
			logger.Printf("new activity %d for user %d\n", data.ObjectID, data.OwnerID)
			go addCommentToActivity(data.ObjectID, data.OwnerID)
		}
	case "GET":
		// Callback validation
		queryParams := r.URL.Query()
		hubMode := queryParams.Get("hub.mode")
		if hubMode != "subscribe" {
			logger.Println("invalid mode")
			http.Error(w, "Invalid mode", http.StatusBadRequest)
			return
		}
		hubVerifyToken := queryParams.Get("hub.verify_token")
		if hubVerifyToken != rootAppVerifyToken {
			logger.Println("invalid verify token")
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
			logger.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.Write(jsonPayload)
		logger.Println("responded to webhook validation request")
		return
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
