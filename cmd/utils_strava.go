package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"
)

// StravaAuthURL is the URL of oauth endpoint
const StravaAuthURL = "https://www.strava.com/oauth/token"

// StravaUploadURL is the URL of Strava upload endpoint
const StravaUploadURL = "https://www.strava.com/api/v3/uploads"

const StravaWebhookSubscribeURL = "https://www.strava.com/api/v3/push_subscriptions"

const StravaListActivitiesURL = "https://www.strava.com/api/v3/athlete/activities"
const StravaUpdateActivityURL = "https://www.strava.com/api/v3/activities"

// StravaResponseRefresh represents JSON data that comes from Strava
type StravaResponseRefresh struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int    `json:"expires_at"`
}

// StravaResponseAuth ...
type StravaResponseAuth struct {
	StravaResponseRefresh
	Athlete AthleteData `json:"athlete"`
}

// AthleteData is the `athlete` key in StravaResponse
type AthleteData struct {
	ID int `json:"id"`
}

type Activity struct {
	ID          int     `json:"id"`
	Type        string  `json:"type"`
	Distance    float64 `json:"distance"`
	Description string  `json:"description"`
}

type StravaWebhookData struct {
	ObjectType string `json:"object_type"`
	ObjectID   int    `json:"object_id"`
	AspectType string `json:"aspect_type"`
	OwnerID    int    `json:"owner_id"`
}

func addCommentToActivity(activityID int, userID int) {
	goal, err := GetGoal(userID)
	if err != nil {
		Logger.Println(err)
		goal = 5000000
	}
	signature := "-- https://go-cycle.yauhen.cc"
	accessToken, err := RefreshAccessToken(userID)
	if err != nil {
		Logger.Println(err)
		return
	}

	activities, err := getYearActivities(accessToken)
	if err != nil {
		Logger.Println(err)
		return
	}

	totalDistance := 0.0
	activityDistance := 0.0
	activityDescription := ""
	for _, activity := range *activities {
		totalDistance += activity.Distance
		if activity.ID == activityID {
			Logger.Printf("found matching activity %d\n", activity.ID)
			activityDistance = activity.Distance
			activityDescription = activity.Description
		}
	}

	if activityDistance == 0 {
		Logger.Printf("activity %d not found\n", activityID)
		return
	}

	Logger.Printf("total distance for user %d: %f\n", userID, totalDistance)
	Logger.Printf("activity %d, distance: %f\n", activityID, activityDistance)
	Logger.Printf("activity %d, description: %s\n", activityID, activityDescription)

	if strings.Contains(activityDescription, signature) {
		Logger.Printf("activity %d already has signature\n", activityID)
		return
	}

	year := time.Now().Year()

	tmplContent, err := TemplatesStorage.ReadFile("templates/description.txt")
	if err != nil {
		Logger.Println(err)
		return
	}

	// Parse the template content
	tmpl, err := template.New("template").Parse(string(tmplContent))
	if err != nil {
		Logger.Println(err)
		return
	}

	// Render the template with the provided data
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]interface{}{
		"Description":   activityDescription,
		"Year":          fmt.Sprintf("%d", year),
		"Goal":          fmt.Sprintf("%.2f", goal/1000),
		"TotalDistance": fmt.Sprintf("%.2f", totalDistance/1000),
		"Progress":      fmt.Sprintf("%.2f", (totalDistance/goal)*100),
		"Contributed":   fmt.Sprintf("%.2f", activityDistance/goal*100),
		"DistanceLeft":  fmt.Sprintf("%.2f", (goal-totalDistance)/1000),
		"DaysLeft":      fmt.Sprintf("%d", int(time.Until(time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC)).Hours()/24-1)),
		"Signature":     signature,
	})
	if err != nil {
		Logger.Println(err)
		return
	}

	// Update activity
	data := struct {
		Description string `json:"description"`
	}{
		Description: strings.TrimSpace(buf.String()),
	}
	dataJson, err := json.Marshal(data)
	if err != nil {
		Logger.Println(err)
		return
	}
	body := bytes.NewBuffer(dataJson)
	req, err := http.NewRequest("PUT", StravaUpdateActivityURL+fmt.Sprintf("/%d", activityID), body)
	if err != nil {
		Logger.Println(err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		Logger.Println(err)
		return
	}
	defer resp.Body.Close()
	Logger.Printf("updating activity %d: %s\n", activityID, resp.Status)
}

func getYearActivities(accessToken string) (*[]Activity, error) {
	startOfYear := time.Date(time.Now().Year(), time.January, 1, 0, 0, 0, 0, time.UTC).Unix()

	url := StravaListActivitiesURL + fmt.Sprintf("?after=%d", startOfYear)
	page := 0

	var activities []Activity

	for {
		page += 1
		fetched, err := makePaginatedRequest(url, accessToken, page)
		if err != nil {
			return nil, err
		}
		if len(*fetched) == 0 {
			break
		}
		activities = append(activities, *fetched...)
	}
	return &activities, nil
}

func makePaginatedRequest(url string, accessToken string, page int) (*[]Activity, error) {
	req, err := http.NewRequest("GET", url+"&page="+fmt.Sprintf("%d", page), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		Logger.Printf("Failed to retrieve activities: %s\n", resp.Status)
		return nil, err
	}

	var activities []Activity
	err = json.NewDecoder(resp.Body).Decode(&activities)
	if err != nil {
		return nil, err
	}
	Logger.Printf("Retrieved %d activities\n", len(activities))
	return &activities, nil
}
