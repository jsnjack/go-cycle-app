package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"golang.org/x/exp/slices"
)

// StravaAuthURL is the URL of oauth endpoint
const StravaAuthURL = "https://www.strava.com/oauth/token"

// StravaUploadURL is the URL of Strava upload endpoint
const StravaUploadURL = "https://www.strava.com/api/v3/uploads"

const StravaWebhookSubscribeURL = "https://www.strava.com/api/v3/push_subscriptions"

const StravaListActivitiesURL = "https://www.strava.com/api/v3/athlete/activities"
const StravaUpdateActivityURL = "https://www.strava.com/api/v3/activities"

// List of activities which are considered to be "cycling" activities
var CyclingActivities = []string{
	"GravelRide",
	"Handcycle",
	"MountainBikeRide",
	"Ride",
	"VirtualRide",
}

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
	SportType   string  `json:"sport_type"`
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
	Logger.Printf("found %d cycling activities\n", len(*activities))

	totalDistance := 0.0
	activityDistance := 0.0
	activityDescription := ""
	for _, activity := range *activities {
		totalDistance += activity.Distance
		if activity.ID == activityID {
			Logger.Printf("found matching activity %d\n", activity.ID)
			activityDistance = activity.Distance
			activityDescription = activity.Description
			if !slices.Contains(CyclingActivities, activity.SportType) {
				Logger.Printf("activity %d is not cycling\n", activityID)
				return
			}
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

	newDesc, err := renderDescription(goal, totalDistance, activityDistance, activityDescription, signature)
	if err != nil {
		Logger.Println(err)
		return
	}

	// Update activity
	data := struct {
		Description string `json:"description"`
	}{
		Description: newDesc,
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

// Returns all cycling activities of the current year
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
		for _, activity := range *fetched {
			if slices.Contains(CyclingActivities, activity.SportType) {
				activities = append(activities, activity)
			}
		}
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

// renderDescription renders description of the activity from the template
// Notes:
//   - all distance is in meters
//   - `totalDistance` already includes `activityDistance`
func renderDescription(goal, totalDistance, activityDistance float64, description, signature string) (string, error) {
	year := time.Now().Year()
	tmplContent, err := TemplatesStorage.ReadFile("templates/description.txt")
	if err != nil {
		return "", err
	}

	funcMap := template.FuncMap{
		"toFixedTwo": func(f float64) string {
			format := fmt.Sprintf("%%.%df", 2)
			return fmt.Sprintf(format, f)
		},
		"greaterFloat": func(a float64, b float64) bool {
			return a >= b
		},
	}

	// Parse the template content
	tmpl := template.Must(template.New("example").Funcs(funcMap).Parse(string(tmplContent)))

	// Render the template with the provided data
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]interface{}{
		"Description":   description,
		"Year":          year,
		"Goal":          goal / 1000,
		"TotalDistance": totalDistance / 1000,
		"Progress":      (totalDistance / goal) * 100,
		"Contributed":   activityDistance / goal * 100,
		"DistanceLeft":  (goal - totalDistance) / 1000,
		"DaysLeft":      int(time.Until(time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC)).Hours()/24 - 1),
		"Signature":     signature,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}
